package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// VisualQARunner is the interface the webhook handler needs to trigger VQA runs.
// Returns the raw result as any to avoid import cycles with the visualqa package.
type VisualQARunner interface {
	Run(ctx context.Context, triggeredBy, triggerDetail string) (any, error)
	GetEnabled() bool
}

// handleVisualQARun triggers a Visual QA run.
// POST /api/visualqa/run
// Body: {"trigger": "manual", "detail": "optional detail"}  (both optional)
func (s *Server) handleVisualQARun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.visualQA == nil || !s.visualQA.GetEnabled() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "disabled",
			"message": "Visual QA is not enabled or not configured",
		})
		return
	}

	// Parse optional body
	trigger := "api"
	detail := ""
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			var req struct {
				Trigger string `json:"trigger"`
				Detail  string `json:"detail"`
			}
			if json.Unmarshal(body, &req) == nil {
				if req.Trigger != "" {
					trigger = req.Trigger
				}
				detail = req.Detail
			}
		}
	}

	log.Printf("[VisualQA] Run triggered via API: trigger=%s detail=%s", trigger, detail)

	// Run synchronously with timeout
	type runOutcome struct {
		result any
		err    error
	}
	done := make(chan runOutcome, 1)

	go func() {
		result, err := s.visualQA.Run(r.Context(), trigger, detail)
		done <- runOutcome{result: result, err: err}
	}()

	select {
	case outcome := <-done:
		if outcome.err != nil {
			log.Printf("[VisualQA] Run failed: %v", outcome.err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"status": "failed",
				"error":  outcome.err.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"result": outcome.result,
		})
	case <-time.After(180 * time.Second):
		log.Printf("[VisualQA] Run timed out after 180s")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGatewayTimeout)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "timeout",
			"error":  "Visual QA run timed out after 180 seconds",
		})
	}
}

// handleVisualQAStatus returns the latest VQA run status.
// GET /api/visualqa/status
func (s *Server) handleVisualQAStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	rows, err := s.db.Query(ctx, "visual_qa_runs", map[string]any{
		"order": "started_at.desc",
		"limit": 10,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": fmt.Sprintf("Failed to query VQA runs: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"enabled": s.visualQA != nil && s.visualQA.GetEnabled(),
		"runs":    json.RawMessage(rows),
	})
}

// SetVisualQA sets the Visual QA runner on the server.
func (s *Server) SetVisualQA(runner VisualQARunner) {
	s.visualQA = runner
}
