package model

import "time"

// Section agrupa las noticias en la portada.
type Section string

const (
	SectionAI        Section = "ai"
	SectionScience   Section = "science"
	SectionTech      Section = "tech"
	SectionTutorials Section = "tutorials"
)

// Normalize traduce los nombres de sección que tenían los datos antiguos
// (en español) a los actuales.
func (s Section) Normalize() Section {
	switch s {
	case "ia":
		return SectionAI
	case "ciencia":
		return SectionScience
	case "tutoriales":
		return SectionTutorials
	}
	return s
}

// AllSections es el orden en que aparecen las secciones en la portada.
var AllSections = []Section{SectionAI, SectionScience, SectionTech, SectionTutorials}

// Label es el nombre que se muestra al lector.
func (s Section) Label() string {
	switch s {
	case SectionAI:
		return "Artificial Intelligence"
	case SectionScience:
		return "Science"
	case SectionTech:
		return "Technology"
	case SectionTutorials:
		return "Tutorials"
	}
	return string(s)
}

// Short es el nombre corto, para menús e insignias.
func (s Section) Short() string {
	switch s {
	case SectionAI:
		return "AI"
	case SectionTutorials:
		return "Tutorials"
	}
	return s.Label()
}

// Article es una noticia recogida de una fuente.
type Article struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Source    string    `json:"source"`
	Section   Section   `json:"section"`
	Published time.Time `json:"published"`
	Excerpt   string    `json:"excerpt,omitempty"`
	ImageURL  string    `json:"image_url,omitempty"`
	// ImageChecked indica que ya se buscó imagen en la página del artículo,
	// con o sin éxito, para no repetir la descarga en cada ejecución.
	ImageChecked bool `json:"image_checked,omitempty"`
	// Points es la señal de popularidad de la fuente (p. ej. puntos de Hacker News).
	Points int `json:"points,omitempty"`
	// Weight es el peso de la fuente; 0 se interpreta como 1.
	Weight float64 `json:"weight,omitempty"`

	// FirstSeen es cuándo vio la web el artículo por primera vez.
	FirstSeen time.Time `json:"first_seen"`
	// Summary y Tags los rellena la fase de enriquecimiento con IA. Tags puede
	// traer etiquetas por palabras clave mientras Enriched sea false.
	Summary string   `json:"summary,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	// Enriched indica que la IA ya procesó el artículo (y no hay que repetirlo).
	Enriched bool `json:"enriched,omitempty"`
	// Skip oculta artículos que la IA juzgó ajenos a los temas de la web.
	Skip bool `json:"skip,omitempty"`
}

// HasPage indica si el artículo tiene contenido propio (resumen o extracto)
// que justifique una página en la web. Sin él, la tarjeta enlaza a la fuente.
func (a Article) HasPage() bool {
	return a.Summary != "" || a.Excerpt != ""
}
