package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noticias-tech/internal/model"
	"noticias-tech/internal/rank"
	"noticias-tech/web"
)

var now = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func TestAgo(t *testing.T) {
	cases := map[time.Duration]string{
		10 * time.Second: "just now",
		5 * time.Minute:  "5m ago",
		3 * time.Hour:    "3h ago",
		50 * time.Hour:   "2d ago",
	}
	for d, want := range cases {
		if got := Ago(now.Add(-d), now); got != want {
			t.Errorf("Ago(-%v) = %q, want %q", d, got, want)
		}
	}
}

func fixture() (rank.Selection, []model.Article) {
	feat := model.Article{
		ID: "feat", Title: "Featured <b>dangerous</b>", URL: "https://example.com/f", Source: "Source",
		Section: model.SectionAI, Published: now.Add(-time.Hour), ImageURL: "http://example.com/f.jpg",
		Summary: "AI summary", Excerpt: "feed excerpt", Tags: []string{"llm", "agents"},
	}
	sci := model.Article{
		ID: "sci", Title: "A discovery", URL: "https://example.com/c", Source: "Nature",
		Section: model.SectionScience, Published: now.Add(-3 * time.Hour),
		Excerpt: "Science excerpt", Tags: []string{"space", "physics"},
	}
	bare := model.Article{
		ID: "bare", Title: "Headline only", URL: "https://news.ycombinator.com/x", Source: "Hacker News",
		Section: model.SectionTech, Published: now.Add(-2 * time.Hour), Tags: []string{"web"},
	}
	hidden := model.Article{
		ID: "hidden", Title: "Hidden by AI", URL: "https://example.com/h", Source: "S",
		Section: model.SectionTech, Published: now, Summary: "must not show", Skip: true, Tags: []string{"secret"},
	}
	sel := rank.Selection{Featured: &feat}
	return sel, []model.Article{feat, sci, bare, hidden}
}

func build(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	sel, all := fixture()
	if err := Site(web.FS, out, now, sel, all); err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, out, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, path))
	if err != nil {
		t.Fatalf("missing %s: %v", path, err)
	}
	return string(b)
}

func exists(out, path string) bool {
	_, err := os.Stat(filepath.Join(out, path))
	return err == nil
}

func mustContain(t *testing.T, what, page string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(page, w) {
			t.Errorf("%s: missing %q", what, w)
		}
	}
}

func TestHomepage(t *testing.T) {
	page := read(t, build(t), "index.html")
	mustContain(t, "home", page,
		`<html lang="en">`, "A discovery", "Nature", "3h ago", "Latest news", "Top story",
		`href="static/style.css"`, `<span class="tag">space</span>`, "AI summary",
		`href="n/sci/"`,                   // with excerpt: own page
		`href="s/science/"`,               // section tab
		`aria-current="page">All</a>`,     // "All" tab is the active one
		`href="t/llm/"`,                   // topic
		`src="https://example.com/f.jpg"`, // http -> https
	)
	// Without summary or excerpt the card links straight to the source.
	mustContain(t, "home", page, `href="https://news.ycombinator.com/x" target="_blank"`)
	if strings.Contains(page, `href="n/bare/"`) {
		t.Error("an article with no content must not get its own page")
	}
	if strings.Contains(page, "feed excerpt") {
		t.Error("with an AI summary the feed excerpt must not be shown")
	}
	if strings.Contains(page, "<b>dangerous</b>") {
		t.Error("title not escaped: possible HTML injection")
	}
	if strings.Contains(page, "Hidden by AI") || strings.Contains(page, "secret") {
		t.Error("hidden articles must not appear")
	}
	// Newest first: "Headline only" (2h) comes before "A discovery" (3h).
	if strings.Index(page, "Headline only") > strings.Index(page, "A discovery") {
		t.Error("the feed must be ordered newest first")
	}
	// The featured story is not repeated in the feed.
	if n := strings.Count(page, `href="n/feat/"`); n != 1 {
		t.Errorf("featured story linked %d times, want 1", n)
	}
	// No Spanish left in the UI.
	for _, es := range []string{"Actualizado", "Temas", "Acerca", "Siguiente"} {
		if strings.Contains(page, es) {
			t.Errorf("Spanish text left in the page: %q", es)
		}
	}
}

