package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"noticias-tech/internal/dedupe"
	"noticias-tech/internal/model"
)

const (
	defaultLimit   = 20
	defaultHNLimit = 30
	maxBodyBytes   = 5 << 20
	maxExcerpt     = 400
)

// Fetcher descarga artículos de las fuentes configuradas.
type Fetcher struct {
	Client    *http.Client
	UserAgent string
}

// NewFetcher devuelve un Fetcher con timeout y User-Agent propios.
func NewFetcher() *Fetcher {
	return &Fetcher{
		Client:    &http.Client{Timeout: 20 * time.Second},
		UserAgent: "noticias-tech/0.1 (+news aggregator; personal project)",
	}
}

// FetchAll recoge todas las fuentes en paralelo. Una fuente que falla no
// afecta a las demás: sus errores se devuelven aparte.
func (f *Fetcher) FetchAll(ctx context.Context, srcs []Source) ([]model.Article, []error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		articles []model.Article
		errs     []error
	)
	for _, s := range srcs {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			got, err := f.Fetch(ctx, s)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
				return
			}
			articles = append(articles, got...)
		}(s)
	}
	wg.Wait()
	return articles, errs
}

// Fetch recoge una sola fuente según su tipo.
func (f *Fetcher) Fetch(ctx context.Context, s Source) ([]model.Article, error) {
	switch s.Type {
	case "hn":
		return f.fetchHN(ctx, s)
	default:
		return f.fetchRSS(ctx, s)
	}
}

func (f *Fetcher) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d en %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}

func (f *Fetcher) fetchRSS(ctx context.Context, s Source) ([]model.Article, error) {
	body, err := f.get(ctx, s.URL)
	if err != nil {
		return nil, err
	}
	feed, err := gofeed.NewParser().ParseString(string(body))
	if err != nil {
		return nil, fmt.Errorf("feed inválido: %w", err)
	}
	limit := s.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	var out []model.Article
	for _, it := range feed.Items {
		if len(out) >= limit {
			break
		}
		if it.Link == "" || it.Title == "" {
			continue
		}
		out = append(out, model.Article{
			ID:        dedupe.ID(it.Link),
			Title:     strings.TrimSpace(html.UnescapeString(it.Title)),
			URL:       it.Link,
			Source:    s.Name,
			Section:   s.Section,
			Weight:    s.Weight,
			Published: publishedAt(it),
			Excerpt:   excerpt(firstNonEmpty(it.Description, it.Content)),
			ImageURL:  imageURL(it),
		})
	}
	return out, nil
}

type hnItem struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Score int    `json:"score"`
	Time  int64  `json:"time"`
	Dead  bool   `json:"dead"`
}

// fetchHN lee las historias más votadas de Hacker News. s.URL es la base de
// la API (https://hacker-news.firebaseio.com/v0).
func (f *Fetcher) fetchHN(ctx context.Context, s Source) ([]model.Article, error) {
	base := strings.TrimRight(s.URL, "/")
	body, err := f.get(ctx, base+"/topstories.json")
	if err != nil {
		return nil, err
	}
	var ids []int
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("topstories inválido: %w", err)
	}
	limit := s.Limit
	if limit == 0 {
		limit = defaultHNLimit
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}

	items := make([]*hnItem, len(ids))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, id := range ids {
		wg.Add(1)
		go func(i, id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, err := f.get(ctx, fmt.Sprintf("%s/item/%d.json", base, id))
			if err != nil {
				return
			}
			var it hnItem
			if json.Unmarshal(b, &it) == nil {
				items[i] = &it
			}
		}(i, id)
	}
	wg.Wait()

	var out []model.Article
	for _, it := range items {
		// Las historias sin URL (Ask HN, etc.) no tienen artículo que resumir.
		if it == nil || it.Dead || it.Type != "story" || it.URL == "" || it.Title == "" {
			continue
		}
		out = append(out, model.Article{
			ID:        dedupe.ID(it.URL),
			Title:     it.Title,
			URL:       it.URL,
			Source:    s.Name,
			Section:   s.Section,
			Weight:    s.Weight,
			Published: time.Unix(it.Time, 0).UTC(),
			Points:    it.Score,
		})
	}
	return out, nil
}

func publishedAt(it *gofeed.Item) time.Time {
	switch {
	case it.PublishedParsed != nil:
		return it.PublishedParsed.UTC()
	case it.UpdatedParsed != nil:
		return it.UpdatedParsed.UTC()
	}
	return time.Now().UTC()
}

func imageURL(it *gofeed.Item) string {
	if it.Image != nil && it.Image.URL != "" {
		return it.Image.URL
	}
	for _, e := range it.Enclosures {
		if strings.HasPrefix(e.Type, "image/") && e.URL != "" {
			return e.URL
		}
	}
	for _, name := range []string{"content", "thumbnail"} {
		for _, ext := range it.Extensions["media"][name] {
			if u := ext.Attrs["url"]; u != "" && (ext.Attrs["medium"] == "" || ext.Attrs["medium"] == "image" || strings.HasPrefix(ext.Attrs["type"], "image/") || name == "thumbnail") {
				return u
			}
		}
	}
	return ""
}

var (
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// excerpt quita el HTML y recorta el texto a un tamaño razonable.
func excerpt(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	r := []rune(s)
	if len(r) > maxExcerpt {
		return strings.TrimSpace(string(r[:maxExcerpt])) + "…"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
