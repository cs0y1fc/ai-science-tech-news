package rank

import (
	"fmt"
	"testing"
	"time"

	"noticias-tech/internal/model"
)

var now = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func art(id string, sec model.Section, age time.Duration) model.Article {
	return model.Article{
		ID: id, Title: "Titular " + id, URL: "https://example.com/" + id,
		Source: "S" + id, Section: sec, Published: now.Add(-age),
	}
}

func TestScoreFreshnessPopularityWeight(t *testing.T) {
	newer := Score(art("a", model.SectionAI, time.Hour), now, 0)
	older := Score(art("b", model.SectionAI, 30*time.Hour), now, 0)
	if newer <= older {
		t.Errorf("lo reciente debe puntuar más: %v vs %v", newer, older)
	}

	pop := art("c", model.SectionAI, time.Hour)
	pop.Points = 500
	if Score(pop, now, 0) <= newer {
		t.Error("los puntos deben sumar")
	}

	heavy := art("d", model.SectionAI, time.Hour)
	heavy.Weight = 2
	light := art("e", model.SectionAI, time.Hour)
	light.Weight = 0.5
	if Score(heavy, now, 0) <= Score(light, now, 0) {
		t.Error("el peso de la fuente debe contar")
	}

	if Score(newer0(), now, 3) <= Score(newer0(), now, 0) {
		t.Error("la cobertura debe sumar")
	}
	if Score(newer0(), now, 10) != Score(newer0(), now, 3) {
		t.Error("la cobertura debe tener tope en 3")
	}
}

func newer0() model.Article { return art("z", model.SectionTech, time.Hour) }

func TestScoreFutureDateDoesNotExplode(t *testing.T) {
	a := art("f", model.SectionAI, -48*time.Hour)
	if s := Score(a, now, 0); s > 10.001 {
		t.Errorf("fecha futura puntúa %v, máximo esperado 10", s)
	}
}

func TestSelectAgeFilter(t *testing.T) {
	in := []model.Article{
		art("n-old", model.SectionTech, 5*24*time.Hour),
		art("n-ok", model.SectionTech, time.Hour),
		art("t-ok", model.SectionTutorials, 10*24*time.Hour),
		art("t-old", model.SectionTutorials, 20*24*time.Hour),
	}
	sel := Select(in, Options{Now: now})
	got := map[string]bool{}
	if sel.Featured != nil {
		got[sel.Featured.ID] = true
	}
	for _, r := range sel.Rows {
		for _, a := range r.Articles {
			got[a.ID] = true
		}
	}
	if got["n-old"] || got["t-old"] || !got["n-ok"] || !got["t-ok"] {
		t.Errorf("filtro de antigüedad incorrecto: %v", got)
	}
}

func TestSelectFeaturedPrefersImageAndIsNotTutorial(t *testing.T) {
	noImg := art("best", model.SectionAI, time.Minute)
	noImg.Points = 1000
	withImg := art("pic", model.SectionScience, 2*time.Hour)
	withImg.ImageURL = "https://example.com/i.jpg"
	tut := art("tut", model.SectionTutorials, time.Minute)
	tut.Points = 9999

	sel := Select([]model.Article{noImg, withImg, tut}, Options{Now: now})
	if sel.Featured == nil || sel.Featured.ID != "pic" {
		t.Fatalf("destacada = %+v, quería pic", sel.Featured)
	}
	for _, r := range sel.Rows {
		for _, a := range r.Articles {
			if a.ID == "pic" {
				t.Error("la destacada no debe repetirse en las filas")
			}
		}
	}
}

func TestSelectFeaturedFallsBackWithoutImages(t *testing.T) {
	a := art("a", model.SectionAI, time.Hour)
	sel := Select([]model.Article{a, art("t", model.SectionTutorials, time.Hour)}, Options{Now: now})
	if sel.Featured == nil || sel.Featured.ID != "a" {
		t.Errorf("destacada = %+v", sel.Featured)
	}
}

