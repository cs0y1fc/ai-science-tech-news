package main

import (
	"testing"

	"noticias-tech/internal/model"
)

func TestMatchTitles(t *testing.T) {
	all := []model.Article{
		{ID: "1", Title: "New Linux kernel released"},
		{ID: "2", Title: "Quantum batteries"},
		{ID: "3", Title: "Best iPhone deals", Skip: true},
	}
	got, err := matchTitles(all, `linux|iphone`)
	if err != nil || !got["1"] || got["2"] || !got["3"] {
		t.Errorf("got %v err=%v: debe incluir también los ocultos", got, err)
	}
	if got, _ := matchTitles(all, ""); len(got) != 0 {
		t.Error("sin expresión no se reprocesa nada")
	}
	if _, err := matchTitles(all, "("); err == nil {
		t.Error("una expresión inválida debe dar error")
	}
}
