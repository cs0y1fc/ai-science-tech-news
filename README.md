# Tech News

Web de noticias de **inteligencia artificial, ciencia y tecnología**, más **tutoriales** de programación y de integración de IA en aplicaciones. Se regenera sola cada 6 horas. Toda la web (interfaz y contenido) está en inglés; este README y los comentarios del código, en español.

Es un sitio **estático** generado por un programa en Go y publicado en GitHub Pages. No hay servidor, base de datos ni JavaScript propio: solo HTML y CSS.

## Qué hace

1. **Recoge** las últimas noticias de 36 fuentes gratuitas (feeds RSS y la API pública de Hacker News).
2. **Deduplica** por URL normalizada y por título.
3. **Puntúa** cada noticia y elige la destacada y las mejores de cada sección.
4. **Enriquece con IA** (capa gratuita): resumen corto, sección y etiquetas. Oculta lo que no trata de estos temas.
5. **Busca imagen** de cada noticia (`og:image`) cuando el feed no la trae.
6. **Genera** el sitio: portada, secciones, etiquetas, una página por noticia, RSS, sitemap y la página "About".

## Páginas

| Ruta | Contenido |
|---|---|
| `/` | Noticia destacada, temas del momento y todas las noticias por orden de llegada (más recientes primero), con pestañas por sección (AI, Science, Technology, Coding, Phones, Tutorials) y paginación (24 por página) |
| `/s/{section}/` (`ai`, `science`, `tech`, `coding`, `phones`, `tutorials`) | Lista paginada de una sección, con sus temas |
| `/t/{tag}/` | Todo lo de una etiqueta |
| `/n/{id}/` | Resumen de una noticia, con enlace al original y noticias relacionadas |
| `/about/` | Cómo se genera, límites de los resúmenes y lista de fuentes |
| `/feed.xml` | RSS con las 50 últimas |
| `/sitemap.xml` | Mapa del sitio |

Las rutas son relativas, así que el sitio funciona bajo una subruta como `usuario.github.io/repo/`.

## Estructura

```
cmd/build/            Programa principal: orquesta todo el proceso
internal/
  sources/            Lee sources.yaml y descarga feeds (RSS y Hacker News)
  dedupe/             Normaliza URLs, genera IDs y elimina duplicados
  rank/               Puntuación y elección de la noticia destacada
  enrich/             IA: cliente compatible con OpenAI, cadena de respaldo,
                      vocabulario de etiquetas, extracción del texto de la página
  images/             Busca la imagen principal (og:image) de cada artículo
  store/              Estado persistente en data/articles.json
  render/             Genera el HTML, el RSS y el sitemap
  model/              Tipo Article y secciones
web/
  templates/          Plantillas HTML (html/template)
  static/style.css    Estilos (tema claro por defecto; oscuro con el botón "Theme")
sources.yaml          Lista de fuentes: nombre, URL, sección, tipo, peso, límite
data/articles.json    Artículos guardados (30 días); lo actualiza el workflow
.github/workflows/    Actualización y publicación cada 6 horas
```

## Cómo funciona

### Puntuación

```
puntos = peso_fuente · (10·e^(−horas/24) + 2·ln(1 + puntos_HN)) + 3·min(cobertura, 3)
```

- **Frescura:** decae con el tiempo.
- **Popularidad:** puntos en Hacker News.
- **Cobertura:** cuántas *otras* fuentes cuentan lo mismo (similitud de títulos ≥ 0,5).
- **Peso:** cada fuente tiene el suyo en `sources.yaml`.

La puntuación solo decide la **noticia destacada** (la mejor de los últimos 3 días que no sea tutorial, con preferencia por las que tienen imagen). El resto de la portada no se ordena por puntuación ni por etiqueta: va **por orden de llegada, las más recientes primero**, y se pagina.

### IA

