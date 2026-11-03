package sources

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"noticias-tech/internal/model"
)

// Source describe una fuente configurada en sources.yaml.
type Source struct {
	Name    string        `yaml:"name"`
	URL     string        `yaml:"url"`
	Section model.Section `yaml:"section"`
	// Type es "rss" (RSS o Atom) o "hn" (API de Hacker News).
	Type string `yaml:"type"`
	// Weight multiplica la puntuación de sus artículos; 0 se trata como 1.
	Weight float64 `yaml:"weight"`
	// Limit acota los artículos que se toman por ejecución; 0 usa el valor por defecto.
	Limit int `yaml:"limit"`
}

type configFile struct {
	Sources []Source `yaml:"sources"`
}

var validSections = map[model.Section]bool{
	model.SectionAI:        true,
	model.SectionScience:   true,
	model.SectionTech:      true,
	model.SectionTutorials: true,
}

// LoadConfig lee y valida el fichero de fuentes.
func LoadConfig(path string) ([]Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	return ParseConfig(data)
}

// ParseConfig valida el contenido YAML de las fuentes.
func ParseConfig(data []byte) ([]Source, error) {
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("YAML inválido: %w", err)
	}
	for i := range cfg.Sources {
		s := &cfg.Sources[i]
		if s.Type == "" {
			s.Type = "rss"
		}
		if s.Weight == 0 {
			s.Weight = 1
		}
		if s.Name == "" || s.URL == "" {
			return nil, fmt.Errorf("fuente %d: name y url son obligatorios", i+1)
		}
		if s.Type != "rss" && s.Type != "hn" {
			return nil, fmt.Errorf("fuente %q: tipo desconocido %q", s.Name, s.Type)
		}
		if !validSections[s.Section] {
			return nil, fmt.Errorf("fuente %q: sección desconocida %q", s.Name, s.Section)
		}
	}
	return cfg.Sources, nil
}
