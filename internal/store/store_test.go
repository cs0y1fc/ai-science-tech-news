package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"noticias-tech/internal/model"
)

var now = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func TestLoadMissingFile(t *testing.T) {
	arts, err := Load(filepath.Join(t.TempDir(), "no-existe.json"))
	if err != nil || len(arts) != 0 {
		t.Errorf("arts=%v err=%v", arts, err)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	os.WriteFile(p, []byte("{no es json"), 0o644)
	if _, err := Load(p); err == nil {
		t.Error("esperaba error")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "articles.json")
	in := []model.Article{{
		ID: "a1", Title: "T", URL: "https://x.com/1", Source: "S", Section: model.SectionAI,
		Published: now, FirstSeen: now, Summary: "resumen", Tags: []string{"llm"}, Points: 4,
	}}
	if err := Save(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil || len(out) != 1 {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if out[0].Summary != "resumen" || out[0].Tags[0] != "llm" || !out[0].Published.Equal(now) {
		t.Errorf("round trip incorrecto: %+v", out[0])
	}
	// No debe quedar ningún temporal.
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("sobran ficheros: %v", entries)
	}
}

func TestMergeKeepsEnrichmentAndFirstSeen(t *testing.T) {
	first := now.Add(-6 * time.Hour)
	old := []model.Article{{
		ID: "a", Title: "Viejo", Published: now.Add(-7 * time.Hour), FirstSeen: first,
		Summary: "mi resumen", Tags: []string{"llm"}, Enriched: true, Section: model.SectionScience, ImageURL: "https://x.com/old.jpg", Points: 10,
	}}
	fresh := []model.Article{{ID: "a", Title: "Nuevo título", Section: model.SectionAI, Published: now.Add(-7 * time.Hour), Points: 99}}

	out := Merge(old, fresh, now, 30*24*time.Hour)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	a := out[0]
	if a.Points != 99 || a.Title != "Nuevo título" {
		t.Errorf("los datos frescos deben ganar: %+v", a)
	}
	if a.Summary != "mi resumen" || len(a.Tags) != 1 || !a.Enriched || !a.FirstSeen.Equal(first) {
		t.Errorf("se perdió el enriquecimiento: %+v", a)
	}
	if a.ImageURL != "https://x.com/old.jpg" {
		t.Errorf("la imagen vieja debe conservarse si la fresca está vacía: %q", a.ImageURL)
	}
}

func TestMergeKeepsAISectionAndSkip(t *testing.T) {
	old := []model.Article{{ID: "a", Published: now, Enriched: true, Skip: true, Section: model.SectionTech}}
	fresh := []model.Article{{ID: "a", Published: now, Section: model.SectionAI}}
	a := Merge(old, fresh, now, time.Hour*24)[0]
	if a.Section != model.SectionTech || !a.Skip {
		t.Errorf("la decisión de la IA debe sobrevivir a un nuevo fetch: %+v", a)
	}
}

func TestMergeDropsNonAIEnrichment(t *testing.T) {
	old := []model.Article{{ID: "a", Published: now, Summary: "extracto", Tags: []string{"web"}, Section: model.SectionTech}}
	fresh := []model.Article{{ID: "a", Published: now, Section: model.SectionAI}}
	a := Merge(old, fresh, now, time.Hour*24)[0]
	if a.Summary != "" || len(a.Tags) != 0 || a.Section != model.SectionAI || a.Enriched {
		t.Errorf("lo no procesado por la IA no debe conservarse: %+v", a)
	}
}

func TestMergeNewGetsFirstSeenAndOldIsDropped(t *testing.T) {
	old := []model.Article{
		{ID: "ancient", Published: now.Add(-40 * 24 * time.Hour)},
		{ID: "keep", Published: now.Add(-2 * 24 * time.Hour)},
	}
	fresh := []model.Article{{ID: "new", Published: now.Add(-time.Hour)}}

	out := Merge(old, fresh, now, 30*24*time.Hour)
	if len(out) != 2 || out[0].ID != "new" || out[1].ID != "keep" {
		t.Fatalf("resultado inesperado: %+v", out)
	}
	if !out[0].FirstSeen.Equal(now) {
		t.Errorf("FirstSeen = %v", out[0].FirstSeen)
	}
}

func TestMergeEmptyFreshKeepsOld(t *testing.T) {
	old := []model.Article{{ID: "a", Published: now.Add(-time.Hour)}}
	if out := Merge(old, nil, now, time.Hour*24); len(out) != 1 {
		t.Errorf("out = %+v", out)
	}
}
