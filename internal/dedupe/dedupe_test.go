package dedupe

import (
	"testing"
	"time"

	"noticias-tech/internal/model"
)

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"http://www.Example.com/a/b/?utm_source=x&id=7#top": "https://example.com/a/b?id=7",
		"https://example.com/a/":                            "https://example.com/a",
		"https://example.com/a?fbclid=abc":                  "https://example.com/a",
	}
	for in, want := range cases {
		if got := NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestIDStableAcrossVariants(t *testing.T) {
	a := ID("https://www.example.com/post/?utm_campaign=1")
	b := ID("http://example.com/post")
	if a != b {
		t.Errorf("IDs distintos para la misma URL: %s vs %s", a, b)
	}
	if len(a) != 12 {
		t.Errorf("longitud de ID = %d, quería 12", len(a))
	}
}

func TestDedupeByURLKeepsMostPoints(t *testing.T) {
	in := []model.Article{
		{Title: "Uno", URL: "https://example.com/x", Source: "A", Points: 5},
		{Title: "Otro título", URL: "http://www.example.com/x/?utm_source=hn", Source: "HN", Points: 300},
		{Title: "Distinto", URL: "https://example.com/y", Source: "B"},
	}
	out := Dedupe(in)
	if len(out) != 2 {
		t.Fatalf("len = %d, quería 2: %+v", len(out), out)
	}
	if out[0].Source != "HN" || out[1].Title != "Distinto" {
		t.Errorf("resultado inesperado: %+v", out)
	}
}

func TestDedupeByTitle(t *testing.T) {
	in := []model.Article{
		{Title: "OpenAI launches new model!", URL: "https://a.com/1", Source: "A"},
		{Title: "OpenAI Launches New Model", URL: "https://b.com/2", Source: "B"},
	}
	out := Dedupe(in)
	if len(out) != 1 || out[0].Source != "A" {
		t.Errorf("quería solo el primero, salió %+v", out)
	}
}

func TestDedupeNoFalsePositives(t *testing.T) {
	in := []model.Article{
		{Title: "", URL: "https://a.com/1"},
		{Title: "", URL: "https://a.com/2"},
	}
	if out := Dedupe(in); len(out) != 2 {
		t.Errorf("títulos vacíos no deben deduplicar: %+v", out)
	}
}

func TestSortByDate(t *testing.T) {
	now := time.Now()
	a := []model.Article{
		{Title: "viejo", Published: now.Add(-2 * time.Hour)},
		{Title: "nuevo", Published: now},
	}
	SortByDate(a)
	if a[0].Title != "nuevo" {
		t.Errorf("orden incorrecto: %+v", a)
	}
}
