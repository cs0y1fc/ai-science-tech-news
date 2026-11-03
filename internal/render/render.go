package render

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"noticias-tech/internal/model"
	"noticias-tech/internal/rank"
)

const (
	siteName = "Noticias Tech"
	// listLimit es el máximo de tarjetas en una página de sección o etiqueta.
	listLimit = 60
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

// RowView es una fila de la portada.
type RowView struct {
	Section model.Section
	Href    string
	Cards   []Card
}

// NavItem es un enlace del menú.
type NavItem struct {
	Label, Href string
}

// Chip es una etiqueta con su recuento.
type Chip struct {
	Tag   string
	Count int
	Href  string
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

	// Portada.
	Featured *Card
	Rows     []RowView
	Topics   []Chip

	// Listas (sección o etiqueta).
	Heading string
	Cards   []Card

	// Artículo.
	Article *model.Article
	Section NavItem
	Tags    []Chip
	Related []Card
}

type renderer struct {
	tpl     *template.Template
	outDir  string
	now     time.Time
	updated time.Time
}

// Site genera el sitio estático en outDir a partir de las plantillas y
// estáticos de fsys (que debe contener templates/ y static/). sel es la
// selección de la portada y all todos los artículos guardados.
func Site(fsys fs.FS, outDir string, now time.Time, sel rank.Selection, all []model.Article) error {
	funcs := template.FuncMap{
		"ago": func(t time.Time) string { return Ago(t, now) },
		"img": secureURL,
	}
	tpl, err := template.New("").Funcs(funcs).ParseFS(fsys, "templates/*.html")
	if err != nil {
		return fmt.Errorf("plantillas: %w", err)
	}
	r := &renderer{tpl: tpl, outDir: outDir, now: now, updated: now}

	visible := visibleArticles(all)

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
		if err := r.list(filepath.Join("s", string(sec)), sec.Label(), in); err != nil {
			return err
		}
	}
	for tag, in := range byTag(visible) {
		if err := r.list(filepath.Join("t", tag), "#"+tag, in); err != nil {
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
	}
	for _, sec := range model.AllSections {
		p.Nav = append(p.Nav, NavItem{Label: sec.Label(), Href: base + "s/" + string(sec) + "/"})
	}
	return p
}

func (r *renderer) home(sel rank.Selection, visible []model.Article) error {
	p := r.page("", "IA, ciencia y tecnología", "Las noticias más destacadas de inteligencia artificial, ciencia y tecnología, y tutoriales para programadores. Se actualiza cada 6 horas.")
	p.Title = siteName + " · IA, ciencia y tecnología"
	if sel.Featured != nil {
		c := card("", *sel.Featured)
		p.Featured = &c
	}
	for _, row := range sel.Rows {
		rv := RowView{Section: row.Section, Href: "s/" + string(row.Section) + "/"}
		for _, a := range row.Articles {
			rv.Cards = append(rv.Cards, card("", a))
		}
		p.Rows = append(p.Rows, rv)
	}
	p.Topics = topChips("", visible, topicCount)
	return r.write("index.html", "", p)
}

func (r *renderer) list(dir, heading string, arts []model.Article) error {
	base := "../../"
	p := r.page(base, heading, fmt.Sprintf("%s: últimas noticias de %s.", heading, siteName))
	p.Heading = heading
	for i, a := range arts {
		if i == listLimit {
			break
		}
		p.Cards = append(p.Cards, card(base, a))
	}
	return r.write("list.html", dir, p)
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

// Ago escribe una antigüedad relativa corta en español ("hace 3 h").
func Ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "ahora mismo"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("hace %d d", int(d.Hours()/24))
	}
}
