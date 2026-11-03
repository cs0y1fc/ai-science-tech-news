package dedupe

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"noticias-tech/internal/model"
)

// trackingParams son parámetros de seguimiento que no cambian el artículo.
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "ref": true, "ref_src": true, "mc_cid": true, "mc_eid": true,
}

// NormalizeURL deja la URL en una forma canónica para compararla: esquema y
// host en minúsculas, sin "www.", sin fragmento, sin parámetros de seguimiento
// y sin barra final.
func NormalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Scheme = "https"
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(k, "utm_") || trackingParams[k] {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}

// ID devuelve un identificador estable y corto para una URL.
func ID(rawURL string) string {
	sum := sha1.Sum([]byte(NormalizeURL(rawURL)))
	return hex.EncodeToString(sum[:])[:12]
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// titleKey reduce un título a sus palabras en minúsculas, sin puntuación.
func titleKey(title string) string {
	return strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(title), " "))
}

// Dedupe elimina artículos repetidos por URL normalizada o por título
// idéntico. Entre duplicados conserva el de más puntos y, a igualdad, el más
// antiguo en la lista de entrada. El orden relativo de los supervivientes se
// mantiene.
func Dedupe(in []model.Article) []model.Article {
	keep := make([]bool, len(in))
	byURL := map[string]int{}
	byTitle := map[string]int{}

	better := func(a, b int) bool { return in[a].Points > in[b].Points }

	for i, a := range in {
		keep[i] = true
		uk, tk := NormalizeURL(a.URL), titleKey(a.Title)
		for _, m := range []struct {
			idx map[string]int
			key string
		}{{byURL, uk}, {byTitle, tk}} {
			if m.key == "" {
				continue
			}
			j, seen := m.idx[m.key]
			if !seen {
				m.idx[m.key] = i
				continue
			}
			// Ya había uno igual: se queda el mejor de los dos.
			if keep[j] && better(i, j) {
				keep[j] = false
				m.idx[m.key] = i
			} else {
				keep[i] = false
			}
			// Un artículo ya descartado no debe registrarse con su título.
			if !keep[i] {
				break
			}
		}
	}

	out := make([]model.Article, 0, len(in))
	for i, a := range in {
		if keep[i] {
			out = append(out, a)
		}
	}
	return out
}

// SortByDate ordena de más reciente a más antiguo, de forma estable.
func SortByDate(a []model.Article) {
	sort.SliceStable(a, func(i, j int) bool { return a[i].Published.After(a[j].Published) })
}
