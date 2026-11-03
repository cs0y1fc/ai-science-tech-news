package images

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"noticias-tech/internal/model"
)

func extract(t *testing.T, doc, base string) string {
	t.Helper()
	u, _ := url.Parse(base)
	return ExtractImage(strings.NewReader(doc), u)
}

func TestExtractImagePriorityAndResolution(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"og:image absoluta": {
			`<head><meta property="og:image" content="https://cdn.x.com/a.jpg"></head>`, "https://cdn.x.com/a.jpg"},
		"og gana a twitter": {
			`<head><meta name="twitter:image" content="https://t.com/t.jpg"><meta property="og:image" content="https://o.com/o.jpg"></head>`, "https://o.com/o.jpg"},
		"solo twitter": {
			`<head><meta name="twitter:image" content="https://t.com/t.jpg"></head>`, "https://t.com/t.jpg"},
		"relativa": {
			`<head><meta property="og:image" content="/img/a.png"></head>`, "https://example.com/img/a.png"},
		"protocolo relativo": {
			`<head><meta property="og:image" content="//cdn.example.com/a.png"></head>`, "https://cdn.example.com/a.png"},
		"link image_src": {
			`<head><link rel="image_src" href="https://l.com/l.jpg"></head>`, "https://l.com/l.jpg"},
		"esquema peligroso": {
			`<head><meta property="og:image" content="javascript:alert(1)"></head>`, ""},
		"data uri": {
			`<head><meta property="og:image" content="data:image/png;base64,AAAA"></head>`, ""},
		"vacía": {
			`<head><meta property="og:image" content=""></head>`, ""},
		"sin imagen": {
			`<head><title>x</title></head>`, ""},
		"después del body no cuenta": {
			`<head></head><body><meta property="og:image" content="https://late.com/x.jpg"></body>`, ""},
		"saltar og inválida y usar twitter": {
			`<head><meta property="og:image" content="javascript:x"><meta name="twitter:image" content="https://t.com/t.jpg"></head>`, "https://t.com/t.jpg"},
	}
	for name, c := range cases {
		if got := extract(t, c.doc, "https://example.com/post/1"); got != c.want {
			t.Errorf("%s: got %q, quería %q", name, got, c.want)
		}
	}
}

func TestFillOnlyFetchesWhatIsNeeded(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte(`<head><meta property="og:image" content="/pic.jpg"></head>`))
		case "/none":
			w.Write([]byte(`<head><title>sin imagen</title></head>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	arts := []model.Article{
		{ID: "has", URL: srv.URL + "/ok", ImageURL: "https://x.com/already.jpg"},
		{ID: "checked", URL: srv.URL + "/ok", ImageChecked: true},
		{ID: "ok", URL: srv.URL + "/ok"},
		{ID: "none", URL: srv.URL + "/none"},
		{ID: "gone", URL: srv.URL + "/gone"},
	}
	f := NewFinder()
	f.Client = srv.Client()
	got := f.Fill(context.Background(), arts)

	if hits.Load() != 3 {
		t.Errorf("descargas = %d, quería 3", hits.Load())
	}
	by := map[string]model.Article{}
	for _, a := range got {
		by[a.ID] = a
	}
	if len(by) != 3 {
		t.Fatalf("devueltos = %d, quería 3", len(by))
	}
	if by["ok"].ImageURL != srv.URL+"/pic.jpg" {
		t.Errorf("imagen = %q", by["ok"].ImageURL)
	}
	for _, id := range []string{"ok", "none", "gone"} {
		if !by[id].ImageChecked {
			t.Errorf("%s debe quedar marcado como comprobado, incluso sin imagen", id)
		}
	}
	if by["none"].ImageURL != "" || by["gone"].ImageURL != "" {
		t.Error("sin imagen no debe inventarse ninguna")
	}
}

func TestFillRespectsMaxPerRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<head></head>`))
	}))
	defer srv.Close()
	var arts []model.Article
	for _, id := range []string{"a", "b", "c", "d"} {
		arts = append(arts, model.Article{ID: id, URL: srv.URL + "/" + id})
	}
	f := NewFinder()
	f.Client, f.MaxPerRun = srv.Client(), 2
	got := f.Fill(context.Background(), arts)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("got %+v", got)
	}
}
