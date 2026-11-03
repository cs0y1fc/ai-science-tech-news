package sources

import (
	"os"
	"testing"

	"noticias-tech/internal/model"
)

func TestParseConfigDefaults(t *testing.T) {
	srcs, err := ParseConfig([]byte(`
sources:
  - name: Demo
    url: https://example.com/feed
    section: ai
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 1 || srcs[0].Type != "rss" || srcs[0].Weight != 1 || srcs[0].Section != model.SectionAI {
		t.Errorf("valores por defecto incorrectos: %+v", srcs)
	}
}

func TestParseConfigErrors(t *testing.T) {
	bad := map[string]string{
		"sin url":      "sources:\n  - name: X\n    section: ai\n",
		"tipo malo":    "sources:\n  - name: X\n    url: u\n    section: ai\n    type: ftp\n",
		"sección mala": "sources:\n  - name: X\n    url: u\n    section: deportes\n",
		"yaml roto":    "sources: [",
	}
	for name, y := range bad {
		if _, err := ParseConfig([]byte(y)); err == nil {
			t.Errorf("%s: esperaba error", name)
		}
	}
}

// El sources.yaml real del repositorio debe ser siempre válido.
func TestRealSourcesFileIsValid(t *testing.T) {
	data, err := os.ReadFile("../../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	srcs, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	sections := map[model.Section]int{}
	for _, s := range srcs {
		sections[s.Section]++
	}
	for _, sec := range []model.Section{model.SectionAI, model.SectionScience, model.SectionTech, model.SectionTutorials} {
		if sections[sec] == 0 {
			t.Errorf("la sección %q no tiene fuentes", sec)
		}
	}
}
