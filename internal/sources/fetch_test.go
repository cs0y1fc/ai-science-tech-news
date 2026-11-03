package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"noticias-tech/internal/model"
)

const rssFixture = `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
<channel><title>Demo</title>
  <item>
    <title>Modelo &amp; datos nuevos</title>
    <link>https://example.com/a?utm_source=rss</link>
    <pubDate>Mon, 02 Nov 2026 08:00:00 GMT</pubDate>
    <description>&lt;p&gt;Hola &lt;b&gt;mundo&lt;/b&gt; &amp;amp; adiós&lt;/p&gt;</description>
    <media:content url="https://example.com/img.jpg" medium="image"/>
  </item>
  <item>
    <title>Con enclosure</title>
    <link>https://example.com/b</link>
    <pubDate>Mon, 02 Nov 2026 07:00:00 GMT</pubDate>
    <enclosure url="https://example.com/enc.png" type="image/png" length="1"/>
  </item>
  <item><title>Sin enlace</title></item>
</channel></rss>`

func newTestFetcher() *Fetcher {
	f := NewFetcher()
	f.Client = http.DefaultClient
	return f
}

func TestFetchRSS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("falta User-Agent")
		}
		w.Write([]byte(rssFixture))
	}))
	defer srv.Close()

	got, err := newTestFetcher().Fetch(context.Background(), Source{
		Name: "Demo", URL: srv.URL, Section: model.SectionTech, Type: "rss",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, quería 2 (el ítem sin enlace se descarta)", len(got))
	}
	a := got[0]
	if a.Title != "Modelo & datos nuevos" || a.Source != "Demo" || a.Section != model.SectionTech {
		t.Errorf("campos incorrectos: %+v", a)
	}
	if a.Excerpt != "Hola mundo & adiós" {
		t.Errorf("extracto = %q", a.Excerpt)
	}
	if a.ImageURL != "https://example.com/img.jpg" {
		t.Errorf("imagen media:content = %q", a.ImageURL)
	}
	if got[1].ImageURL != "https://example.com/enc.png" {
		t.Errorf("imagen enclosure = %q", got[1].ImageURL)
	}
	if a.ID == "" || a.ID == got[1].ID {
		t.Errorf("IDs inválidos: %q %q", a.ID, got[1].ID)
	}
	if a.Published.IsZero() || a.Published.Year() != 2026 {
		t.Errorf("fecha = %v", a.Published)
	}
}

func TestFetchRSSLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(rssFixture))
	}))
	defer srv.Close()
	got, err := newTestFetcher().Fetch(context.Background(), Source{Name: "D", URL: srv.URL, Section: model.SectionAI, Limit: 1})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %d, err %v", len(got), err)
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), Source{Name: "D", URL: srv.URL, Section: model.SectionAI}); err == nil {
		t.Error("esperaba error por HTTP 500")
	}
}

func TestFetchInvalidFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>no soy un feed</html>"))
	}))
	defer srv.Close()
	if _, err := newTestFetcher().Fetch(context.Background(), Source{Name: "D", URL: srv.URL, Section: model.SectionAI}); err == nil {
		t.Error("esperaba error por feed inválido")
	}
}

func TestFetchHN(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/topstories.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[1,2,3,4]`))
	})
	mux.HandleFunc("/item/1.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"story","title":"Historia buena","url":"https://example.com/hn1","score":250,"time":1793600000}`))
	})
	mux.HandleFunc("/item/2.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"story","title":"Ask HN: sin url","score":10,"time":1793600000}`))
	})
	mux.HandleFunc("/item/3.json", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/item/4.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"job","title":"Trabajo","url":"https://example.com/job","score":0,"time":1793600000}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := newTestFetcher().Fetch(context.Background(), Source{
		Name: "Hacker News", URL: srv.URL + "/", Section: model.SectionAI, Type: "hn",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, quería 1: %+v", len(got), got)
	}
	if got[0].Points != 250 || got[0].Title != "Historia buena" || !strings.HasSuffix(got[0].URL, "/hn1") {
		t.Errorf("artículo HN incorrecto: %+v", got[0])
	}
}

func TestFetchAllToleratesFailures(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(rssFixture))
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "caído", http.StatusBadGateway)
	}))
	defer bad.Close()

	arts, errs := newTestFetcher().FetchAll(context.Background(), []Source{
		{Name: "Bueno", URL: good.URL, Section: model.SectionTech},
		{Name: "Malo", URL: bad.URL, Section: model.SectionTech},
	})
	if len(arts) != 2 {
		t.Errorf("artículos = %d, quería 2", len(arts))
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "Malo") {
		t.Errorf("errores = %v", errs)
	}
}

func TestExcerptTruncates(t *testing.T) {
	long := strings.Repeat("palabra ", 100)
	got := excerpt("<p>" + long + "</p>")
	if r := []rune(got); len(r) > maxExcerpt+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("no se recortó bien (%d runas)", len(r))
	}
}
