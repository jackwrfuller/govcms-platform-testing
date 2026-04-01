package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handlers) APIListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.Store.ListRuns()
	if err != nil {
		log.Printf("API error listing runs: %v", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

func (h *Handlers) APICreateRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.Orchestrator.StartRun()
	if err != nil {
		log.Printf("API error creating run: %v", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, `{"error":"no active sites"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(run)
}

func (h *Handlers) APIGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	run, err := h.Store.GetRun(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	results, _ := h.Store.ListResultsForRun(id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"run":     run,
		"results": results,
	})
}
