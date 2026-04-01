package handlers

import (
	"html/template"
	"net/http"

	"github.com/govcms/platform-testing/internal/models"
	"github.com/govcms/platform-testing/internal/orchestrator"
	"github.com/govcms/platform-testing/internal/sse"
)

type Handlers struct {
	Store        *models.Store
	Orchestrator *orchestrator.Orchestrator
	Broker       *sse.Broker
	Pages        map[string]*template.Template // per-page templates (layout + partials + page)
	Partials     *template.Template            // partials-only (for rendering fragments)
}

func New(store *models.Store, orch *orchestrator.Orchestrator, broker *sse.Broker, pages map[string]*template.Template, partials *template.Template) *Handlers {
	return &Handlers{
		Store:        store,
		Orchestrator: orch,
		Broker:       broker,
		Pages:        pages,
		Partials:     partials,
	}
}

func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("GET /", h.Dashboard)
	mux.HandleFunc("GET /sites", h.Sites)
	mux.HandleFunc("POST /sites/{id}/toggle", h.ToggleSite)
	mux.HandleFunc("GET /runs", h.Runs)
	mux.HandleFunc("POST /runs", h.CreateRun)
	mux.HandleFunc("GET /runs/{id}", h.RunDetail)
	mux.HandleFunc("POST /runs/{id}/cancel", h.CancelRun)
	mux.HandleFunc("GET /runs/{id}/results/{rid}", h.ResultDetail)

	// SSE
	mux.Handle("GET /sse", h.Broker)

	// Worker API
	mux.HandleFunc("POST /api/worker/poll", h.WorkerPoll)
	mux.HandleFunc("POST /api/worker/result", h.WorkerResult)

	// External API
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/runs", h.APIListRuns)
	mux.HandleFunc("POST /api/runs", h.APICreateRun)
	mux.HandleFunc("GET /api/runs/{id}", h.APIGetRun)

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
