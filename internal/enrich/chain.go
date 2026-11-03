package enrich

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// maxConsecutiveFailures son los fallos seguidos tras los que un proveedor se
// da por caído durante el resto de la ejecución.
const maxConsecutiveFailures = 3

// Provider es un Completer con nombre dentro de una Chain.
type Provider struct {
	Name string
	LLM  Completer
}

// Chain prueba los proveedores por orden: si uno falla o está limitado, la
// misma petición pasa al siguiente. Un proveedor que agota su cuota, o falla
// varias veces seguidas, se salta durante el resto de la ejecución.
type Chain struct {
	providers []Provider
	down      []bool
	failures  []int
}

// NewChain crea una cadena con los proveedores en orden de preferencia.
func NewChain(providers ...Provider) *Chain {
	return &Chain{
		providers: providers,
		down:      make([]bool, len(providers)),
		failures:  make([]int, len(providers)),
	}
}

// Complete devuelve la primera respuesta correcta. Solo devuelve
// ErrRateLimited cuando todos los proveedores han agotado su cuota.
func (c *Chain) Complete(ctx context.Context, system, user string) (string, error) {
	var lastErr error
	limited, tried := 0, 0
	for i, p := range c.providers {
		if c.down[i] {
			if errors.Is(c.failuresErr(i), ErrRateLimited) {
				limited++
			}
			continue
		}
		tried++
		out, err := p.LLM.Complete(ctx, system, user)
		if err == nil {
			c.failures[i] = 0
			return out, nil
		}
		lastErr = fmt.Errorf("%s: %w", p.Name, err)
		if errors.Is(err, ErrRateLimited) {
			c.down[i] = true
			c.failures[i] = -1 // marca "limitado"
			limited++
		} else if c.failures[i]++; c.failures[i] >= maxConsecutiveFailures {
			c.down[i] = true
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	if limited == len(c.providers) {
		return "", ErrRateLimited
	}
	if lastErr == nil {
		lastErr = errors.New("API de IA: ningún proveedor disponible")
	}
	return "", lastErr
}

// failuresErr indica por qué está caído el proveedor i.
func (c *Chain) failuresErr(i int) error {
	if c.failures[i] == -1 {
		return ErrRateLimited
	}
	return errors.New("caído")
}

// ChainFromEnv construye la cadena a partir de LLM_* (principal) y de
// LLM2_*, LLM3_*… (respaldos, en ese orden), cada uno con _API_KEY, _MODEL y
// _BASE_URL. En los respaldos _BASE_URL es obligatoria. Devuelve nil si no hay
// ningún proveedor configurado.
func ChainFromEnv() (*Chain, error) {
	var providers []Provider
	primary, err := ClientFromEnv()
	if err != nil {
		return nil, err
	}
	if primary != nil {
		providers = append(providers, Provider{Name: primary.Model, LLM: primary})
	}
	for n := 2; n <= 9; n++ {
		prefix := fmt.Sprintf("LLM%d_", n)
		key := strings.TrimSpace(os.Getenv(prefix + "API_KEY"))
		if key == "" {
			continue
		}
		model := strings.TrimSpace(os.Getenv(prefix + "MODEL"))
		base := strings.TrimSpace(os.Getenv(prefix + "BASE_URL"))
		if model == "" || base == "" {
			return nil, fmt.Errorf("%sAPI_KEY está definida pero faltan %sMODEL o %sBASE_URL", prefix, prefix, prefix)
		}
		providers = append(providers, Provider{Name: model, LLM: newClient(base, key, model)})
	}
	if len(providers) == 0 {
		return nil, nil
	}
	return NewChain(providers...), nil
}
