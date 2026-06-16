package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleTaskPause pauses a single task by ID.
// POST /api/task/pause  {"task_id": "uuid"}
// Saves previous status to routing_flag so resume can restore it.
func (s *Server) handleTaskPause(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TaskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task_id required"})
		return
	}

	ctx := r.Context()

	// Get current status
	raw, err := s.db.Query(ctx, "tasks", map[string]any{
		"select":  "status",
		"id":      "eq." + req.TaskID,
		"limit":   1,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("query failed: %v", err)})
		return
	}
	var results []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &results); err != nil || len(results) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
		return
	}
	currentStatus := results[0].Status

	// Don't pause already completed/cancelled tasks
	terminalStatuses := map[string]bool{"merged": true, "complete": true, "cancelled": true}
	if terminalStatuses[currentStatus] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("cannot pause task in '%s' status", currentStatus)})
		return
	}

	// Save current status to routing_flag so resume knows what to restore
	_, err = s.db.Update(ctx, "tasks", req.TaskID, map[string]any{
		"status":       "paused",
		"routing_flag": currentStatus,
		"updated_at":   time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("update failed: %v", err)})
		return
	}
	// pg_notify is safe to call directly - it's a built-in PostgreSQL function
	s.db.RPC(ctx, "pg_notify", map[string]interface{}{
		"channel": "vp_changes",
		"payload": fmt.Sprintf(`{"table":"tasks","action":"UPDATE","id":"%s","status":"paused"}`, req.TaskID),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"task_id":        req.TaskID,
		"previous_status": currentStatus,
		"status":         "paused",
	})
}

// handleTaskResume resumes a paused task.
// POST /api/task/resume  {"task_id": "uuid"}
// Restores the status saved in routing_flag.
func (s *Server) handleTaskResume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TaskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task_id required"})
		return
	}

	ctx := r.Context()

	// Get current status and saved previous status
	raw, err := s.db.Query(ctx, "tasks", map[string]any{
		"select":  "status, routing_flag",
		"id":      "eq." + req.TaskID,
		"limit":   1,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("query failed: %v", err)})
		return
	}
	var results []struct {
		Status      string `json:"status"`
		RoutingFlag string `json:"routing_flag"`
	}
	if err := json.Unmarshal(raw, &results); err != nil || len(results) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
		return
	}

	if results[0].Status != "paused" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("task is not paused (status: %s)", results[0].Status)})
		return
	}

	// Restore previous status from routing_flag, default to "pending"
	restoredStatus := results[0].RoutingFlag
	if restoredStatus == "" {
		restoredStatus = "pending"
	}

	_, err = s.db.Update(ctx, "tasks", req.TaskID, map[string]any{
		"status":       restoredStatus,
		"routing_flag": "",
		"updated_at":   time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("update failed: %v", err)})
		return
	}
	// Notify SSE listeners so dashboard refreshes immediately
	s.db.RPC(ctx, "pg_notify", map[string]interface{}{
		"channel": "vp_changes",
		"payload": fmt.Sprintf(`{"table":"tasks","action":"UPDATE","id":"%s","status":"%s"}`, req.TaskID, restoredStatus),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"task_id": req.TaskID,
		"status":  restoredStatus,
	})
}

// handleTaskKill cancels a single task.
// POST /api/task/kill  {"task_id": "uuid", "reason": "..."}
func (s *Server) handleTaskKill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TaskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task_id required"})
		return
	}

	ctx := r.Context()

	reason := req.Reason
	if reason == "" {
		reason = "Killed by operator"
	}

	_, err := s.db.Update(ctx, "tasks", req.TaskID, map[string]any{
		"status":        "cancelled",
		"failure_notes": reason,
		"updated_at":    time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("update failed: %v", err)})
		return
	}
	// Notify SSE listeners so dashboard refreshes immediately
	s.db.RPC(ctx, "pg_notify", map[string]interface{}{
		"channel": "vp_changes",
		"payload": fmt.Sprintf(`{"table":"tasks","action":"UPDATE","id":"%s","status":"cancelled"}`, req.TaskID),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"task_id": req.TaskID,
		"status":  "cancelled",
		"reason":  reason,
	})
}

// handleTasksPauseAll pauses all non-completed tasks.
// POST /api/tasks/pause-all
func (s *Server) handleTasksPauseAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	ctx := r.Context()
	paused, errors := s.pauseAllActive(ctx)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"paused":  paused,
		"errors":  errors,
		"message": fmt.Sprintf("Paused %d tasks", paused),
	})
}

// handleTasksClearAll deletes all non-completed tasks.
// POST /api/tasks/clear-all  {"confirm": true}
func (s *Server) handleTasksClearAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	var req struct {
		Confirm bool `json:"confirm"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if !req.Confirm {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm required: set confirm=true"})
		return
	}

	ctx := r.Context()

	// Get all non-terminal tasks
	raw, err := s.db.Query(ctx, "tasks", map[string]any{
		"select":        "id, status",
		"status":        "not.in.(merged,complete,cancelled)",
		"limit":         1000,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("query failed: %v", err)})
		return
	}

	var tasks []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(raw, &tasks)

	cleared := 0
	for _, t := range tasks {
		_, err := s.db.Update(ctx, "tasks", t.ID, map[string]any{
			"status":        "cancelled",
			"failure_notes": "Cleared by operator (clear-all)",
			"updated_at":    time.Now().UTC(),
		})
		if err == nil {
			cleared++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"cleared": cleared,
		"message": fmt.Sprintf("Cancelled %d tasks", cleared),
	})
}

// handleTasksActive returns all active (non-completed) tasks.
// GET /api/tasks/active
func (s *Server) handleTasksActive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.checkAdminAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	ctx := r.Context()

	raw, err := s.db.Query(ctx, "tasks", map[string]any{
		"select":        "id, title, type, status, assigned_to, attempts, slice_id, phase, task_number, created_at, updated_at",
		"status":        "not.in.(merged,complete,cancelled)",
		"order":         "updated_at.desc",
		"limit":         200,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("query failed: %v", err)})
		return
	}

	w.Write(raw)
}

// pauseAllActive is the shared logic for pause-all (used by both API and internal calls).
func (s *Server) pauseAllActive(ctx context.Context) (paused int, errors int) {
	raw, err := s.db.Query(ctx, "tasks", map[string]any{
		"select":  "id, status",
		"status":  "not.in.(merged,complete,cancelled,paused)",
		"limit":   1000,
	})
	if err != nil {
		return 0, 0
	}

	var tasks []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(raw, &tasks)

	for _, t := range tasks {
		_, err := s.db.Update(ctx, "tasks", t.ID, map[string]any{
			"status":       "paused",
			"routing_flag": t.Status,
			"updated_at":   time.Now().UTC(),
		})
		if err != nil {
			errors++
		} else {
			paused++
		}
	}
	return
}
