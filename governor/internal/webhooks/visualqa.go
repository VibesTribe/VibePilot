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
	ApproveBaseline(ctx context.Context, pageName string, viewportWidth int) error
}

// SetVisualQA sets the Visual QA runner on the server.
func (s *Server) SetVisualQA(runner VisualQARunner) {
	s.visualQA = runner
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
		writeJSON(w, http.StatusOK, map[string]any{
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

	// Run in background goroutine with timeout
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, err := s.visualQA.Run(ctx, trigger, detail)
		if err != nil {
			log.Printf("[VisualQA] Run failed: %v", err)
			return
		}
		log.Printf("[VisualQA] Run completed: %+v", result)
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "started",
		"triggered_by": trigger,
	})
}

// handleVisualQAStatus returns the latest VQA run status.
// GET /api/visualqa/status
func (s *Server) handleVisualQAStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	data, err := s.db.Query(r.Context(), "visual_qa_runs", map[string]any{
		"order": "started_at.desc",
		"limit": 20,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Failed to query VQA runs: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleVisualQAApprove approves a new baseline from a recent capture.
// POST /api/visualqa/approve
// Body: {"page_name": "...", "viewport": 1280}
func (s *Server) handleVisualQAApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.visualQA == nil || !s.visualQA.GetEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{"error": "Visual QA is not enabled"})
		return
	}

	body, _ := io.ReadAll(r.Body)
	var req struct {
		PageName string `json:"page_name"`
		Viewport int    `json:"viewport"`
	}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}
	if req.PageName == "" || req.Viewport == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "page_name and viewport required"})
		return
	}

	if err := s.visualQA.ApproveBaseline(r.Context(), req.PageName, req.Viewport); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "approved",
		"page_name": req.PageName,
		"viewport":  req.Viewport,
	})
}

// writeJSON is a helper that writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
