package rank

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"noticias-tech/internal/model"
)

// Options controla qué artículos entran en la portada.
type Options struct {
	Now time.Time
	// PerSection es el máximo de artículos por fila (por defecto 8).
	PerSection int
	// MaxAgeNews y MaxAgeTutorials descartan artículos más antiguos.
	MaxAgeNews      time.Duration
	MaxAgeTutorials time.Duration
}

func (o Options) withDefaults() Options {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.PerSection == 0 {
		o.PerSection = 8
	}
	if o.MaxAgeNews == 0 {
		o.MaxAgeNews = 3 * 24 * time.Hour
	}
	if o.MaxAgeTutorials == 0 {
		o.MaxAgeTutorials = 14 * 24 * time.Hour
	}
	return o
}

// Row es una fila de la portada.
type Row struct {
	Section  model.Section
	Articles []model.Article
}

// Selection es lo que se muestra en la portada.
type Selection struct {
	Featured *model.Article
	Rows     []Row
}

// Score puntúa un artículo: frescura (decae con una vida media de ~17 h),
// popularidad en la fuente (logarítmica) y peso de la fuente, más un bonus
// por cada otra fuente que cubre lo mismo (hasta 3).
func Score(a model.Article, now time.Time, coverage int) float64 {
	age := now.Sub(a.Published).Hours()
	if age < 0 {
		age = 0
	}
	fresh := 10 * math.Exp(-age/24)
	pop := 2 * math.Log1p(float64(a.Points))
	w := a.Weight
	if w <= 0 {
		w = 1
	}
	return w*(fresh+pop) + 3*float64(min(coverage, 3))
}

type scored struct {
	a model.Article
	s float64
}

// ranked filtra por antigüedad y descarta los ocultos, y devuelve el resto
// ordenado por puntuación de mayor a menor.
func ranked(in []model.Article, o Options) []scored {
	var fresh []model.Article
	for _, a := range in {
		if a.Skip {
			continue
		}
		maxAge := o.MaxAgeNews
		if a.Section == model.SectionTutorials {
			maxAge = o.MaxAgeTutorials
		}
		if o.Now.Sub(a.Published) <= maxAge {
			fresh = append(fresh, a)
		}
	}
	cov := coverage(fresh)
	all := make([]scored, len(fresh))
	for i, a := range fresh {
		all[i] = scored{a, Score(a, o.Now, cov[i])}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].s > all[j].s })
	return all
}

// Candidates devuelve los artículos vigentes por orden de puntuación. Sirve
// para decidir a cuáles gastar cuota de IA antes de elegir la portada.
func Candidates(in []model.Article, o Options) []model.Article {
	o = o.withDefaults()
	all := ranked(in, o)
	out := make([]model.Article, len(all))
	for i, x := range all {
		out[i] = x.a
	}
	return out
}

// Select elige la destacada y las filas por sección.
func Select(in []model.Article, o Options) Selection {
	o = o.withDefaults()

	all := ranked(in, o)

	// La destacada es lo mejor que no sea un tutorial, preferiblemente con imagen.
	featured := -1
	for i, x := range all {
		if x.a.Section == model.SectionTutorials {
			continue
		}
		if x.a.ImageURL != "" {
			featured = i
			break
		}
		if featured == -1 {
			featured = i
		}
	}

	var sel Selection
	if featured >= 0 {
		f := all[featured].a
		sel.Featured = &f
	}
	for _, sec := range model.AllSections {
		row := Row{Section: sec}
		for i, x := range all {
			if i == featured || x.a.Section != sec {
				continue
			}
			if len(row.Articles) == o.PerSection {
				break
			}
			row.Articles = append(row.Articles, x.a)
		}
		if len(row.Articles) > 0 {
			sel.Rows = append(sel.Rows, row)
		}
	}
	return sel
}

var (
	wordRe    = regexp.MustCompile(`[a-z0-9]+`)
	stopwords = map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true, "that": true,
		"this": true, "your": true, "will": true, "how": true, "why": true, "what": true,
		"are": true, "new": true, "its": true, "has": true, "have": true, "into": true,
	}
)

func tokens(title string) map[string]bool {
	set := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(title), -1) {
		if len(w) > 2 && !stopwords[w] {
			set[w] = true
		}
	}
	return set
}

// similar usa la similitud de Jaccard entre las palabras de dos títulos.
func similar(a, b map[string]bool) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter)/float64(union) >= 0.5
}

// coverage cuenta, para cada artículo, cuántas otras fuentes distintas
// publican algo con un título parecido.
func coverage(in []model.Article) []int {
	toks := make([]map[string]bool, len(in))
	for i, a := range in {
		toks[i] = tokens(a.Title)
	}
	out := make([]int, len(in))
	for i := range in {
		others := map[string]bool{}
		for j := range in {
			if i != j && in[i].Source != in[j].Source && similar(toks[i], toks[j]) {
				others[in[j].Source] = true
			}
		}
		out[i] = len(others)
	}
	return out
}
