package handlers

import (
	"log"
	"net/http"
	"strconv"
)

func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	run, err := h.Store.LatestRun()
	if err != nil {
		log.Printf("Error fetching latest run: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Run": run,
	}

	if run != nil {
		results, err := h.Store.ListResultsForRun(run.ID)
		if err != nil {
			log.Printf("Error fetching results: %v", err)
		}
		data["Results"] = results
	}

	h.render(w, "dashboard.html", data)
}

func (h *Handlers) Sites(w http.ResponseWriter, r *http.Request) {
	sites, err := h.Store.ListSites()
	if err != nil {
		log.Printf("Error fetching sites: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, "sites.html", map[string]any{"Sites": sites})
}

func (h *Handlers) ToggleSite(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid site ID", http.StatusBadRequest)
		return
	}

	site, err := h.Store.ToggleSite(id)
	if err != nil {
		log.Printf("Error toggling site %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.Partials.ExecuteTemplate(w, "site_row.html", site)
}

func (h *Handlers) Runs(w http.ResponseWriter, r *http.Request) {
	runs, err := h.Store.ListRuns()
	if err != nil {
		log.Printf("Error fetching runs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, "runs.html", map[string]any{"Runs": runs})
}

func (h *Handlers) CreateRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.Orchestrator.StartRun()
	if err != nil {
		log.Printf("Error creating run: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, "No active sites", http.StatusBadRequest)
		return
	}

	if isHTMX(r) {
		results, _ := h.Store.ListResultsForRun(run.ID)
		h.Partials.ExecuteTemplate(w, "run_detail_content.html", map[string]any{
			"Run":     run,
			"Results": results,
		})
		return
	}

	http.Redirect(w, r, "/runs/"+strconv.Itoa(run.ID), http.StatusSeeOther)
}

func (h *Handlers) RunDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := h.Store.GetRun(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	results, err := h.Store.ListResultsForRun(id)
	if err != nil {
		log.Printf("Error fetching results for run %d: %v", id, err)
	}

	h.render(w, "run_detail.html", map[string]any{
		"Run":     run,
		"Results": results,
	})
}

func (h *Handlers) CancelRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	if err := h.Store.CancelRun(id); err != nil {
		log.Printf("Error cancelling run %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.Orchestrator.PublishRunComplete(id)

	if isHTMX(r) {
		run, _ := h.Store.GetRun(id)
		results, _ := h.Store.ListResultsForRun(id)
		h.Partials.ExecuteTemplate(w, "run_detail_content.html", map[string]any{
			"Run":     run,
			"Results": results,
		})
		return
	}

	http.Redirect(w, r, "/runs/"+strconv.Itoa(id), http.StatusSeeOther)
}

func (h *Handlers) ResultDetail(w http.ResponseWriter, r *http.Request) {
	rid, err := strconv.Atoi(r.PathValue("rid"))
	if err != nil {
		http.Error(w, "Invalid result ID", http.StatusBadRequest)
		return
	}

	result, err := h.Store.GetResult(rid)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h.render(w, "result_detail.html", map[string]any{"Result": result})
}

func (h *Handlers) render(w http.ResponseWriter, name string, data any) {
	t, ok := h.Pages[name]
	if !ok {
		log.Printf("Template %s not found", name)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("Error rendering %s: %v", name, err)
	}
}
