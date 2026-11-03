// Package web contiene las plantillas y los estáticos, incrustados en el binario.
package web

import "embed"

//go:embed templates static
var FS embed.FS
