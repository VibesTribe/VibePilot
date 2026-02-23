package courier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

type WebhookHandler struct {
	secret     []byte
	onComplete func(Result)
}

type Result struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Output    string `json:"output"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	ChatURL   string `json:"chat_url"`
	Error     string `json:"error"`
}

func NewWebhookHandler(secret string, onComplete func(Result)) *WebhookHandler {
	return &WebhookHandler{
		secret:     []byte(secret),
		onComplete: onComplete,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	if len(h.secret) > 0 && !h.validateSignature(payload, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var result Result
	if err := json.Unmarshal(payload, &result); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if result.TaskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}

	log.Printf("Courier: received webhook for %s: %s", result.TaskID[:8], result.Status)

	if h.onComplete != nil {
		h.onComplete(result)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func (h *WebhookHandler) validateSignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	expected := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, h.secret)
	mac.Write(payload)
	actual := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(actual))
}
