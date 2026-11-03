package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const articleHTML = `<!doctype html><html><head><title>Un gran artículo</title>
<meta property="og:image" content="https://example.com/portada.jpg"></head>
<body><nav>menú menú menú</nav>
<article><h1>Un gran artículo</h1>
<p>Este es el primer párrafo del artículo, con bastante texto para que la extracción lo considere contenido principal y no ruido de la página.</p>
<p>Este es el segundo párrafo, que también aporta contenido relevante y suficiente longitud para superar los umbrales de legibilidad del extractor.</p>
<p>Y un tercer párrafo para asegurar que el algoritmo identifica esta sección como el cuerpo del artículo y descarta el resto de elementos.</p>
</article><footer>pie de página</footer></body></html>`

func TestHTTPPagesExtractsTextAndImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("falta User-Agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	p, err := NewHTTPPages().Page(context.Background(), srv.URL+"/post")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Text, "primer párrafo") || strings.Contains(p.Text, "pie de página") {
		t.Errorf("texto mal extraído: %q", p.Text)
	}
	if p.Image != "https://example.com/portada.jpg" {
		t.Errorf("imagen = %q", p.Image)
	}
}

func TestHTTPPagesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := NewHTTPPages().Page(context.Background(), srv.URL); err == nil {
		t.Error("esperaba error por HTTP 403")
	}
}
