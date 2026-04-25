package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/runtime"
)

type CourierResultFunc func(taskID string, result json.RawMessage) error

// VaultManager is the interface the server needs for vault API endpoints.
// Keeps server.go decoupled from the vault package.
type VaultManager interface {
	GetSecretNoCache(ctx context.Context, keyName string) (string, error)
	StoreSecret(ctx context.Context, keyName, plaintext string) error
	ListSecrets(ctx context.Context) ([]string, error)
	DeleteSecret(ctx context.Context, keyName string) error
	RotateKey(ctx context.Context, newMasterKey string) (int, error)
}

type Server struct {
	port      int
	path      string
	secret    string
	version   string
	startTime time.Time
	router    *runtime.EventRouter
	github    *GitHubWebhookHandler
	server    *http.Server
	handlers  map[string]EventHandler
	db        DBQuerier
	wsPath    string
	wsUpgrader any
	sseBroker  *SSEBroker
	courierResultFn CourierResultFunc
	vault      VaultManager
	adminToken string
}

type DBQuerier interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
}

type EventHandler func(ctx context.Context, payload *Payload) error

type Config struct {
	Port    int
	Path    string
	Secret  string
	Version string
}

type Payload struct {
	Type      string         `json:"type"`
	Table     string         `json:"table"`
	Schema    string         `json:"schema"`
	Record    map[string]any `json:"record"`
	OldRecord map[string]any `json:"old_record"`
	Auth      map[string]any `json:"auth"`
	Timestamp time.Time      `json:"timestamp"`
}

func NewServer(cfg *Config, router *runtime.EventRouter) *Server {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Path == "" {
		cfg.Path = "/webhooks"
	}

	return &Server{
		port:      cfg.Port,
		path:      cfg.Path,
		secret:    cfg.Secret,
		version:   cfg.Version,
		startTime: time.Now(),
		router:    router,
		handlers:  make(map[string]EventHandler),
		sseBroker: NewSSEBroker(),
	}
}

func (s *Server) SetGitHubHandler(handler *GitHubWebhookHandler) {
	s.github = handler
}

func (s *Server) SetDB(db DBQuerier) {
	s.db = db
}

// SetSSEBroker replaces the default SSE broker with a shared instance.
// Used to share one broker between pgnotify and the webhook server.
func (s *Server) SetSSEBroker(broker *SSEBroker) {
	s.sseBroker = broker
}

// SetCourierResultFn registers the callback for courier result POSTs.
// The callback receives (taskID, rawJSON) and returns error.
func (s *Server) SetCourierResultFn(fn CourierResultFunc) {
	s.courierResultFn = fn
}

// SetVault registers the vault manager for /api/vault/* endpoints.
func (s *Server) SetVault(v VaultManager) {
	s.vault = v
}

// SetAdminToken sets the token required for admin endpoints (vault management).
// If empty, vault endpoints return 403.
func (s *Server) SetAdminToken(token string) {
	s.adminToken = token
}

