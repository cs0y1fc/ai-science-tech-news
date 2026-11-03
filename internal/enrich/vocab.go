package enrich

import (
	"regexp"
	"strings"
)

// Vocabulary es el conjunto cerrado de etiquetas. Que sea cerrado mantiene las
// etiquetas coherentes entre ejecuciones y evita que la IA invente variantes
// ("LLM", "llms", "large-language-models").
var Vocabulary = []string{
	"llm", "agents", "rag", "prompting", "multimodal", "open-source", "safety", "policy",
	"research", "startups", "chips", "hardware", "robotics", "cloud", "data", "security",
	"web", "mobile", "gaming", "devtools", "tutorial",
	"go", "python", "javascript", "rust",
	"space", "physics", "biology", "health", "climate", "energy", "math",
}

var vocabSet = func() map[string]bool {
	m := make(map[string]bool, len(Vocabulary))
	for _, t := range Vocabulary {
		m[t] = true
	}
	return m
}()

// CleanTags deja solo etiquetas del vocabulario, sin repetir y como mucho max.
func CleanTags(in []string, max int) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if !vocabSet[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == max {
			break
		}
	}
	return out
}

// keywordRules es el respaldo cuando la IA no está disponible. Es burdo a
// propósito: sirve para que la portada tenga etiquetas, no para acertar siempre.
var keywordRules = []struct {
	tag string
	re  *regexp.Regexp
}{
	{"llm", regexp.MustCompile(`(?i)\b(llms?|gpt|chatgpt|claude|gemini|language models?|openai|anthropic)\b`)},
	{"agents", regexp.MustCompile(`(?i)\b(agents?|agentic)\b`)},
	{"rag", regexp.MustCompile(`(?i)\b(rag|retrieval[- ]augmented|embeddings?|vector (db|database|search))\b`)},
	{"prompting", regexp.MustCompile(`(?i)\bprompt(s|ing)?\b`)},
	{"multimodal", regexp.MustCompile(`(?i)\b(multimodal|text-to-image|text-to-video|diffusion)\b`)},
	{"open-source", regexp.MustCompile(`(?i)\bopen[- ]?source(d)?\b`)},
	{"safety", regexp.MustCompile(`(?i)\b(ai safety|alignment|jailbreak)\b`)},
	{"policy", regexp.MustCompile(`(?i)\b(regulation|lawsuit|antitrust|legislation|copyright)\b`)},
	{"startups", regexp.MustCompile(`(?i)\b(startup|raises \$|funding round|series [a-d])\b`)},
	{"chips", regexp.MustCompile(`(?i)\b(chips?|gpus?|nvidia|semiconductors?|tsmc)\b`)},
	{"robotics", regexp.MustCompile(`(?i)\b(robots?|robotics|humanoid)\b`)},
	{"cloud", regexp.MustCompile(`(?i)\b(aws|azure|cloud|kubernetes|serverless)\b`)},
	{"security", regexp.MustCompile(`(?i)\b(cyber-?attack|vulnerabilit(y|ies)|ransomware|malware|breach|exploit)\b`)},
	{"web", regexp.MustCompile(`(?i)\b(browser|css|html|react|next\.?js|frontend)\b`)},
	{"mobile", regexp.MustCompile(`(?i)\b(iphone|android|ios|smartphone)\b`)},
	{"gaming", regexp.MustCompile(`(?i)\b(games?|gaming|playstation|xbox|steam)\b`)},
	{"go", regexp.MustCompile(`\b(Go|Golang|golang)\b`)},
	{"python", regexp.MustCompile(`(?i)\bpython\b`)},
	{"javascript", regexp.MustCompile(`(?i)\b(javascript|typescript|node\.?js)\b`)},
	{"rust", regexp.MustCompile(`(?i)\brust\b`)},
	{"tutorial", regexp.MustCompile(`(?i)\b(tutorial|how to|guide|step[- ]by[- ]step|walkthrough)\b`)},
	{"space", regexp.MustCompile(`(?i)\b(nasa|space|galax(y|ies)|planets?|telescope|asteroid|mars|moon)\b`)},
	{"physics", regexp.MustCompile(`(?i)\b(physics|quantum|particles?|neutrinos?)\b`)},
	{"biology", regexp.MustCompile(`(?i)\b(biology|species|genes?|genetic|dna|evolution)\b`)},
	{"health", regexp.MustCompile(`(?i)\b(cancer|disease|vaccines?|medical|patients?|clinical)\b`)},
	{"climate", regexp.MustCompile(`(?i)\b(climate|warming|emissions|carbon)\b`)},
	{"energy", regexp.MustCompile(`(?i)\b(energy|solar|battery|batteries|nuclear|fusion)\b`)},
	{"math", regexp.MustCompile(`(?i)\b(mathematic(s|ian)|theorem|proof)\b`)},
}

// KeywordTags etiqueta un texto por palabras clave, con un máximo de max.
func KeywordTags(text string, max int) []string {
	var out []string
	for _, r := range keywordRules {
		if r.re.MatchString(text) {
			out = append(out, r.tag)
			if len(out) == max {
				break
			}
		}
	}
	return out
}
