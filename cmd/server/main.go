package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/govcms/platform-testing/internal/config"
	"github.com/govcms/platform-testing/internal/db"
	"github.com/govcms/platform-testing/internal/handlers"
	"github.com/govcms/platform-testing/internal/models"
	"github.com/govcms/platform-testing/internal/orchestrator"
	"github.com/govcms/platform-testing/internal/sse"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("Running migrations: %v", err)
	}
	log.Println("Database migrations complete")

	store := models.NewStore(database)

	// Sync sites from YAML file
	if _, err := os.Stat(cfg.SitesFile); err == nil {
		sites, err := config.LoadSites(cfg.SitesFile)
		if err != nil {
			log.Fatalf("Loading sites file: %v", err)
		}
		entries := make([]models.SiteEntry, len(sites))
		for i, s := range sites {
			entries[i] = models.SiteEntry{Name: s.Name, CanaryURL: s.CanaryURL, Active: s.Active}
		}
		if err := store.SyncSites(entries); err != nil {
			log.Fatalf("Syncing sites: %v", err)
		}
		log.Printf("Synced %d sites from %s", len(sites), cfg.SitesFile)
	} else {
		log.Printf("No sites file found at %s, skipping sync", cfg.SitesFile)
	}

	// Template functions
	funcMap := template.FuncMap{
		"mulf": func(a, b int) float64 { return float64(a) * float64(b) },
		"divf": func(a float64, b int) float64 {
			if b == 0 {
				return 0
			}
			return a / float64(b)
		},
	}

	// Parse partials + layout as the base template set.
	// Each page gets its own clone so "content" blocks don't collide.
	baseFiles := []string{
		filepath.Join("web", "templates", "layout.html"),
	}
	partialFiles, _ := filepath.Glob(filepath.Join("web", "templates", "partials", "*.html"))
	baseFiles = append(baseFiles, partialFiles...)

	base := template.Must(template.New("").Funcs(funcMap).ParseFiles(baseFiles...))

	// Build per-page templates
	pageFiles, _ := filepath.Glob(filepath.Join("web", "templates", "*.html"))
	pages := make(map[string]*template.Template)
	for _, pf := range pageFiles {
		name := filepath.Base(pf)
		if name == "layout.html" {
			continue
		}
		t := template.Must(template.Must(base.Clone()).ParseFiles(pf))
		pages[name] = t
	}

	// Partials-only template for rendering fragments (HTMX swaps, SSE events)
	partials := template.Must(template.New("").Funcs(funcMap).ParseFiles(append(partialFiles,
		filepath.Join("web", "templates", "run_detail.html"),
	)...))

	broker := sse.NewBroker()
	orch := orchestrator.New(store, broker, partials)
	h := handlers.New(store, orch, broker, pages, partials)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
