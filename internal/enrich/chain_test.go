package enrich

import (
	"context"
	"errors"
	"testing"
)

type stub struct {
	calls int
	errs  []error // error por llamada; nil = éxito; se repite el último
	out   string
}

func (s *stub) Complete(context.Context, string, string) (string, error) {
	i := min(s.calls, len(s.errs)-1)
	s.calls++
	if s.errs[i] != nil {
		return "", s.errs[i]
	}
	return s.out, nil
}

func TestChainFallsBackOnError(t *testing.T) {
	a := &stub{errs: []error{errors.New("HTTP 503")}}
	b := &stub{errs: []error{nil}, out: "b"}
	c := NewChain(Provider{"a", a}, Provider{"b", b})
	got, err := c.Complete(context.Background(), "s", "u")
	if err != nil || got != "b" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestChainSkipsRateLimitedProvider(t *testing.T) {
	a := &stub{errs: []error{ErrRateLimited}}
	b := &stub{errs: []error{nil}, out: "b"}
	c := NewChain(Provider{"a", a}, Provider{"b", b})
	for i := 0; i < 3; i++ {
		if got, err := c.Complete(context.Background(), "s", "u"); err != nil || got != "b" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	if a.calls != 1 {
		t.Errorf("el proveedor limitado se llamó %d veces, quería 1", a.calls)
	}
}

func TestChainDropsProviderAfterRepeatedFailures(t *testing.T) {
	a := &stub{errs: []error{errors.New("HTTP 500")}}
	b := &stub{errs: []error{nil}, out: "b"}
	c := NewChain(Provider{"a", a}, Provider{"b", b})
	for i := 0; i < 6; i++ {
		c.Complete(context.Background(), "s", "u")
	}
	if a.calls != maxConsecutiveFailures {
		t.Errorf("llamadas a a = %d, quería %d", a.calls, maxConsecutiveFailures)
	}
}

func TestChainRateLimitedOnlyWhenAllAre(t *testing.T) {
	a := &stub{errs: []error{ErrRateLimited}}
	b := &stub{errs: []error{errors.New("HTTP 500")}}
	c := NewChain(Provider{"a", a}, Provider{"b", b})
	if _, err := c.Complete(context.Background(), "s", "u"); errors.Is(err, ErrRateLimited) {
		t.Error("con un proveedor aún disponible no debe devolver ErrRateLimited")
	}
	b.errs = []error{ErrRateLimited}
	if _, err := c.Complete(context.Background(), "s", "u"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, quería ErrRateLimited", err)
	}
}

func TestChainFromEnv(t *testing.T) {
	for _, k := range []string{"LLM_API_KEY", "LLM_MODEL", "LLM_BASE_URL", "LLM2_API_KEY", "LLM2_MODEL", "LLM2_BASE_URL", "LLM3_API_KEY"} {
		t.Setenv(k, "")
	}
	if c, err := ChainFromEnv(); c != nil || err != nil {
		t.Fatalf("sin configuración: %v %v", c, err)
	}
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_MODEL", "m")
	t.Setenv("LLM2_API_KEY", "k2")
	if _, err := ChainFromEnv(); err == nil {
		t.Error("un respaldo sin modelo ni URL debe dar error")
	}
	t.Setenv("LLM2_MODEL", "m2")
	t.Setenv("LLM2_BASE_URL", "https://api.groq.com/openai/v1")
	c, err := ChainFromEnv()
	if err != nil || len(c.providers) != 2 || c.providers[1].Name != "m2" {
		t.Fatalf("c=%+v err=%v", c, err)
	}
}
