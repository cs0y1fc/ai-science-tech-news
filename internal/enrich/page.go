package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-shiori/go-readability"
)

// Page es lo que se extrae de la página de un artículo.
type Page struct {
	Text  string
	Image string
}

// PageSource descarga y limpia la página de un artículo.
type PageSource interface {
	Page(ctx context.Context, rawURL string) (Page, error)
}

// HTTPPages implementa PageSource con net/http y go-readability.
type HTTPPages struct {
	Client    *http.Client
	UserAgent string
}

// NewHTTPPages devuelve un PageSource con timeout y User-Agent propios.
func NewHTTPPages() *HTTPPages {
	return &HTTPPages{
		Client:    &http.Client{Timeout: 20 * time.Second},
		UserAgent: "noticias-tech/0.1 (+news aggregator; personal project)",
	}
}

const maxPageBytes = 2 << 20

// Page descarga la URL y extrae el texto principal y la imagen destacada.
func (h *HTTPPages) Page(ctx context.Context, rawURL string) (Page, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Page{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", h.UserAgent)
	resp, err := h.Client.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Page{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	art, err := readability.FromReader(io.LimitReader(resp.Body, maxPageBytes), u)
	if err != nil {
		return Page{}, err
	}
	return Page{Text: art.TextContent, Image: art.Image}, nil
}
