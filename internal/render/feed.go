package render

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"noticias-tech/internal/model"
)

// feedLimit es el máximo de entradas del feed RSS.
const feedLimit = 50

// abs convierte una ruta relativa del sitio en URL absoluta.
func (r *renderer) abs(rel string) string {
	rel = filepath.ToSlash(rel)
	if rel == "" || rel == "." {
		return r.opts.SiteURL + "/"
	}
	return r.opts.SiteURL + "/" + strings.Trim(rel, "/") + "/"
}

func (r *renderer) about() error {
	p := r.page("../", "About", "How "+siteName+" is built, where the news comes from and what the limits of automatic summaries are.")
	p.Sources = r.opts.Sources
	return r.write("about.html", "about", p)
}

// searchIndex escribe search.json, el índice que usa el buscador del
// navegador: las etiquetas (con su número de noticias, de más a menos
// frecuente) y las noticias visibles, de más reciente a más antigua.
func (r *renderer) searchIndex(visible []model.Article) error {
	type tagEntry struct {
		T string `json:"t"`
		N int    `json:"n"`
	}
	type artEntry struct {
		T string `json:"t"` // título
		S string `json:"s"` // fuente
		K string `json:"k"` // sección
		H string `json:"h"` // "n/id/" (página propia) o URL absoluta
	}
	tags := []tagEntry{}
	for tag, in := range byTag(visible) {
		tags = append(tags, tagEntry{tag, len(in)})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].N != tags[j].N {
			return tags[i].N > tags[j].N
		}
		return tags[i].T < tags[j].T
	})
	arts := make([]artEntry, 0, len(visible))
	for _, a := range visible {
		c := card("", a)
		arts = append(arts, artEntry{T: a.Title, S: a.Source, K: string(a.Section), H: c.Href})
	}
	b, err := json.Marshal(struct {
		Tags     []tagEntry `json:"tags"`
		Articles []artEntry `json:"articles"`
	}{tags, arts})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.outDir, "search.json"), b, 0o644)
}

// saved genera la página de favoritos. Se rellena en el navegador con los
// favoritos del usuario; sin Supabase configurado no se genera.
func (r *renderer) saved() error {
	if r.opts.SupabaseURL == "" || r.opts.SupabaseKey == "" {
		return nil
	}
	p := r.page("../", "Saved", "Your saved stories.")
	return r.write("saved.html", "saved", p)
}

type rss struct {
	XML     xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Self          atomLink  `xml:"atom:link"`
	Items         []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        rssGUID  `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}

type rssGUID struct {
	Permalink bool   `xml:"isPermaLink,attr"`
	Value     string `xml:",chardata"`
}

// itemURL es la página propia del artículo si existe, o la fuente original.
func (r *renderer) itemURL(a model.Article) string {
	if a.HasPage() {
		return r.abs("n/" + a.ID)
	}
	return a.URL
}

func (r *renderer) feed(visible []model.Article) error {
	ch := rssChannel{
		Title:         siteName,
		Link:          r.abs(""),
		Description:   "News on AI, science and technology, updated every 6 hours.",
		Language:      "en",
		LastBuildDate: r.now.UTC().Format(time.RFC1123Z),
		Self:          atomLink{Href: r.opts.SiteURL + "/feed.xml", Rel: "self", Type: "application/rss+xml"},
	}
	for i, a := range visible {
		if i == feedLimit {
			break
		}
		desc := a.Summary
		if desc == "" {
			desc = a.Excerpt
		}
		link := r.itemURL(a)
		ch.Items = append(ch.Items, rssItem{
			Title:       a.Title,
			Link:        link,
			GUID:        rssGUID{Permalink: a.HasPage(), Value: link},
			PubDate:     a.Published.UTC().Format(time.RFC1123Z),
			Description: truncate(desc, 600),
			Categories:  append([]string{a.Section.Label()}, a.Tags...),
		})
	}
	return r.writeXML("feed.xml", rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Channel: ch})
}

type urlset struct {
	XML   xml.Name `xml:"urlset"`
	Xmlns string   `xml:"xmlns,attr"`
	URLs  []siteURL
}

type siteURL struct {
	XML     xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

func (r *renderer) sitemap(visible []model.Article) error {
	day := func(t time.Time) string { return t.UTC().Format("2006-01-02") }
	set := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(rel string, t time.Time) {
		u := siteURL{Loc: r.abs(rel)}
		if !t.IsZero() {
			u.LastMod = day(t)
		}
		set.URLs = append(set.URLs, u)
	}

	add("", r.now)
	add("about", time.Time{})
	for _, sec := range model.AllSections {
		for _, a := range visible { // visible va de más reciente a más antiguo
			if a.Section == sec {
				add("s/"+string(sec), a.Published)
				break
			}
		}
	}
	tags := byTag(visible)
	names := make([]string, 0, len(tags))
	for tag := range tags {
		names = append(names, tag)
	}
	sort.Strings(names)
	for _, tag := range names {
		add("t/"+tag, tags[tag][0].Published)
	}
	for _, a := range visible {
		if a.HasPage() {
			add("n/"+a.ID, a.Published)
		}
	}
	return r.writeXML("sitemap.xml", set)
}

func (r *renderer) writeXML(name string, v any) error {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.outDir, name), append([]byte(xml.Header), out...), 0o644)
}
