package webhooks

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// chatUsageRequest is the payload Hermes sends after a dashboard chat completes.
type chatUsageRequest struct {
	SessionID string `json:"session_id"`
	ModelID   string `json:"model_id"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Source    string `json:"token_source"`
}

// handleChatUsage accepts token usage from Hermes dashboard chat sessions.
// POST /api/chat/usage
func (s *Server) handleChatUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if r.Body != nil {
		defer r.Body.Close()
	}
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var req chatUsageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" || req.ModelID == "" {
		http.Error(w, "session_id and model_id required", http.StatusBadRequest)
		return
	}

	if req.Source == "" {
		req.Source = "exact"
	}

	// Call the record_chat_usage RPC
	_, err = s.db.RPC(r.Context(), "record_chat_usage", map[string]interface{}{
		"p_session_id":  req.SessionID,
		"p_model_id":    req.ModelID,
		"p_tokens_in":   req.TokensIn,
		"p_tokens_out":  req.TokensOut,
		"p_token_source": req.Source,
	})
	if err != nil {
		log.Printf("[ChatUsage] RPC error for session %s: %v", req.SessionID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[ChatUsage] Recorded %d+%d tokens for %s (model %s)",
		req.TokensIn, req.TokensOut, req.SessionID, req.ModelID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
