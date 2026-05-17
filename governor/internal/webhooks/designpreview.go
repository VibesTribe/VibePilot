package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// DesignPreviewer is the interface for the design preview system.
type DesignPreviewer interface {
	Generate(ctx context.Context, req DesignPreviewRequest) (*DesignPreviewResponse, error)
	Approve(ctx context.Context, previewID, reviewer, notes string) error
	Reject(ctx context.Context, previewID, reviewer, notes string) error
	GetEnabled() bool
}

// DesignPreviewRequest is the input for generating a design preview.
type DesignPreviewRequest struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url,omitempty"`
	DesignHints string `json:"design_hints,omitempty"`
}

// DesignPreviewResponse is the output of a design preview generation.
type DesignPreviewResponse struct {
	PreviewID   string `json:"preview_id"`
	HTMLContent string `json:"html_content"`
	FilePath    string `json:"file_path"`
	Model       string `json:"model"`
	TokensUsed  int    `json:"tokens_used"`
}

// SetDesignPreview sets the Design Preview engine on the server.
func (s *Server) SetDesignPreview(previewer DesignPreviewer) {
	s.designPreview = previewer
}

// handleDesignPreviewGenerate generates a new design mockup for a task.
// POST /api/design-preview/generate
func (s *Server) handleDesignPreviewGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.designPreview == nil || !s.designPreview.GetEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Design Preview is not enabled",
		})
		return
	}

	var req DesignPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.TaskID == "" || req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task_id and title are required"})
		return
	}

	resp, err := s.designPreview.Generate(r.Context(), req)
	if err != nil {
		log.Printf("[DesignPreview] Generate failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[DesignPreview] Generated preview %s for task %s (%d tokens)", resp.PreviewID, req.TaskID, resp.TokensUsed)

	// Insert into unified review_items for the review hub
	if s.db != nil {
		existing, _ := s.db.Query(r.Context(), "review_items", map[string]any{
			"type":      "design_preview",
			"source_id": resp.PreviewID,
			"status":    "pending",
		})
		var dupes []map[string]any
		if existing == nil || json.Unmarshal(existing, &dupes) != nil || len(dupes) == 0 {
			payload, _ := json.Marshal(map[string]any{
				"task_id":    req.TaskID,
				"file_path":  resp.FilePath,
				"model":      resp.Model,
			})
			s.db.Insert(r.Context(), "review_items", map[string]any{
				"type":      "design_preview",
				"source_id": resp.PreviewID,
				"title":     "Design preview: " + req.Title,
				"summary":   fmt.Sprintf("Design mockup generated (%d tokens). Awaiting review.", resp.TokensUsed),
				"payload":   string(payload),
				"status":    "pending",
				"priority":  "medium",
			})
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleDesignPreviewApprove approves a design preview.
// POST /api/design-preview/approve
func (s *Server) handleDesignPreviewApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.designPreview == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Design Preview not available"})
		return
	}

	var body struct {
		PreviewID string `json:"preview_id"`
		Reviewer  string `json:"reviewer"`
		Notes     string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.designPreview.Approve(r.Context(), body.PreviewID, body.Reviewer, body.Notes); err != nil {
		log.Printf("[DesignPreview] Approve failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[DesignPreview] Approved preview %s by %s", body.PreviewID, body.Reviewer)
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// handleDesignPreviewReject rejects a design preview with feedback.
// POST /api/design-preview/reject
func (s *Server) handleDesignPreviewReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.designPreview == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Design Preview not available"})
		return
	}

	var body struct {
		PreviewID string `json:"preview_id"`
		Reviewer  string `json:"reviewer"`
		Notes     string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.designPreview.Reject(r.Context(), body.PreviewID, body.Reviewer, body.Notes); err != nil {
		log.Printf("[DesignPreview] Reject failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[DesignPreview] Rejected preview %s by %s: %s", body.PreviewID, body.Reviewer, body.Notes)
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// handleDesignPreviewList returns design previews for a task.
// GET /api/design-preview/list?task_id=xxx
func (s *Server) handleDesignPreviewList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task_id query parameter required"})
		return
	}

	if s.db == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	data, err := s.db.Query(r.Context(), "design_previews", map[string]any{
		"task_id": taskID,
		"select":  "id,task_id,status,file_path,reviewer,review_notes,version,created_at,reviewed_at",
		"order":   "created_at.desc",
		"limit":   20,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Query failed: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