func TestHomeIsChronologicalAndPaginated(t *testing.T) {
	var all []model.Article
	n := listLimit*2 + 5
	for i := 0; i < n; i++ {
		sec := model.AllSections[i%len(model.AllSections)] // mixed sections
		all = append(all, model.Article{
			ID: fmt.Sprintf("a%03d", i), Title: fmt.Sprintf("Story %03d", i), URL: fmt.Sprintf("https://x.com/%d", i),
			Section: sec, Published: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, all); err != nil {
		t.Fatal(err)
	}
	p1 := read(t, out, "index.html")
	if c := strings.Count(p1, `class="card `); c != listLimit {
		t.Errorf("page 1: %d cards, want %d", c, listLimit)
	}
	mustContain(t, "page 1", p1, "Story 000", `href="page/2/" rel="next"`)
	if strings.Contains(p1, "Story 024") {
		t.Error("page 2 content on page 1")
	}
	// Order is by recency regardless of section.
	if strings.Index(p1, "Story 000") > strings.Index(p1, "Story 001") {
		t.Error("not newest first")
	}

	p2 := read(t, out, "page/2/index.html")
	mustContain(t, "page 2", p2, "Story 024", `href="../../" rel="prev"`, `href="../3/" rel="next"`,
		`href="../../static/style.css"`, "(page 2)")
	if strings.Contains(p2, "Top story") {
		t.Error("the top story only appears on page 1")
	}
	p3 := read(t, out, "page/3/index.html")
	if c := strings.Count(p3, `class="card `); c != 5 {
		t.Errorf("last page: %d cards, want 5", c)
	}
	if exists(out, "page/4/index.html") {
		t.Error("empty extra page")
	}
}

func TestArticlePage(t *testing.T) {
	out := build(t)
	page := read(t, out, "n/feat/index.html")
	mustContain(t, "article", page,
		`href="../../static/style.css"`, `href="../../"`, "AI summary",
		"Automatically generated summary", `href="https://example.com/f"`, "Read the original article at Source",
		`href="../../t/llm/"`, `href="../../s/ai/"`,
		"<title>Featured &lt;b&gt;dangerous&lt;/b&gt; · Tech News</title>",
	)
	if strings.Contains(page, "<b>dangerous</b>") {
		t.Error("unescaped title")
	}

	sci := read(t, out, "n/sci/index.html")
	mustContain(t, "article without AI", sci, "Science excerpt", "Excerpt provided by the source")

	if exists(out, "n/bare/index.html") || exists(out, "n/hidden/index.html") {
		t.Error("no page for content-less or hidden articles")
	}
}

func TestSectionAndTagPages(t *testing.T) {
	out := build(t)

	sec := read(t, out, "s/ai/index.html")
	mustContain(t, "section", sec, "Artificial Intelligence", "Featured", `href="../../n/feat/"`,
		`href="../../static/style.css"`, "Models, research", `href="../../t/llm/"`, `aria-current="page">AI</a>`)
	if strings.Contains(sec, `class="pager"`) {
		t.Error("a single page has no pager")
	}

	tag := read(t, out, "t/space/index.html")
	mustContain(t, "tag", tag, "#space", "A discovery")
	if strings.Contains(tag, "Featured") {
		t.Error("a tag page lists only its own articles")
	}
	if exists(out, "t/secret/index.html") {
		t.Error("tags of hidden articles must not get a page")
	}
	if exists(out, "s/tutorials/index.html") {
		t.Error("an empty section must not get a page")
	}
}

func TestStaticAssetsCopied(t *testing.T) {
	if !exists(build(t), "static/style.css") {
		t.Error("CSS not copied")
	}
}

func TestEmptySiteShowsMessage(t *testing.T) {
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, nil); err != nil {
		t.Fatal(err)
	}
	mustContain(t, "empty home", read(t, out, "index.html"), "No news yet")
}

func TestSectionListPagination(t *testing.T) {
	var all []model.Article
	for i := 0; i < listLimit*2+5; i++ {
		all = append(all, model.Article{
			ID: fmt.Sprintf("a%02d", i), Title: "T", URL: fmt.Sprintf("https://x.com/%d", i),
			Section: model.SectionAI, Published: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, all); err != nil {
		t.Fatal(err)
	}
	p1 := read(t, out, "s/ai/index.html")
	if n := strings.Count(p1, `class="card `); n != listLimit {
		t.Errorf("page 1: %d cards, want %d", n, listLimit)
	}
	mustContain(t, "page 1", p1, `href="page/2/" rel="next"`, `aria-current="page"`)
	if strings.Contains(p1, `rel="prev"`) {
		t.Error("the first page has no previous")
	}
	p2 := read(t, out, "s/ai/page/2/index.html")
	mustContain(t, "page 2", p2, `href="../../" rel="prev"`, `href="../3/" rel="next"`,
		`href="../../../../static/style.css"`, "(page 2)")
	p3 := read(t, out, "s/ai/page/3/index.html")
	if n := strings.Count(p3, `class="card `); n != 5 {
		t.Errorf("last page: %d cards, want 5", n)
	}
	if strings.Contains(p3, `rel="next"`) {
		t.Error("the last page has no next")
	}
}

func TestTopChipsOrder(t *testing.T) {
	arts := []model.Article{
		{Tags: []string{"b", "a"}}, {Tags: []string{"b"}}, {Tags: []string{"c"}},
	}
	got := topChips("", arts, 2)
	if len(got) != 2 || got[0].Tag != "b" || got[0].Count != 2 || got[1].Tag != "a" {
		t.Errorf("got %+v", got)
	}
}

func TestSecureURL(t *testing.T) {
	if secureURL("http://a.com/x.jpg") != "https://a.com/x.jpg" || secureURL("https://a.com/x.jpg") != "https://a.com/x.jpg" {
		t.Error("secureURL incorrecto")
	}
}

func buildWith(t *testing.T, opts Options) string {
	t.Helper()
	out := t.TempDir()
	sel, all := fixture()
	if err := SiteWith(opts, web.FS, out, now, sel, all); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFeedAndSitemap(t *testing.T) {
	out := buildWith(t, Options{SiteURL: "https://u.github.io/repo/", Sources: []string{"Nature", "Ars"}})

	feed := read(t, out, "feed.xml")
	mustContain(t, "feed", feed,
		`<rss version="2.0"`, "https://u.github.io/repo/n/feat/", "AI summary",
		"Featured &lt;b&gt;dangerous&lt;/b&gt;", // escaped title
		"https://news.ycombinator.com/x",        // sin página propia: enlaza a la fuente
		"<category>llm</category>",
	)
	if strings.Contains(feed, "Hidden by AI") {
		t.Error("el feed no debe incluir artículos ocultos")
	}

	sm := read(t, out, "sitemap.xml")
	mustContain(t, "sitemap", sm,
		"<loc>https://u.github.io/repo/</loc>", "<loc>https://u.github.io/repo/about/</loc>",
		"<loc>https://u.github.io/repo/n/feat/</loc>", "<loc>https://u.github.io/repo/t/llm/</loc>",
		"<loc>https://u.github.io/repo/s/ai/</loc>",
	)
	for _, no := range []string{"/n/bare/", "/n/hidden/", "/t/secret/", "/s/tutorials/"} {
		if strings.Contains(sm, no) {
			t.Errorf("el sitemap no debe incluir %s", no)
		}
	}

	home := read(t, out, "index.html")
	mustContain(t, "portada", home, `rel="canonical" href="https://u.github.io/repo/"`, `href="feed.xml"`)
	mustContain(t, "artículo", read(t, out, "n/feat/index.html"), `rel="canonical" href="https://u.github.io/repo/n/feat/"`)
}

func TestAboutPageAndNoFeedWithoutSiteURL(t *testing.T) {
	out := buildWith(t, Options{Sources: []string{"Nature", "Ars"}, RepoURL: "https://github.com/u/r"})
	about := read(t, out, "about/index.html")
	mustContain(t, "acerca de", about, "About", "Nature · Ars", `href="https://github.com/u/r"`, `href="../static/style.css"`)
	if strings.Contains(about, "feed.xml") {
		t.Error("sin SiteURL no debe enlazarse un feed inexistente")
	}
	if exists(out, "feed.xml") || exists(out, "sitemap.xml") {
		t.Error("sin SiteURL no se generan feed ni sitemap")
	}
}

func TestHomeStopsAtMaxPages(t *testing.T) {
	var all []model.Article
	for i := 0; i < listLimit*(homeMaxPages+2); i++ {
		all = append(all, model.Article{
			ID: fmt.Sprintf("a%03d", i), Title: "T", URL: fmt.Sprintf("https://x.com/%d", i),
			Section: model.SectionTech, Published: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, all); err != nil {
		t.Fatal(err)
	}
	last := read(t, out, fmt.Sprintf("page/%d/index.html", homeMaxPages))
	if strings.Contains(last, `rel="next"`) {
		t.Error("the last home page must not link onwards")
	}
	if strings.Contains(read(t, out, "index.html"), fmt.Sprintf(`>%d</a>`, homeMaxPages+1)) {
		t.Errorf("home must not link to page %d", homeMaxPages+1)
	}
	if exists(out, fmt.Sprintf("page/%d/index.html", homeMaxPages+1)) {
		t.Errorf("page %d must not exist", homeMaxPages+1)
	}
	// Sections are not capped.
	if !exists(out, "s/tech/page/6/index.html") {
		t.Error("section pages keep full pagination")
	}
}

func TestAuthHiddenWithoutSupabase(t *testing.T) {
	out := build(t)
	page := read(t, out, "index.html")
	for _, no := range []string{`id="auth"`, "auth.js", "authdlg", "Sign in"} {
		if strings.Contains(page, no) {
			t.Errorf("home without Supabase must not contain %q", no)
		}
	}
	if exists(out, "saved/index.html") {
		t.Error("saved page must not be generated without Supabase")
	}
}

func TestAuthAndSavedWithSupabase(t *testing.T) {
	out := buildWith(t, Options{SupabaseURL: "https://x.supabase.co", SupabaseKey: "anon-public"})
	page := read(t, out, "index.html")
	mustContain(t, "home", page,
		`data-auth-open`, `id="authdlg"`, `data-provider="google"`, `static/auth.js`,
		`data-supabase-url="https://x.supabase.co"`, `data-supabase-key="anon-public"`,
		`data-id="`, `data-url="`)
	mustContain(t, "saved", read(t, out, "saved/index.html"), "Saved stories", `id="saved-grid"`)
	if !exists(out, "static/auth.js") {
		t.Error("auth.js not copied")
	}
	art := read(t, out, "n/"+firstArticleDir(t, out)+"/index.html")
	mustContain(t, "article", art, `data-fav-article`)
}

func firstArticleDir(t *testing.T, out string) string {
	t.Helper()
	es, err := os.ReadDir(filepath.Join(out, "n"))
	if err != nil || len(es) == 0 {
		t.Fatalf("no article pages: %v", err)
	}
	return es[0].Name()
}

func TestFooterAndTagSearch(t *testing.T) {
	out := build(t)
	page := read(t, out, "index.html")
	mustContain(t, "home", page, "© honolulu &amp; bcn", `id="search"`, `static/search.js`, `aria-label="Search tags"`)
	for _, no := range []string{"Summaries are generated", ">RSS<"} {
		if strings.Contains(page, no) {
			t.Errorf("footer must not contain %q", no)
		}
	}
	tags := read(t, out, "tags.json")
	mustContain(t, "tags.json", tags, `"t":"llm"`, `"n":`)
	if !exists(out, "static/search.js") {
		t.Error("search.js not copied")
	}
	mustContain(t, "article page search", read(t, out, "n/feat/index.html"), `data-base="../../"`)
}