- Habla con cualquier API compatible con *Chat Completions* de OpenAI (Gemini, Groq, Mistral…).
- Procesa los artículos nuevos en lotes de 5, hasta 30 por ejecución, y guarda el resultado: no repite trabajo.
- Las etiquetas salen de un **vocabulario cerrado** de unas 30 (`llm`, `agents`, `space`, `go`, `tutorial`…) para que sean coherentes.
- La sección debe ser una de `ai`, `science`, `tech`, `coding`, `phones`, `tutorials` o `none`; con `none` la noticia se oculta, y también si no está escrita en inglés.
- **Cadena de respaldo:** si el proveedor principal falla o agota su cuota, la misma petición pasa al siguiente. Un proveedor limitado, o que falla 3 veces seguidas, se salta el resto de la ejecución.
- **Sin IA o si falla:** la web sigue funcionando con el extracto del feed y etiquetas por palabras clave. Nunca queda vacía.
- Sin texto de origen no se genera resumen, para no inventarlo.

### Seguridad frente a prompt injection

El texto de los artículos es contenido no fiable. Se envía a la IA como datos JSON, con un mensaje de sistema que le indica que no obedezca instrucciones que aparezcan dentro. Además, la salida se valida: secciones y etiquetas solo pueden tomar valores permitidos, y las plantillas escapan todo el HTML.

### Derechos

Solo se muestra un resumen propio y corto, con la fuente y un enlace visible al original. No se republican artículos completos. Las imágenes se cargan desde los servidores de cada fuente.

## Ejecutar en local

Requisitos: Go 1.26 o superior.

```bash
go test ./...                     # tests
go run ./cmd/build                # genera public/ (sin IA: etiquetas por palabras clave)
python3 -m http.server -d public  # verlo en http://localhost:8000
```

Opciones de `cmd/build`: `-sources`, `-data`, `-out`, `-site-url` y `-repo-url`. Con `-site-url` (o `SITE_URL`) se generan también `feed.xml`, `sitemap.xml` y las URL canónicas.

### Activar la IA

Define estas variables de entorno (un ejemplo con Gemini):

```bash
export LLM_API_KEY=...                  # tu clave; nunca se guarda en el repositorio
export LLM_MODEL=gemini-flash-lite-latest
# LLM_BASE_URL es opcional; por defecto, el endpoint compatible con OpenAI de Gemini
```

Proveedores de respaldo, en orden, con el prefijo `LLM2_`, `LLM3_`… (hasta `LLM9_`). En los respaldos `BASE_URL` es obligatoria:

```bash
export LLM2_API_KEY=... LLM2_MODEL=openai/gpt-oss-120b LLM2_BASE_URL=https://api.groq.com/openai/v1
```

Los nombres de modelo de las capas gratuitas cambian con el tiempo; revisa la documentación de tu proveedor.

## Publicación

El workflow `.github/workflows/update.yml` se ejecuta cada 6 horas (`0 */6 * * *`), a mano y en cada push a `main`:

1. Pasa los tests.
2. Genera el sitio.
3. Guarda `data/articles.json` en el repositorio (commit del bot con `[skip ci]`).
4. Publica `public/` en GitHub Pages.

Configuración necesaria en el repositorio (*Settings*):

| Dónde | Nombre | Para qué |
|---|---|---|
| Secrets | `LLM_API_KEY`, `LLM2_API_KEY`, `LLM3_API_KEY` | Claves de la IA (principal y respaldos) |
| Variables | `LLM_MODEL`, `LLM2_MODEL`, `LLM2_BASE_URL`, `LLM3_MODEL`, `LLM3_BASE_URL` | Modelos y URLs (no son secretos) |
| Pages | Origen: *GitHub Actions* | Publicación |

`SITE_URL` y `REPO_URL` las calcula el propio workflow.

## Añadir o quitar fuentes

Edita `sources.yaml`; no hace falta tocar código:

```yaml
- name: Nombre visible
  url: https://ejemplo.com/feed.xml
  section: tech          # ai | science | tech | coding | phones | tutorials
  type: rss              # rss (por defecto) | hn
  weight: 1.0            # relevancia relativa (por defecto 1)
  limit: 20              # máximo de artículos por ejecución
```

## Límites conocidos

- Los resúmenes son automáticos y pueden contener errores.
- Solo ~45–50 % de las noticias tienen imagen; cada ejecución revisa 60 páginas más.
- Las capas gratuitas de IA tienen cuotas: si se agotan, esa ejecución se queda con extractos y se reintenta en la siguiente.
- Algunos extractos de las fuentes son pobres hasta que la IA los resume.

## Licencia

Sin licencia definida todavía. Si se publica con intención de que otros lo reutilicen, conviene añadir una (por ejemplo MIT).
