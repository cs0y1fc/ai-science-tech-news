// Package images busca la imagen principal de los artículos que no la traen
// en su feed, leyendo las metaetiquetas de la página (og:image y similares).
package images

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"noticias-tech/internal/model"
)

// maxHeadBytes basta de sobra para llegar al final del <head>.
const maxHeadBytes = 512 << 10

// Finder descarga páginas y extrae su imagen principal.
type Finder struct {
	Client    *http.Client
	UserAgent string
	// MaxPerRun limita las páginas descargadas por ejecución (por defecto 60).
	MaxPerRun int
	// Workers son las descargas simultáneas (por defecto 8).
	Workers int
}

// NewFinder devuelve un Finder con valores por defecto.
func NewFinder() *Finder {
	return &Finder{
		Client:    &http.Client{Timeout: 15 * time.Second},
		UserAgent: "noticias-tech/0.1 (+news aggregator; personal project)",
		MaxPerRun: 60,
		Workers:   8,
	}
}

// Fill busca imagen para los artículos que no tienen y aún no se han
// comprobado, como mucho MaxPerRun y en el orden dado. Devuelve solo los
// artículos modificados, ya marcados como comprobados.
func (f *Finder) Fill(ctx context.Context, arts []model.Article) []model.Article {
	var todo []model.Article
	for _, a := range arts {
		if a.ImageURL == "" && !a.ImageChecked {
			todo = append(todo, a)
			if len(todo) == f.MaxPerRun {
				break
			}
		}
	}

	out := make([]model.Article, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(f.Workers, 1))
	for i, a := range todo {
		wg.Add(1)
		go func(i int, a model.Article) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			a.ImageURL = f.find(ctx, a.URL)
			a.ImageChecked = true
			out[i] = a
		}(i, a)
	}
	wg.Wait()
	return out
}

func (f *Finder) find(ctx context.Context, rawURL string) string {
	base, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	return ExtractImage(io.LimitReader(resp.Body, maxHeadBytes), base)
}

// ExtractImage lee el <head> de un documento HTML y devuelve la imagen
// principal como URL absoluta http(s), o "" si no hay ninguna válida.
// Prioridad: og:image, twitter:image, <link rel="image_src">.
func ExtractImage(r io.Reader, base *url.URL) string {
	found := map[string]string{}
	z := html.NewTokenizer(r)
loop:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break loop
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			switch tok.Data {
			case "body":
				break loop
			case "meta":
				key := strings.ToLower(attr(tok, "property"))
				if key == "" {
					key = strings.ToLower(attr(tok, "name"))
				}
				if v := attr(tok, "content"); v != "" && found[key] == "" {
					found[key] = v
				}
			case "link":
				if strings.EqualFold(attr(tok, "rel"), "image_src") {
					if v := attr(tok, "href"); v != "" && found["image_src"] == "" {
						found["image_src"] = v
					}
				}
			}
		}
	}
	for _, key := range []string{"og:image:secure_url", "og:image", "og:image:url", "twitter:image", "twitter:image:src", "image_src"} {
		if u := resolve(found[key], base); u != "" {
			return u
		}
	}
	return ""
}

func attr(t html.Token, name string) string {
	for _, a := range t.Attr {
		if a.Key == name {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

// resolve convierte ref en una URL absoluta http(s) respecto a base.
func resolve(ref string, base *url.URL) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}
