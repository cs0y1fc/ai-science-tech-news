package render

import (
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
		10 * time.Second: "ahora mismo",
		5 * time.Minute:  "hace 5 min",
		3 * time.Hour:    "hace 3 h",
		50 * time.Hour:   "hace 2 d",
	}
	for d, want := range cases {
		if got := Ago(now.Add(-d), now); got != want {
			t.Errorf("Ago(-%v) = %q, quería %q", d, got, want)
		}
	}
}

func fixture() (rank.Selection, []model.Article) {
	feat := model.Article{
		ID: "feat", Title: "Destacada <b>peligrosa</b>", URL: "https://example.com/f", Source: "Fuente",
		Section: model.SectionAI, Published: now.Add(-time.Hour), ImageURL: "http://example.com/f.jpg",
		Summary: "Resumen de la IA", Excerpt: "extracto del feed", Tags: []string{"llm", "agents"},
	}
	sci := model.Article{
		ID: "sci", Title: "Un hallazgo", URL: "https://example.com/c", Source: "Nature",
		Section: model.SectionScience, Published: now.Add(-3 * time.Hour),
		Excerpt: "Extracto de ciencia", Tags: []string{"space", "physics"},
	}
	bare := model.Article{
		ID: "bare", Title: "Solo titular", URL: "https://news.ycombinator.com/x", Source: "Hacker News",
		Section: model.SectionTech, Published: now.Add(-2 * time.Hour), Tags: []string{"web"},
	}
	hidden := model.Article{
		ID: "hidden", Title: "Oculto por la IA", URL: "https://example.com/h", Source: "S",
		Section: model.SectionTech, Published: now, Summary: "no debe verse", Skip: true, Tags: []string{"secreta"},
	}
	sel := rank.Selection{
		Featured: &feat,
		Rows: []rank.Row{
			{Section: model.SectionScience, Articles: []model.Article{sci}},
			{Section: model.SectionTech, Articles: []model.Article{bare}},
		},
	}
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
		t.Fatalf("falta %s: %v", path, err)
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
			t.Errorf("%s: falta %q", what, w)
		}
	}
}

func TestHomepage(t *testing.T) {
	page := read(t, build(t), "index.html")
	mustContain(t, "portada", page,
		"Un hallazgo", "Nature", "hace 3 h", `id="ciencia"`, "Actualizado ahora mismo",
		`href="static/style.css"`, `<span class="tag">space</span>`, "Resumen de la IA",
		`href="n/sci/"`,                   // con extracto: página propia
		`href="s/ciencia/"`,               // título de la fila enlaza a la sección
		`href="t/llm/"`,                   // tema en la portada
		`src="https://example.com/f.jpg"`, // http -> https
	)
	// Sin resumen ni extracto, la tarjeta va directa a la fuente.
	mustContain(t, "portada", page, `href="https://news.ycombinator.com/x" target="_blank"`)
	if strings.Contains(page, `href="n/bare/"`) {
		t.Error("un artículo sin contenido no debe tener página propia")
	}
	if strings.Contains(page, "extracto del feed") {
		t.Error("con resumen de la IA no debe mostrarse el extracto del feed")
	}
	if strings.Contains(page, "<b>peligrosa</b>") {
		t.Error("el título no se escapó: posible inyección de HTML")
	}
	if strings.Contains(page, "Oculto por la IA") || strings.Contains(page, "secreta") {
		t.Error("los artículos ocultos no deben aparecer")
	}
	if strings.Contains(page, "http://example.com/f.jpg") {
		t.Error("las imágenes deben servirse por https")
	}
}

func TestArticlePage(t *testing.T) {
	out := build(t)
	page := read(t, out, "n/feat/index.html")
	mustContain(t, "artículo", page,
		`href="../../static/style.css"`, `href="../../"`, "Resumen de la IA",
		"Resumen generado automáticamente", `href="https://example.com/f"`, "Leer el artículo original en Fuente",
		`href="../../t/llm/"`, `href="../../s/ia/"`,
		"<title>Destacada &lt;b&gt;peligrosa&lt;/b&gt; · Noticias Tech</title>",
	)
	if strings.Contains(page, "<b>peligrosa</b>") {
		t.Error("título sin escapar")
	}

	// Sin resumen de IA se muestra el extracto, indicándolo.
	sci := read(t, out, "n/sci/index.html")
	mustContain(t, "artículo sin IA", sci, "Extracto de ciencia", "Extracto proporcionado por la fuente")

	if exists(out, "n/bare/index.html") || exists(out, "n/hidden/index.html") {
		t.Error("no debe haber página para artículos sin contenido ni ocultos")
	}
}

