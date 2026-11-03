package enrich

import (
	"strings"
	"testing"
)

func TestCleanTags(t *testing.T) {
	got := CleanTags([]string{" LLM ", "llm", "nope", "Rust", "go", "web", "cloud"}, 3)
	if strings.Join(got, ",") != "llm,rust,go" {
		t.Errorf("got %v", got)
	}
	if CleanTags(nil, 3) != nil {
		t.Error("entrada vacía debe dar nil")
	}
}

func TestVocabularyIsCleanAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range Vocabulary {
		if v != strings.ToLower(v) || strings.ContainsAny(v, " /?#") {
			t.Errorf("etiqueta %q no es un slug válido (se usará en URLs)", v)
		}
		if seen[v] {
			t.Errorf("etiqueta repetida: %q", v)
		}
		seen[v] = true
	}
}

func TestEveryKeywordRuleTargetsAVocabularyTag(t *testing.T) {
	for _, r := range keywordRules {
		if !vocabSet[r.tag] {
			t.Errorf("la regla %q no está en el vocabulario", r.tag)
		}
	}
}

func TestKeywordTags(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"NASA telescope finds new galaxy", "space"},
		{"Building a RAG chatbot with Python: a step-by-step guide", "rag,python,tutorial"},
		{"Why we moved our service to Go", "go"},
		{"Cat videos are great", ""},
		{"Rust and Python and Go and JavaScript", "go,python,javascript"}, // máximo 3, por orden de regla
	}
	for _, c := range cases {
		if got := strings.Join(KeywordTags(c.text, 3), ","); got != c.want {
			t.Errorf("KeywordTags(%q) = %q, quería %q", c.text, got, c.want)
		}
	}
}

func TestKeywordGoRuleIsCaseSensitive(t *testing.T) {
	if tags := KeywordTags("let's go to the park", 3); len(tags) != 0 {
		t.Errorf("el verbo 'go' no debe etiquetar el lenguaje: %v", tags)
	}
}
