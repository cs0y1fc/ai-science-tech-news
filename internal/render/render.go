package render

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"noticias-tech/internal/model"
	"noticias-tech/internal/rank"
)

const (
	siteName = "Tech News"
	// listLimit son las tarjetas por página en una sección o etiqueta.
	listLimit = 24
	// topicCount son las etiquetas que se muestran en la portada.
	topicCount = 12
	// relatedCount son las noticias relacionadas al pie de un artículo.
	relatedCount = 4
)

// Card es un artículo listo para pintar como tarjeta.
type Card struct {
	Article model.Article
	// Href es el destino de la tarjeta: la página propia si el artículo tiene
	// contenido, o la fuente original si no.
	Href     string
	External bool
}

// NavItem es un enlace del menú.
type NavItem struct {
	Label, Href string
}

// Tab es una pestaña de filtro por sección sobre la lista de noticias.
type Tab struct {
	Label, Href string
	Current     bool
}

// Chip es una etiqueta con su recuento.
type Chip struct {
	Tag   string
	Count int
	Href  string
}

// PageLink es un enlace numerado del paginador.
type PageLink struct {
	N       int
	Href    string
	Current bool
}

// Pager es la navegación entre páginas de una lista.
type Pager struct {
	Prev, Next string
	Links      []PageLink
}

// blurbs describe cada sección bajo su título.
var blurbs = map[model.Section]string{
	model.SectionAI:        "Models, research, products and debates in artificial intelligence.",
	model.SectionScience:   "Discoveries and advances in physics, space, biology and more.",
	model.SectionTech:      "Hardware, software, security and the technology industry.",
	model.SectionCoding:    "Programming languages, developer tools, software engineering and open-source releases.",
	model.SectionPhones:    "Smartphones, Android and iOS, mobile apps and the devices in your pocket.",
	model.SectionTutorials: "Hands-on guides to programming and to building apps with AI.",
}

// Page reúne los datos de cualquier página; cada plantilla usa los que necesita.
type Page struct {
	SiteName    string
	Title       string
	Description string
	// Base es el prefijo relativo hacia la raíz del sitio ("" en la portada,
	// "../../" en las demás), para que funcione bajo una subruta como
	// user.github.io/repo/.
	Base    string
	Updated time.Time
	Nav     []NavItem

	// Portada y listas.
	Featured *Card
	Topics   []Chip
	Tabs     []Tab

	// Listas (sección o etiqueta).
	Heading string
	Blurb   string
	Cards   []Card
	Pager   *Pager

	// Artículo.
	Article *model.Article
	Section NavItem
	Tags    []Chip
	Related []Card

	// Canonical es la URL absoluta de la página (solo si hay SiteURL).
	Canonical string
	// HasFeed indica que el sitio publica feed.xml.
	HasFeed bool
	// Acerca de.
	Sources []string
	RepoURL string
}

// Options son los datos opcionales del despliegue. Sin SiteURL no se generan
// feed.xml ni sitemap.xml, que necesitan URLs absolutas.
type Options struct {
	// SiteURL es la URL pública de la raíz del sitio, p. ej.
	// https://usuario.github.io/repo
	SiteURL string
	// RepoURL enlaza al código fuente desde la página "Acerca de".
	RepoURL string
	// Sources son los nombres de las fuentes que se muestran en "Acerca de".
	Sources []string
}

type renderer struct {
	tpl     *template.Template
	outDir  string
	now     time.Time
	updated time.Time
	opts    Options
	all     []model.Article // artículos visibles, para los temas de la portada
}

// Site genera el sitio estático en outDir a partir de las plantillas y
// estáticos de fsys (que debe contener templates/ y static/). sel es la
// selección de la portada y all todos los artículos guardados.
func Site(fsys fs.FS, outDir string, now time.Time, sel rank.Selection, all []model.Article) error {
	return SiteWith(Options{}, fsys, outDir, now, sel, all)
}