func TestSelectPerSectionLimitOrderAndEmptyRows(t *testing.T) {
	var in []model.Article
	for i := 0; i < 12; i++ {
		in = append(in, art(fmt.Sprintf("ai%02d", i), model.SectionAI, time.Duration(i+1)*time.Hour))
	}
	sel := Select(in, Options{Now: now, PerSection: 5})
	if len(sel.Rows) != 1 || sel.Rows[0].Section != model.SectionAI {
		t.Fatalf("debe haber solo la fila de IA: %+v", sel.Rows)
	}
	// 12 artículos: 1 destacada + 5 en la fila.
	if n := len(sel.Rows[0].Articles); n != 5 {
		t.Errorf("fila con %d artículos, quería 5", n)
	}
	rows := sel.Rows[0].Articles
	for i := 1; i < len(rows); i++ {
		if Score(rows[i-1], now, 0) < Score(rows[i], now, 0) {
			t.Error("la fila debe ir ordenada por puntuación")
		}
	}
}

func TestSelectRowOrderFollowsSections(t *testing.T) {
	in := []model.Article{
		art("t", model.SectionTutorials, time.Hour),
		art("c", model.SectionScience, time.Hour),
		art("i", model.SectionAI, time.Hour),
		art("x", model.SectionTech, time.Hour),
		art("y", model.SectionTech, time.Hour),
	}
	sel := Select(in, Options{Now: now})
	var got []model.Section
	for _, r := range sel.Rows {
		got = append(got, r.Section)
	}
	for i := 1; i < len(got); i++ {
		if indexOf(got[i-1]) > indexOf(got[i]) {
			t.Errorf("filas desordenadas: %v", got)
		}
	}
}

func indexOf(s model.Section) int {
	for i, x := range model.AllSections {
		if x == s {
			return i
		}
	}
	return -1
}

func TestSelectEmpty(t *testing.T) {
	sel := Select(nil, Options{Now: now})
	if sel.Featured != nil || len(sel.Rows) != 0 {
		t.Errorf("selección no vacía: %+v", sel)
	}
}

func TestCoverageBoostsSharedStories(t *testing.T) {
	a := art("a", model.SectionAI, time.Hour)
	a.Title = "OpenAI releases new reasoning model for developers"
	b := art("b", model.SectionTech, time.Hour)
	b.Title = "OpenAI releases a new reasoning model for developers today"
	c := art("c", model.SectionTech, time.Hour)
	c.Title = "Completely unrelated story about quantum batteries"
	cov := coverage([]model.Article{a, b, c})
	if cov[0] != 1 || cov[1] != 1 || cov[2] != 0 {
		t.Errorf("cobertura = %v, quería [1 1 0]", cov)
	}
}

func TestCoverageIgnoresSameSource(t *testing.T) {
	a := art("a", model.SectionAI, time.Hour)
	a.Source = "X"
	a.Title = "OpenAI releases new reasoning model for developers"
	b := a
	b.ID = "b"
	if cov := coverage([]model.Article{a, b}); cov[0] != 0 {
		t.Errorf("la misma fuente no cuenta como cobertura: %v", cov)
	}
}

func TestSelectAndCandidatesExcludeSkipped(t *testing.T) {
	hidden := art("hidden", model.SectionAI, time.Minute)
	hidden.Skip = true
	hidden.Points = 5000
	shown := art("shown", model.SectionAI, time.Hour)
	in := []model.Article{hidden, shown}

	for _, a := range Candidates(in, Options{Now: now}) {
		if a.ID == "hidden" {
			t.Error("Candidates no debe devolver artículos ocultos")
		}
	}
	sel := Select(in, Options{Now: now})
	if sel.Featured == nil || sel.Featured.ID != "shown" {
		t.Errorf("destacada = %+v", sel.Featured)
	}
}

func TestCandidatesOrderedByScore(t *testing.T) {
	low := art("low", model.SectionTech, 40*time.Hour)
	high := art("high", model.SectionTech, time.Hour)
	got := Candidates([]model.Article{low, high}, Options{Now: now})
	if len(got) != 2 || got[0].ID != "high" {
		t.Errorf("got %+v", got)
	}
}
