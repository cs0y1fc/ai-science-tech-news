package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"noticias-tech/internal/dedupe"
	"noticias-tech/internal/enrich"
	"noticias-tech/internal/images"
	"noticias-tech/internal/model"
	"noticias-tech/internal/rank"
	"noticias-tech/internal/render"
	"noticias-tech/internal/sources"
	"noticias-tech/internal/store"
	"noticias-tech/web"
)

// enrichTimeout acota toda la fase de IA (descargas + peticiones + pausas).
const enrichTimeout = 10 * time.Minute

// retention es cuánto tiempo se conservan los artículos en data/articles.json.
const retention = 30 * 24 * time.Hour

func main() {
	cfgPath := flag.String("sources", "sources.yaml", "fichero de fuentes")
	dataPath := flag.String("data", "data/articles.json", "estado persistente de artículos")
	outDir := flag.String("out", "public", "directorio de salida del sitio")
	siteURL := flag.String("site-url", os.Getenv("SITE_URL"), "URL pública del sitio; activa feed.xml y sitemap.xml")
	repoURL := flag.String("repo-url", os.Getenv("REPO_URL"), "URL del código fuente, para la página Acerca de")
	flag.Parse()

	if err := run(*cfgPath, *dataPath, *outDir, render.Options{SiteURL: *siteURL, RepoURL: *repoURL}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, dataPath, outDir string, ropts render.Options) error {
	srcs, err := sources.LoadConfig(cfgPath)
	if err != nil {
		return err
	}
	old, err := store.Load(dataPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fetched, errs := sources.NewFetcher().FetchAll(ctx, srcs)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "aviso:", e)
	}
	// Si todo falla pero hay datos guardados, se reconstruye el sitio con ellos.
	if len(fetched) == 0 && len(old) == 0 {
		return fmt.Errorf("no se recogió ningún artículo (%d fuentes con error)", len(errs))
	}

	now := time.Now().UTC()
	unique := dedupe.Dedupe(fetched)
	all := store.Merge(old, unique, now, retention)

	opts := rank.Options{Now: now}
	enrichWithAI(all, opts)
	fillImages(all, opts)
	enrich.ApplyKeywordTags(all)

	if len(fetched) > 0 {
		if err := store.Save(dataPath, all); err != nil {
			return err
		}
	}

	sel := rank.Select(all, opts)
	seen := map[string]bool{}
	for _, src := range srcs {
		if !seen[src.Name] {
			seen[src.Name] = true
			ropts.Sources = append(ropts.Sources, src.Name)
		}
	}
	if err := render.SiteWith(ropts, web.FS, outDir, now, sel, all); err != nil {
		return err
	}

	shown := 0
	for _, r := range sel.Rows {
		shown += len(r.Articles)
	}
	fmt.Printf("%d fuentes (%d con error) · %d recogidos · %d guardados · %d en portada → %s/index.html\n",
		len(srcs), len(errs), len(fetched), len(all), shown, outDir)
	return nil
}

// enrichWithAI pide a la IA resumen, etiquetas y sección de los mejores
// artículos que aún no ha procesado y vuelca el resultado en all. Sin
// LLM_API_KEY, o si la API falla, la web sigue funcionando con el respaldo
// por palabras clave: aquí nunca se devuelve un error que tumbe el build.
//
// Variables de entorno: LLM_API_KEY, LLM_MODEL y, opcionalmente, LLM_BASE_URL
// (por defecto el endpoint compatible con OpenAI de Gemini). Los respaldos
// se definen igual con prefijo LLM2_, LLM3_… (con BASE_URL obligatoria).
func enrichWithAI(all []model.Article, opts rank.Options) {
	client, err := enrich.ChainFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso: IA desactivada:", err)
		return
	}
	if client == nil {
		fmt.Println("IA desactivada (sin LLM_API_KEY): etiquetas por palabras clave")
		return
	}

	var todo []model.Article
	for _, a := range rank.Candidates(all, opts) {
		if !a.Enriched {
			todo = append(todo, a)
		}
	}
	if len(todo) == 0 {
		fmt.Println("IA: nada pendiente")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()
	done, st := enrich.New(client, enrich.NewHTTPPages()).Enrich(ctx, todo)

	byID := make(map[string]model.Article, len(done))
	for _, a := range done {
		byID[a.ID] = a
	}
	for i := range all {
		if a, ok := byID[all[i].ID]; ok {
			all[i] = a
		}
	}
	fmt.Printf("IA: %d intentados · %d con resumen · %d ocultados · %d fallidos", st.Tried, st.Enriched, st.Hidden, st.Failed)
	if st.RateLimited {
		fmt.Print(" · límite de uso alcanzado (se reintentará en la próxima ejecución)")
	}
	fmt.Println()
}

// fillImages busca la imagen de los mejores artículos que no la traen en el
// feed. Va después de la IA, que ya rescata alguna al leer la página.
func fillImages(all []model.Article, opts rank.Options) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	done := images.NewFinder().Fill(ctx, rank.Candidates(all, opts))
	byID := make(map[string]model.Article, len(done))
	found := 0
	for _, a := range done {
		byID[a.ID] = a
		if a.ImageURL != "" {
			found++
		}
	}
	for i := range all {
		if a, ok := byID[all[i].ID]; ok {
			all[i] = a
		}
	}
	if len(done) > 0 {
		fmt.Printf("Imágenes: %d páginas revisadas · %d imágenes encontradas\n", len(done), found)
	}
}
