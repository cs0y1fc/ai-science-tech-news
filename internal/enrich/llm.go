package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrRateLimited indica que la API agotó su cuota o pide esperar. Quien llama
// debe dejar de hacer peticiones en esta ejecución.
var ErrRateLimited = errors.New("API de IA: límite de uso alcanzado")

// Completer devuelve la respuesta de un modelo de lenguaje a un par
// sistema/usuario. Es una interfaz para poder probar sin red.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Client habla con cualquier API compatible con OpenAI Chat Completions, lo
// que cubre Gemini (endpoint /openai), Groq y otros con capa gratuita.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// defaultBaseURL es el endpoint compatible con OpenAI de Gemini.
const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

// ClientFromEnv crea un cliente a partir de LLM_API_KEY, LLM_MODEL y,
// opcionalmente, LLM_BASE_URL. Devuelve nil si no hay clave: la web funciona
// igualmente, con el respaldo por palabras clave.
func ClientFromEnv() (*Client, error) {
	key := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	if key == "" {
		return nil, nil
	}
	model := strings.TrimSpace(os.Getenv("LLM_MODEL"))
	if model == "" {
		return nil, errors.New("LLM_API_KEY está definida pero falta LLM_MODEL (los nombres de modelo cambian con el tiempo; elige uno de la capa gratuita de tu proveedor)")
	}
	base := strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	if base == "" {
		base = defaultBaseURL
	}
	return newClient(base, key, model), nil
}

func newClient(base, key, model string) *Client {
	return &Client{
		BaseURL: base,
		APIKey:  key,
		Model:   model,
		HTTP:    &http.Client{Timeout: 90 * time.Second},
	}
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete envía la petición y devuelve el texto de la respuesta.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	reqBody := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	}
	reqBody.ResponseFormat.Type = "json_object"
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		// El cuerpo de error no incluye la clave, pero se recorta por si acaso.
		return "", fmt.Errorf("API de IA: HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("API de IA: respuesta ilegible: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("API de IA: respuesta vacía")
	}
	return out.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