// SiteWith es Site con opciones de despliegue (URL pública, fuentes, código).
func SiteWith(opts Options, fsys fs.FS, outDir string, now time.Time, sel rank.Selection, all []model.Article) error {
	opts.SiteURL = strings.TrimRight(opts.SiteURL, "/")
	funcs := template.FuncMap{
		"ago": func(t time.Time) string { return Ago(t, now) },
		"img": secureURL,
	}
	tpl, err := template.New("").Funcs(funcs).ParseFS(fsys, "templates/*.html")
	if err != nil {
		return fmt.Errorf("plantillas: %w", err)
	}
	r := &renderer{tpl: tpl, outDir: outDir, now: now, updated: now, opts: opts}

	visible := visibleArticles(all)
	r.all = visible

	if err := r.home(sel, visible); err != nil {
		return err
	}
	for _, sec := range model.AllSections {
		var in []model.Article
		for _, a := range visible {
			if a.Section == sec {
				in = append(in, a)
			}
		}
		if len(in) == 0 {
			continue
		}
		if err := r.list(listSpec{
			Dir: filepath.Join("s", string(sec)), Heading: sec.Label(), Blurb: blurbs[sec], Tpl: "list.html",
			Tabs: func(base string) []Tab { return r.tabs(base, sec) }, Topics: true, Arts: in,
		}); err != nil {
			return err
		}
	}
	for tag, in := range byTag(visible) {
		if err := r.list(listSpec{Dir: filepath.Join("t", tag), Heading: "#" + tag, Tpl: "list.html", Arts: in}); err != nil {
			return err
		}
	}
	for _, a := range visible {
		if a.HasPage() {
			if err := r.article(a, visible); err != nil {
				return err
			}
		}
	}
	if err := r.about(); err != nil {
		return err
	}
	if opts.SiteURL != "" {
		if err := r.feed(visible); err != nil {
			return err
		}
		if err := r.sitemap(visible); err != nil {
			return err
		}
	}
	return copyDir(fsys, "static", filepath.Join(outDir, "static"))
}

// visibleArticles descarta los ocultos y ordena de más reciente a más antiguo.
func visibleArticles(all []model.Article) []model.Article {
	var out []model.Article
	for _, a := range all {
		if !a.Skip {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Published.After(out[j].Published) })
	return out
}

func byTag(arts []model.Article) map[string][]model.Article {
	m := map[string][]model.Article{}
	for _, a := range arts {
		for _, t := range a.Tags {
			m[t] = append(m[t], a)
		}
	}
	return m
}

func (r *renderer) page(base, title, desc string) Page {
	p := Page{
		SiteName:    siteName,
		Title:       title + " · " + siteName,
		Description: desc,
		Base:        base,
		Updated:     r.updated,
		HasFeed:     r.opts.SiteURL != "",
		RepoURL:     r.opts.RepoURL,
	}
	for _, sec := range model.AllSections {
		p.Nav = append(p.Nav, NavItem{Label: sec.Short(), Href: base + "s/" + string(sec) + "/"})
	}
	return p
}

// listSpec describe una lista paginada de noticias, ya sea la portada, una
// sección o una etiqueta.
type listSpec struct {
	Dir, Heading, Blurb, Title, Desc string
	Tpl                              string
	Tabs                             func(base string) []Tab
	Topics                           bool
	Featured                         *Card
	Arts                             []model.Article
}

func (r *renderer) home(sel rank.Selection, visible []model.Article) error {
	spec := listSpec{
		Tpl:     "index.html",
		Heading: "Latest news",
		Title:   siteName + " · AI, science & technology news",
		Desc:    "The latest news on artificial intelligence, science and technology, plus programming tutorials. Updated every 6 hours.",
		Tabs:    func(base string) []Tab { return r.tabs(base, "") },
		Topics:  true,
		Arts:    visible,
	}
	if sel.Featured != nil {
		c := card("", *sel.Featured)
		spec.Featured = &c
		// El destacado ya sale arriba: no se repite en la lista.
		spec.Arts = nil
		for _, a := range visible {
			if a.ID != sel.Featured.ID {
				spec.Arts = append(spec.Arts, a)
			}
		}
	}
	return r.list(spec)
}

// tabs devuelve las pestañas de sección; current marca la activa ("" = todas).
func (r *renderer) tabs(base string, current model.Section) []Tab {
	tabs := []Tab{{Label: "All", Href: base, Current: current == ""}}
	for _, sec := range model.AllSections {
		tabs = append(tabs, Tab{Label: sec.Short(), Href: base + "s/" + string(sec) + "/", Current: sec == current})
	}
	return tabs
}