func TestSectionAndTagPages(t *testing.T) {
	out := build(t)

	sec := read(t, out, "s/ia/index.html")
	mustContain(t, "sección", sec, "Inteligencia artificial", "Destacada", `href="../../n/feat/"`, `href="../../static/style.css"`)

	tag := read(t, out, "t/space/index.html")
	mustContain(t, "etiqueta", tag, "#space", "Un hallazgo")
	if strings.Contains(tag, "Destacada") {
		t.Error("la página de una etiqueta solo debe listar sus artículos")
	}

	if exists(out, "t/secreta/index.html") {
		t.Error("las etiquetas de artículos ocultos no deben generar página")
	}
	if exists(out, "s/tutoriales/index.html") {
		t.Error("una sección sin artículos no debe generar página")
	}
}

func TestStaticAssetsCopied(t *testing.T) {
	if !exists(build(t), "static/style.css") {
		t.Error("no se copió el CSS")
	}
}

func TestEmptySiteShowsMessage(t *testing.T) {
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, nil); err != nil {
		t.Fatal(err)
	}
	mustContain(t, "portada vacía", read(t, out, "index.html"), "Todavía no hay noticias")
}

func TestListLimit(t *testing.T) {
	var all []model.Article
	for i := 0; i < listLimit+15; i++ {
		all = append(all, model.Article{
			ID: string(rune('a'+i%26)) + string(rune('a'+i/26)), Title: "T", URL: "https://x.com/" + string(rune('a'+i)),
			Section: model.SectionAI, Published: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	out := t.TempDir()
	if err := Site(web.FS, out, now, rank.Selection{}, all); err != nil {
		t.Fatal(err)
	}
	page := read(t, out, "s/ia/index.html")
	if n := strings.Count(page, `class="card"`); n != listLimit {
		t.Errorf("tarjetas = %d, quería %d", n, listLimit)
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
		`<rss version="2.0"`, "https://u.github.io/repo/n/feat/", "Resumen de la IA",
		"Destacada &lt;b&gt;peligrosa&lt;/b&gt;", // título escapado
		"https://news.ycombinator.com/x",         // sin página propia: enlaza a la fuente
		"<category>llm</category>",
	)
	if strings.Contains(feed, "Oculto por la IA") {
		t.Error("el feed no debe incluir artículos ocultos")
	}

	sm := read(t, out, "sitemap.xml")
	mustContain(t, "sitemap", sm,
		"<loc>https://u.github.io/repo/</loc>", "<loc>https://u.github.io/repo/about/</loc>",
		"<loc>https://u.github.io/repo/n/feat/</loc>", "<loc>https://u.github.io/repo/t/llm/</loc>",
		"<loc>https://u.github.io/repo/s/ia/</loc>",
	)
	for _, no := range []string{"/n/bare/", "/n/hidden/", "/t/secreta/", "/s/tutoriales/"} {
		if strings.Contains(sm, no) {
			t.Errorf("el sitemap no debe incluir %s", no)
		}
	}

	home := read(t, out, "index.html")
	mustContain(t, "portada", home, `rel="canonical" href="https://u.github.io/repo/"`, `href="feed.xml"`, `href="about/"`)
	mustContain(t, "artículo", read(t, out, "n/feat/index.html"), `rel="canonical" href="https://u.github.io/repo/n/feat/"`)
}

func TestAboutPageAndNoFeedWithoutSiteURL(t *testing.T) {
	out := buildWith(t, Options{Sources: []string{"Nature", "Ars"}, RepoURL: "https://github.com/u/r"})
	about := read(t, out, "about/index.html")
	mustContain(t, "acerca de", about, "Acerca de", "Nature · Ars", `href="https://github.com/u/r"`, `href="../static/style.css"`)
	if strings.Contains(about, "feed.xml") {
		t.Error("sin SiteURL no debe enlazarse un feed inexistente")
	}
	if exists(out, "feed.xml") || exists(out, "sitemap.xml") {
		t.Error("sin SiteURL no se generan feed ni sitemap")
	}
}
