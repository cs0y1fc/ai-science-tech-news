package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"noticias-tech/internal/model"
)

// Load lee los artículos guardados. Si el fichero no existe devuelve una
// lista vacía, que es lo normal en la primera ejecución.
func Load(path string) ([]model.Article, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var arts []model.Article
	if err := json.Unmarshal(data, &arts); err != nil {
		return nil, fmt.Errorf("%s: JSON inválido: %w", path, err)
	}
	// Los datos antiguos usaban nombres de sección en español.
	for i := range arts {
		arts[i].Section = arts[i].Section.Normalize()
	}
	return arts, nil
}

// Save escribe los artículos de forma atómica (fichero temporal + rename)
// para que una ejecución interrumpida no deje el JSON a medias.
func Save(path string, arts []model.Article) error {
	data, err := json.MarshalIndent(arts, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".articles-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp usa 0600; el JSON se versiona y se lee desde otros procesos.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Merge combina lo guardado con lo recién recogido. Los datos frescos (puntos,
// imagen, extracto) sustituyen a los viejos, pero se conservan FirstSeen y lo
// que haya decidido la IA (sección, resumen, etiquetas, si se oculta), para no
// repetir trabajo. Los
// artículos publicados hace más de retention se descartan. El resultado va de
// más reciente a más antiguo.
func Merge(old, fresh []model.Article, now time.Time, retention time.Duration) []model.Article {
	byID := make(map[string]model.Article, len(old)+len(fresh))
	for _, a := range old {
		byID[a.ID] = a
	}
	for _, a := range fresh {
		prev, seen := byID[a.ID]
		if seen {
			a.FirstSeen = prev.FirstSeen
			// Solo lo que ya procesó la IA es definitivo; las etiquetas por
			// palabras clave se recalculan en cada ejecución.
			if prev.Enriched {
				a.Section = prev.Section
				a.Summary = prev.Summary
				a.Tags = prev.Tags
				a.Enriched = true
				a.Skip = prev.Skip
			}
			if a.ImageURL == "" {
				a.ImageURL = prev.ImageURL
			}
			a.ImageChecked = a.ImageChecked || prev.ImageChecked
		} else {
			a.FirstSeen = now
		}
		byID[a.ID] = a
	}

	out := make([]model.Article, 0, len(byID))
	for _, a := range byID {
		if now.Sub(a.Published) <= retention {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Published.Equal(out[j].Published) {
			return out[i].Published.After(out[j].Published)
		}
		return out[i].ID < out[j].ID
	})
	return out
}
