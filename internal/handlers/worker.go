package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

type PollResponse struct {
	ResultID  int    `json:"result_id"`
	RunID     int    `json:"run_id"`
	SiteID    int    `json:"site_id"`
	SiteName  string `json:"site_name"`
	CanaryURL string `json:"canary_url"`
}

type ResultSubmission struct {
	ResultID    int    `json:"result_id"`
	Status      string `json:"status"`
	ExitCode    int    `json:"exit_code"`
	TestsTotal  int    `json:"tests_total"`
	TestsPassed int    `json:"tests_passed"`
	TestsFailed int    `json:"tests_failed"`
	DurationMs  int    `json:"duration_ms"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ErrorMsg    string `json:"error_msg"`
}

func (h *Handlers) WorkerPoll(w http.ResponseWriter, r *http.Request) {
	result, err := h.Store.ClaimPendingResult()
	if err != nil {
		log.Printf("Worker poll error: %v", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	if result == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PollResponse{
		ResultID:  result.ID,
		RunID:     result.RunID,
		SiteID:    result.SiteID,
		SiteName:  result.SiteName,
		CanaryURL: result.CanaryURL,
	})
}

func (h *Handlers) WorkerResult(w http.ResponseWriter, r *http.Request) {
	var sub ResultSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	err := h.Store.SubmitResult(
		sub.ResultID, sub.Status, sub.ExitCode,
		sub.TestsTotal, sub.TestsPassed, sub.TestsFailed,
		sub.DurationMs, sub.Stdout, sub.Stderr, sub.ErrorMsg,
	)
	if err != nil {
		log.Printf("Error submitting result %d: %v", sub.ResultID, err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	// Publish SSE events
	h.Orchestrator.PublishSiteComplete(sub.ResultID)

	// Get the run ID from the result to publish progress
	result, err := h.Store.GetResult(sub.ResultID)
	if err == nil {
		h.Orchestrator.PublishProgress(result.RunID)

		// Check if run just completed
		run, err := h.Store.GetRun(result.RunID)
		if err == nil && run.Status == "completed" {
			h.Orchestrator.PublishRunComplete(run.ID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