func (s *Server) RegisterHandler(eventType string, handler EventHandler) {
	s.handlers[eventType] = handler
	log.Printf("[Webhooks] Registered handler for: %s", eventType)
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWebhook)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/api/bookmarks", s.handleBookmark)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/dashboard/stream", s.handleSSE)
	mux.HandleFunc("/api/courier/result", s.handleCourierResult)
	mux.HandleFunc("/api/vault/", s.handleVaultAPI)


	s.server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.port),
		Handler: mux,
	}

	log.Printf("[Webhooks] Server starting on port %d at %s", s.port, s.path)
	log.Printf("[WebSocket] Listening at %d%s", s.port, s.wsPath)

	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return s.Shutdown(ctx)
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		log.Printf("[Webhooks] Server shutting down")
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Webhooks] Failed to read body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	githubEventType := r.Header.Get("X-GitHub-Event")
	if githubEventType != "" {
		s.handleGitHubWebhook(w, r, body, githubEventType)
		return
	}

	if s.secret != "" {
		authHeader := r.Header.Get("Authorization")
		signature := r.Header.Get("X-Supabase-Signature")

		if authHeader != "" {
			if authHeader != s.secret && authHeader != "Bearer "+s.secret {
				log.Printf("[Webhooks] Invalid Authorization header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		} else if signature != "" {
			if !s.verifySignature(body, signature) {
				log.Printf("[Webhooks] Invalid signature")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		} else {
			log.Printf("[Webhooks] WARNING: No auth header - accepting for debugging")
		}
	}

	var supabasePayload Payload
	if err := json.Unmarshal(body, &supabasePayload); err != nil {
		log.Printf("[Webhooks] Failed to parse payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	eventType := s.mapToEventType(&supabasePayload)
	if eventType == "" {
		log.Printf("[Webhooks] Unknown event for table %s, type %s", supabasePayload.Table, supabasePayload.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := r.Context()

	if handler, ok := s.handlers[eventType]; ok {
		if err := handler(ctx, &supabasePayload); err != nil {
			log.Printf("[Webhooks] Handler error for %s: %v", eventType, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	recordJSON, _ := json.Marshal(supabasePayload.Record)
	event := runtime.Event{
		Type:      runtime.EventType(eventType),
		ID:        extractID(supabasePayload.Record),
		Table:     supabasePayload.Table,
		Record:    recordJSON,
		Timestamp: time.Now(),
	}

	if s.router != nil {
		s.router.Route(event)
	}

	log.Printf("[Webhooks] Processed %s from %s", eventType, supabasePayload.Table)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request, body []byte, eventType string) {
	if s.github == nil {
		log.Printf("[GitHub Webhooks] No handler configured for GitHub events")
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := r.Context()

	switch eventType {
	case "push":
		s.github.HandlePush(ctx, body)
	default:
		log.Printf("[GitHub Webhooks] Unhandled event type: %s", eventType)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func (s *Server) mapToEventType(payload *Payload) string {
	table := payload.Table
	action := payload.Type

	switch {
	case table == "tasks":
		status, _ := payload.Record["status"].(string)
		switch {
		case status == "pending" && action == "INSERT":
			return string(runtime.EventTaskAvailable)
		case status == "pending" && action == "UPDATE":
			if oldStatus, _ := payload.OldRecord["status"].(string); oldStatus != "pending" {
				return string(runtime.EventTaskAvailable)
			}
		case status == "review":
			return string(runtime.EventTaskReview)
		case status == "testing" || status == "complete":
			return string(runtime.EventTaskCompleted)
		}

	case table == "plans":
		status, _ := payload.Record["status"].(string)
		switch status {
		case "draft":
			return string(runtime.EventPlanCreated)
		case "review":
			return string(runtime.EventPlanReview)
		case "council_review":
			return string(runtime.EventCouncilReview)
		case "council_done":
			return string(runtime.EventCouncilDone)
		case "approved":
			return string(runtime.EventPlanApproved)
		case "blocked":
			return string(runtime.EventPlanBlocked)
		case "revision_needed":
			return string(runtime.EventRevisionNeeded)
		}

	case table == "prd_files" || (table == "plans" && payload.Record["prd_path"] != nil):
		return string(runtime.EventPRDReady)

	case table == "research_suggestions":
		status, _ := payload.Record["status"].(string)
		switch status {
		case "ready":
			return string(runtime.EventResearchReady)
		case "council_review":
			return string(runtime.EventResearchCouncil)
		}

	case table == "maintenance_commands":
		return string(runtime.EventMaintenanceCmd)

	case table == "test_results":
		return string(runtime.EventTestResults)
	}

	return ""
}

func extractID(record map[string]any) string {
	if id, ok := record["id"].(string); ok {
		return id
	}
	return ""
}

func (s *Server) handleBookmark(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Title  string `json:"title"`
		Note   string `json:"note"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	if req.Source == "" {
		req.Source = "bookmarklet"
	}

	ctx := r.Context()
	if s.db != nil {
		result, err := s.db.RPC(ctx, "add_bookmark", map[string]interface{}{
			"p_url":    req.URL,
			"p_title":  req.Title,
			"p_note":   req.Note,
			"p_source": req.Source,
		})
		if err != nil {
			log.Printf("[Bookmarks] Failed to save: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(result)
		log.Printf("[Bookmarks] Saved: %s", req.URL)
	} else {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	// Query all tables the dashboard needs in parallel
	type tableResult struct {
		name string
		data json.RawMessage
		err  error
	}

	tables := []struct {
		name    string
		filters map[string]any
	}{
		{"tasks", map[string]any{"order": "updated_at.desc", "limit": 100}},
		{"task_runs", map[string]any{"order": "started_at.desc", "limit": 500}},
		{"models", nil},
		{"platforms", nil},
		{"orchestrator_events", map[string]any{"order": "created_at.desc", "limit": 500}},
		{"plans", map[string]any{"order": "created_at.desc", "limit": 100}},
		{"council_reviews", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"test_results", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"exchange_rates", nil},
		{"failure_records", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"maintenance_commands", map[string]any{"order": "created_at.desc", "limit": 200}},
	}

	results := make(chan tableResult, len(tables))
	for _, t := range tables {
		go func(name string, filters map[string]any) {
			data, err := s.db.Query(ctx, name, filters)
			results <- tableResult{name: name, data: data, err: err}
		}(t.name, t.filters)
	}

	response := make(map[string]json.RawMessage, len(tables))
	for i := 0; i < len(tables); i++ {
		res := <-results
		if res.err != nil {
			log.Printf("[Dashboard] Error querying %s: %v", res.name, res.err)
			response[res.name] = json.RawMessage("[]")
		} else if res.data == nil {
			response[res.name] = json.RawMessage("[]")
		} else {
			response[res.name] = res.data
		}
	}

	// ETag: hash only the actual data (no volatile timestamp)
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("[Dashboard] Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h := sha256.New()
	h.Write(responseBytes)
	etag := hex.EncodeToString(h.Sum(nil))[:16]

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("ETag", etag)

	// 304 Not Modified — skip sending 181KB
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Write(responseBytes)
}



func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Seconds()
	resp := map[string]any{
		"governor":       "vibepilot",
		"version":        s.version,
		"status":         "running",
		"uptime_seconds": uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetPort() int {
	return s.port
}


// GetSSEBroker returns the SSE broker so other packages (pgnotify) can broadcast.
func (s *Server) GetSSEBroker() *SSEBroker {
	return s.sseBroker
}

// handleSSE serves Server-Sent Events to dashboard clients.
// The browser's EventSource API connects here and receives real-time
// notifications when any monitored table changes.
// Format: data: {"table":"tasks","action":"UPDATE","id":"abc-123"}\n\n
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Subscribe to notifications
	ch := s.sseBroker.Subscribe()
	defer s.sseBroker.Unsubscribe(ch)

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	// Keepalive ticker — sends a comment every 30s so connections don't time out
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case notif, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(notif)
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// handleCourierResult accepts POST from courier agents (GitHub Actions) with task results.
// Replaces the old Supabase REST write + realtime notify pattern.
// Payload: {"task_id": "...", "status": "completed|failed", "output": "...", "error": "...", "tokens_in": 0, "tokens_out": 0}
func (s *Server) handleCourierResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload struct {
		TaskID    string `json:"task_id"`
		Status    string `json:"status"`
		Output    string `json:"output"`
		Error     string `json:"error"`
		TokensIn  int    `json:"tokens_in"`
		TokensOut int    `json:"tokens_out"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.TaskID == "" {
		http.Error(w, "Missing task_id", http.StatusBadRequest)
		return
	}

	log.Printf("[CourierResult] Received: task=%s status=%s", payload.TaskID, payload.Status)

	// Notify the courier runner (delivers to waiting goroutine via channel)
	if s.courierResultFn != nil {
		if err := s.courierResultFn(payload.TaskID, body); err != nil {
			log.Printf("[CourierResult] Handler error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleWebSocket is deprecated — replaced by SSE (/api/dashboard/stream).
// Kept as stub so any references don't break at compile time.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Deprecated: use /api/dashboard/stream (SSE) instead", http.StatusGone)
}

func (s *Server) SetWSUpgrader(upgrader any) {
	s.wsUpgrader = upgrader
}

func (s *Server) SetWSPath(path string) {
	s.wsPath = path
}

func (s *Server) IsRunning() bool {
	return s.server != nil
}

func GetWebhookURL(baseURL string, port int, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s:%d%s", baseURL, port, path)
}

// --- Vault API endpoints ---
// All require admin token in Authorization: Bearer <token> header.
// Routes:
//   GET  /api/vault/list        → list key names
//   GET  /api/vault/get?key=X   → decrypt and return value
//   POST /api/vault/set          → {"key":"X","value":"Y"} → encrypt and store
//   POST /api/vault/delete       → {"key":"X"} → delete
//   POST /api/vault/rotate-key   → {"new_key":"X"} → re-encrypt all

func (s *Server) checkAdminAuth(r *http.Request) bool {
	if s.adminToken == "" {
		return false
	}
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:] == s.adminToken
	}
	return false
}

func (s *Server) handleVaultAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.checkAdminAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.vault == nil {
		http.Error(w, "Vault not available", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	sub := strings.TrimPrefix(r.URL.Path, "/api/vault/")

	switch {
	case sub == "list" && r.Method == http.MethodGet:
		names, err := s.vault.ListSecrets(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": names})

	case sub == "get" && r.Method == http.MethodGet:
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Missing key parameter", http.StatusBadRequest)
			return
		}
		val, err := s.vault.GetSecretNoCache(ctx, key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": val})

	case sub == "set" && r.Method == http.MethodPost:
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Key == "" || req.Value == "" {
			http.Error(w, "Missing key or value", http.StatusBadRequest)
			return
		}
		if err := s.vault.StoreSecret(ctx, req.Key, req.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stored", "key": req.Key})

	case sub == "delete" && r.Method == http.MethodPost:
		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Key == "" {
			http.Error(w, "Missing key", http.StatusBadRequest)
			return
		}
		if err := s.vault.DeleteSecret(ctx, req.Key); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "key": req.Key})

	case sub == "rotate-key" && r.Method == http.MethodPost:
		var req struct {
			NewKey string `json:"new_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.NewKey == "" {
			http.Error(w, "Missing new_key", http.StatusBadRequest)
			return
		}
		count, err := s.vault.RotateKey(ctx, req.NewKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "rotated", "count": count})

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