// depth es el número de niveles de directorio de dir.
func depth(dir string) int {
	dir = filepath.ToSlash(dir)
	if dir == "" || dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// list escribe una lista paginada: la primera página va en Dir y las demás en
// Dir/page/N/. La portada usa Dir vacío.
func (r *renderer) list(sp listSpec) error {
	total := max((len(sp.Arts)+listLimit-1)/listLimit, 1)
	for n := 1; n <= total; n++ {
		pdir := sp.Dir
		if n > 1 {
			pdir = filepath.Join(sp.Dir, "page", strconv.Itoa(n))
		}
		base := strings.Repeat("../", depth(pdir))
		title, desc := sp.Title, sp.Desc
		if title == "" {
			title = sp.Heading
			desc = fmt.Sprintf("%s: latest news from %s.", sp.Heading, siteName)
		}
		p := r.page(base, title, desc)
		if sp.Title != "" { // la portada ya trae el título completo
			p.Title = sp.Title
		}
		if n > 1 {
			p.Title = fmt.Sprintf("%s (page %d) · %s", sp.Heading, n, siteName)
		}
		p.Heading, p.Blurb = sp.Heading, sp.Blurb
		if sp.Tabs != nil {
			p.Tabs = sp.Tabs(base)
		}
		if n == 1 {
			p.Featured = sp.Featured
		}
		for _, a := range sp.Arts[(n-1)*listLimit : min(n*listLimit, len(sp.Arts))] {
			p.Cards = append(p.Cards, card(base, a))
		}
		if sp.Topics {
			src := sp.Arts // en una sección, los temas de esa sección
			if sp.Dir == "" {
				src = r.all
			}
			p.Topics = topChips(base, src, topicCount)
		}
		if total > 1 {
			p.Pager = pager(n, total)
		}
		if err := r.write(sp.Tpl, pdir, p); err != nil {
			return err
		}
	}
	return nil
}

// pageHref es el enlace relativo desde la página from hasta la página to.
func pageHref(from, to int) string {
	switch {
	case from == 1 && to == 1:
		return "./"
	case from == 1:
		return fmt.Sprintf("page/%d/", to)
	case to == 1:
		return "../../"
	}
	return fmt.Sprintf("../%d/", to)
}

func pager(cur, total int) *Pager {
	pg := &Pager{}
	if cur > 1 {
		pg.Prev = pageHref(cur, cur-1)
	}
	if cur < total {
		pg.Next = pageHref(cur, cur+1)
	}
	for n := 1; n <= total; n++ {
		pg.Links = append(pg.Links, PageLink{N: n, Href: pageHref(cur, n), Current: n == cur})
	}
	return pg
}

func (r *renderer) article(a model.Article, visible []model.Article) error {
	base := "../../"
	desc := a.Summary
	if desc == "" {
		desc = a.Excerpt
	}
	p := r.page(base, a.Title, truncate(desc, 160))
	p.Article = &a
	p.Section = NavItem{Label: a.Section.Label(), Href: base + "s/" + string(a.Section) + "/"}
	for _, t := range a.Tags {
		p.Tags = append(p.Tags, Chip{Tag: t, Href: base + "t/" + t + "/"})
	}
	for _, o := range visible {
		if len(p.Related) == relatedCount {
			break
		}
		if o.ID != a.ID && o.Section == a.Section {
			p.Related = append(p.Related, card(base, o))
		}
	}
	return r.write("article.html", filepath.Join("n", a.ID), p)
}

// card decide el destino de una tarjeta según el artículo tenga página propia.
func card(base string, a model.Article) Card {
	if a.HasPage() {
		return Card{Article: a, Href: base + "n/" + a.ID + "/"}
	}
	return Card{Article: a, Href: a.URL, External: true}
}

// topChips devuelve las etiquetas más usadas, de más a menos y por nombre.
func topChips(base string, arts []model.Article, n int) []Chip {
	counts := map[string]int{}
	for _, a := range arts {
		for _, t := range a.Tags {
			counts[t]++
		}
	}
	chips := make([]Chip, 0, len(counts))
	for t, c := range counts {
		chips = append(chips, Chip{Tag: t, Count: c, Href: base + "t/" + t + "/"})
	}
	sort.Slice(chips, func(i, j int) bool {
		if chips[i].Count != chips[j].Count {
			return chips[i].Count > chips[j].Count
		}
		return chips[i].Tag < chips[j].Tag
	})
	if len(chips) > n {
		chips = chips[:n]
	}
	return chips
}

func (r *renderer) write(tplName, dir string, p Page) error {
	if r.opts.SiteURL != "" {
		p.Canonical = r.abs(dir)
	}
	full := filepath.Join(r.outDir, dir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(full, "index.html"))
	if err != nil {
		return err
	}
	if err := r.tpl.ExecuteTemplate(f, tplName, p); err != nil {
		f.Close()
		return fmt.Errorf("%s (%s): %w", tplName, dir, err)
	}
	return f.Close()
}

func copyDir(fsys fs.FS, src, dst string) error {
	return fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// secureURL pasa las imágenes http:// a https:// para que el navegador no
// las bloquee como contenido mixto cuando el sitio se sirva por HTTPS.
func secureURL(u string) string {
	if strings.HasPrefix(u, "http://") {
		return "https://" + strings.TrimPrefix(u, "http://")
	}
	return u
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// Ago escribe una antigüedad relativa corta ("3h ago").
func Ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
