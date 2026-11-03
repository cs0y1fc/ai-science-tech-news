package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCompleteRequestAndResponse(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"items\":[]}"}}]}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL + "/v1/", APIKey: "secreto", Model: "modelo-x", HTTP: srv.Client()}
	out, err := c.Complete(context.Background(), "sistema", "usuario")
	if err != nil || out != `{"items":[]}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if gotPath != "/v1/chat/completions" || gotAuth != "Bearer secreto" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
	if gotBody.Model != "modelo-x" || gotBody.ResponseFormat.Type != "json_object" ||
		len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" || gotBody.Messages[1].Content != "usuario" {
		t.Errorf("cuerpo inesperado: %+v", gotBody)
	}
}

func TestClientRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quota", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: "k", Model: "m", HTTP: srv.Client()}
	if _, err := c.Complete(context.Background(), "s", "u"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, quería ErrRateLimited", err)
	}
}

func TestClientErrors(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"http 500":        {500, "boom"},
		"json roto":       {200, "{no"},
		"sin choices":     {200, `{"choices":[]}`},
		"contenido vacío": {200, `{"choices":[{"message":{"content":"  "}}]}`},
	}
	for name, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			w.Write([]byte(tc.body))
		}))
		c := &Client{BaseURL: srv.URL, APIKey: "k", Model: "m", HTTP: srv.Client()}
		_, err := c.Complete(context.Background(), "s", "u")
		srv.Close()
		if err == nil || errors.Is(err, ErrRateLimited) {
			t.Errorf("%s: esperaba un error normal, got %v", name, err)
		}
	}
}

func TestClientErrorNeverLeaksKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: "SUPER-SECRETA", Model: "m", HTTP: srv.Client()}
	_, err := c.Complete(context.Background(), "s", "u")
	if err == nil || strings.Contains(err.Error(), "SUPER-SECRETA") {
		t.Errorf("el error no debe contener la clave: %v", err)
	}
}

func TestClientFromEnv(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	if c, err := ClientFromEnv(); c != nil || err != nil {
		t.Errorf("sin clave debe devolver nil,nil: %v %v", c, err)
	}

	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_MODEL", "")
	if _, err := ClientFromEnv(); err == nil {
		t.Error("con clave pero sin modelo debe dar error claro")
	}

	t.Setenv("LLM_MODEL", "m")
	c, err := ClientFromEnv()
	if err != nil || c.BaseURL != defaultBaseURL || c.Model != "m" {
		t.Errorf("c=%+v err=%v", c, err)
	}

	t.Setenv("LLM_BASE_URL", "https://api.groq.com/openai/v1")
	if c, _ := ClientFromEnv(); c.BaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("LLM_BASE_URL ignorada: %q", c.BaseURL)
	}
}
