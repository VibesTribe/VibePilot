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
	"os"
	"path/filepath"
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
	configDir     string // governor config directory (e.g. governor/config)
	creditTracker CreditTracker
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

func (s *Server) SetConfigDir(dir string) {
	s.configDir = dir
}

// CreditTracker interface — only needs SetCredit for admin API
type CreditTracker interface {
	SetCredit(ctx context.Context, modelID string, totalUSD float64) error
}

func (s *Server) SetCreditTracker(tracker CreditTracker) {
	s.creditTracker = tracker
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
	mux.HandleFunc("/api/task/review", s.handleTaskReview)
	mux.HandleFunc("/api/project/snapshot", s.handleProjectSnapshot)
	mux.HandleFunc("/api/project/history", s.handleProjectHistory)
	mux.HandleFunc("/api/project/alerts", s.handleProjectAlerts)
	mux.HandleFunc("/api/project-costs", s.handleProjectCosts)
	mux.HandleFunc("/api/admin/model", s.handleAdminModel)
	mux.HandleFunc("/api/admin/models", s.handleAdminModels)
	mux.HandleFunc("/api/chat/usage", s.handleChatUsage)

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

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Webhooks] Failed to parse payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	eventType := s.mapToEventType(&payload)
	if eventType == "" {
		log.Printf("[Webhooks] Unknown event for table %s, type %s", payload.Table, payload.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := r.Context()

	if handler, ok := s.handlers[eventType]; ok {
		if err := handler(ctx, &payload); err != nil {
			log.Printf("[Webhooks] Handler error for %s: %v", eventType, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	recordJSON, _ := json.Marshal(payload.Record)
	event := runtime.Event{
		Type:      runtime.EventType(eventType),
		ID:        extractID(payload.Record),
		Table:     payload.Table,
		Record:    recordJSON,
		Timestamp: time.Now(),
	}

	if s.router != nil {
		s.router.Route(event)
	}

	log.Printf("[Webhooks] Processed %s from %s", eventType, payload.Table)
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
		case status == "testing":
			return string(runtime.EventTaskTesting)
		case status == "complete":
			return string(runtime.EventTaskApproval)
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
		case "approved":
			return string(runtime.EventPlanApproved)
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
		{"system_counters", nil},
		{"project_costs", map[string]any{"order": "incurred_at.desc", "limit": 200}},
		{"subscription_history", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"project_snapshots", map[string]any{"order": "created_at.desc", "limit": 50}},
		{"chat_usage", map[string]any{"order": "created_at.desc", "limit": 500}},
		{"agent_sessions", map[string]any{"order": "last_activity_at.desc", "limit": 100}},
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
// Payload: {"task_id": "...", "status": "success|failed", "output": "...", "error": "...", "tokens_in": 0, "tokens_out": 0}
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

// handleTaskReview accepts POST from dashboard when human reviews a task in human_review status.
// Payload: {"task_id": "...", "action": "approve"|"reject", "notes": "..."}
// On approve: transitions to "complete" → merge pipeline picks it up.
// On reject: transitions to "pending" with rejection notes for re-execution.
func (s *Server) handleTaskReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	var req struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"` // "approve" or "reject"
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.TaskID == "" || req.Action == "" {
		http.Error(w, "Missing task_id or action", http.StatusBadRequest)
		return
	}

	// Verify task is actually in human_review status
	data, err := s.db.Query(ctx, "tasks", map[string]any{
		"id":     "eq." + req.TaskID,
		"select": "id,status,type,title",
	})
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	var tasks []map[string]any
	if err := json.Unmarshal(data, &tasks); err != nil || len(tasks) == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	if tasks[0]["status"] != "human_review" {
		http.Error(w, "Task is not in human_review status", http.StatusConflict)
		return
	}

	switch req.Action {
	case "approve":
		// Transition to complete — the maintenance handler's handleTaskApproved
		// will pick this up via pgnotify and proceed with merge.
		_, err := s.db.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    req.TaskID,
			"p_new_status": "complete",
			"p_result":     fmt.Sprintf(`{"human_approved":true,"notes":%q}`, req.Notes),
		})
		if err != nil {
			http.Error(w, "Failed to approve task", http.StatusInternalServerError)
			return
		}
		log.Printf("[TaskReview] Task %s APPROVED by human → complete", req.TaskID[:8])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "approved"})

	case "reject":
		// Transition back to pending with rejection notes.
		// The task will be re-routed to a different model for re-execution.
		_, err := s.db.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        req.TaskID,
			"p_new_status":     "pending",
			"p_failure_reason": fmt.Sprintf("human_rejected: %s", req.Notes),
		})
		if err != nil {
			http.Error(w, "Failed to reject task", http.StatusInternalServerError)
			return
		}
		log.Printf("[TaskReview] Task %s REJECTED by human → pending", req.TaskID[:8])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})

	default:
		http.Error(w, "Invalid action: must be approve or reject", http.StatusBadRequest)
	}
}

// handleProjectSnapshot creates a named snapshot of current project state.
// POST with {"label": "my snapshot"} returns the snapshot data.
// DELETE with {"id": "uuid"} removes a snapshot.
func (s *Server) handleProjectSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Label string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Label == "" {
			req.Label = fmt.Sprintf("snapshot-%s", time.Now().Format("2006-01-02-1504"))
		}
		snapID, err := s.db.RPC(ctx, "create_project_snapshot", map[string]any{
			"p_label": req.Label,
		})
		if err != nil {
			log.Printf("[Project] Snapshot failed: %v", err)
			http.Error(w, "Failed to create snapshot", http.StatusInternalServerError)
			return
		}
		log.Printf("[Project] Snapshot created: %s (%s)", req.Label, snapID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "id": snapID, "label": req.Label})

	case http.MethodGet:
		// Return all snapshots
		data, err := s.db.Query(ctx, "project_snapshots", map[string]any{
			"order": "created_at.desc",
			"limit": 50,
		})
		if err != nil {
			http.Error(w, "Failed to query snapshots", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}

// handleProjectHistory returns archived subscription history.
func (s *Server) handleProjectHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	data, err := s.db.Query(ctx, "subscription_history", map[string]any{
		"order": "created_at.desc",
		"limit": 100,
	})
	if err != nil {
		http.Error(w, "Failed to query history", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleProjectAlerts returns current subscription/credit threshold alerts.
func (s *Server) handleProjectAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	data, err := s.db.RPC(ctx, "check_subscription_thresholds", map[string]any{})
	if err != nil {
		http.Error(w, "Failed to check thresholds", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"alerts": %s}`, data)))
}

// handleProjectCosts handles add/archive of project cost entries
func (s *Server) handleProjectCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Action      string  `json:"action"` // "add", "archive", "update"
			ID          string  `json:"id"`
			Category    string  `json:"category"`
			Description string  `json:"description"`
			AmountUSD   float64 `json:"amount_usd"`
			Frequency   string  `json:"frequency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "add":
			if req.Description == "" || req.AmountUSD <= 0 {
				http.Error(w, `{"error":"description and amount_usd required"}`, http.StatusBadRequest)
				return
			}
			if req.Category == "" {
				req.Category = "other"
			}
			if req.Frequency == "" {
				req.Frequency = "one_time"
			}
			result, err := s.db.RPC(ctx, "add_project_cost", map[string]any{
				"p_category":    req.Category,
				"p_description": req.Description,
				"p_amount_usd":  req.AmountUSD,
				"p_frequency":   req.Frequency,
			})
			if err != nil {
				log.Printf("[ProjectCosts] Add failed: %v", err)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			w.Write(result)

		case "archive":
			if req.ID == "" {
				http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
				return
			}
			_, err := s.db.RPC(ctx, "archive_project_cost", map[string]any{
				"p_id": req.ID,
			})
			if err != nil {
				log.Printf("[ProjectCosts] Archive failed: %v", err)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"archived": true})

		case "update":
			if req.ID == "" {
				http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
				return
			}
			params := map[string]any{"p_id": req.ID}
			if req.Category != "" {
				params["p_category"] = req.Category
			}
			if req.Description != "" {
				params["p_description"] = req.Description
			}
			if req.AmountUSD > 0 {
				params["p_amount_usd"] = req.AmountUSD
			}
			if req.Frequency != "" {
				params["p_frequency"] = req.Frequency
			}
			result, err := s.db.RPC(ctx, "update_project_cost", params)
			if err != nil {
				log.Printf("[ProjectCosts] Update failed: %v", err)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			w.Write(result)

		default:
			http.Error(w, `{"error":"action must be add, archive, or update"}`, http.StatusBadRequest)
		}

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAdminModels returns all models from the database (GET /api/admin/models).
// Used by the admin panel and agent chat to see current model roster.
func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	result, err := s.db.RPC(ctx, "get_models_summary", map[string]interface{}{
		"p_status": "all",
	})
	if err != nil {
		// Fallback: query models table directly
		raw, err2 := s.db.Query(ctx, "models", map[string]any{"select": "*"})
		if err2 != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}
		w.Write(raw)
		return
	}
	w.Write(result)
}

// handleAdminModel handles add/update/bench/unbench operations on models.
//
// POST /api/admin/model  — Add or update a model
//   Body: {
//     "action": "add",
//     "model_id": "deepseek/deepseek-chat",
//     "name": "DeepSeek V4",
//     "provider": "deepseek",
//     "tier": "paid",           // "free" or "paid"
//     "role": "backup",         // "primary", "backup", "fallback"
//     "context_limit": 128000,
//     "capabilities": ["code", "reasoning"],
//     "api_key_name": "DEEPSEEK_API_KEY",
//     "api_key_value": "sk-...",
//     "credit_info": "$10 credit",
//     "access_via": ["deepseek-api"]
//   }
//
// POST /api/admin/model  — Bench or unbench
//   Body: {"action": "bench", "model_id": "...", "reason": "..."}
//   Body: {"action": "unbench", "model_id": "..."}
//
// POST /api/admin/model  — Health probe
//   Body: {"action": "probe", "model_id": "..."}
func (s *Server) handleAdminModel(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Action       string   `json:"action"`        // add, update, bench, unbench, probe
		ModelID      string   `json:"model_id"`
		Name         string   `json:"name"`
		Provider     string   `json:"provider"`
		Tier         string   `json:"tier"`          // free or paid
		Role         string   `json:"role"`          // primary, backup, fallback
		ContextLimit int      `json:"context_limit"`
		Capabilities []string `json:"capabilities"`
		APIKeyName   string   `json:"api_key_name"`
		APIKeyValue  string   `json:"api_key_value"`
		CreditInfo   string   `json:"credit_info"`
		Reason       string   `json:"reason"`         // for bench
		AccessVia    []string `json:"access_via"`
		CreditAmount float64  `json:"credit_amount"`  // for set_credit
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %v"}`, err), http.StatusBadRequest)
		return
	}

	if req.ModelID == "" && req.Action != "add" {
		http.Error(w, `{"error":"model_id required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch req.Action {
	case "add", "update":
		s.handleModelAddUpdate(ctx, w, &req)
	case "bench":
		s.handleModelBench(ctx, w, &req)
	case "unbench":
		s.handleModelUnbench(ctx, w, &req)
	case "probe":
		s.handleModelProbe(ctx, w, &req)
	case "set_credit":
		s.handleModelSetCredit(ctx, w, &req)
	default:
		http.Error(w, fmt.Sprintf(`{"error":"unknown action: %s"}`, req.Action), http.StatusBadRequest)
	}
}

func (s *Server) handleModelAddUpdate(ctx context.Context, w http.ResponseWriter, req *struct {
	Action       string   `json:"action"`
	ModelID      string   `json:"model_id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tier         string   `json:"tier"`
	Role         string   `json:"role"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	APIKeyName   string   `json:"api_key_name"`
	APIKeyValue  string   `json:"api_key_value"`
	CreditInfo   string   `json:"credit_info"`
	Reason       string   `json:"reason"`
	AccessVia    []string `json:"access_via"`
	CreditAmount float64  `json:"credit_amount"`
}) {
	modelID := req.ModelID
	if modelID == "" {
		modelID = req.Name // fallback
	}

	// 1. Store API key in vault if provided
	if req.APIKeyValue != "" && s.vault != nil {
		keyName := req.APIKeyName
		if keyName == "" {
			// Auto-generate key name from provider
			keyName = strings.ToUpper(req.Provider) + "_API_KEY"
		}
		if err := s.vault.StoreSecret(ctx, keyName, req.APIKeyValue); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"vault store failed: %v"}`, err), http.StatusInternalServerError)
			return
		}
		log.Printf("[AdminModel] Stored API key %s in vault", keyName)
	}

	// 2. Update models.json
	if s.configDir == "" {
		http.Error(w, `{"error":"config dir not set"}`, http.StatusInternalServerError)
		return
	}

	modelsPath := filepath.Join(s.configDir, "models.json")
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"read models.json: %v"}`, err), http.StatusInternalServerError)
		return
	}

	var modelsFile struct {
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.Unmarshal(data, &modelsFile); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"parse models.json: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Build the model entry
	tier := req.Tier
	if tier == "" {
		tier = "free"
	}
	contextLimit := req.ContextLimit
	if contextLimit == 0 {
		contextLimit = 128000
	}
	capabilities := req.Capabilities
	if len(capabilities) == 0 {
		capabilities = []string{"code", "reasoning", "instruction"}
	}

	// Default access_via based on provider
	accessVia := req.AccessVia
	if len(accessVia) == 0 {
		accessVia = []string{req.Provider + "-api"}
	}

	newModel := map[string]interface{}{
		"id":            modelID,
		"name":          req.Name,
		"provider":      req.Provider,
		"status":        "active",
		"context_limit": contextLimit,
		"capabilities":  capabilities,
		"access_via":    accessVia,
		"buffer_pct":    80,
		"rate_limits": map[string]interface{}{
			"requests_per_minute": 60,
			"requests_per_day":    nil,
			"tokens_per_minute":   nil,
			"tokens_per_day":      nil,
		},
		"tier":        tier,
		"credit_info": req.CreditInfo,
	}

	// Find and update or append
	found := false
	for i, m := range modelsFile.Models {
		if m["id"] == modelID {
			// Preserve existing fields that aren't overridden
			for k, v := range newModel {
				modelsFile.Models[i][k] = v
			}
			found = true
			break
		}
	}
	if !found {
		modelsFile.Models = append(modelsFile.Models, newModel)
	}

	updatedData, err := json.MarshalIndent(modelsFile, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"marshal: %v"}`, err), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(modelsPath, updatedData, 0644); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"write models.json: %v"}`, err), http.StatusInternalServerError)
		return
	}
	log.Printf("[AdminModel] %s model %s in models.json", map[bool]string{true: "Updated", false: "Added"}[found], modelID)

	// 3. Add to routing cascade if requested
	if req.Role != "" {
		s.addToRoutingCascade(modelID, req.Role)
	}

	// 4. Sync to database via RPC
	if s.db != nil {
		s.db.RPC(ctx, "upsert_model", map[string]interface{}{
			"p_id":         modelID,
			"p_name":       req.Name,
			"p_provider":   req.Provider,
			"p_status":     "active",
			"p_tier":       tier,
			"p_credit_info": req.CreditInfo,
		})
	}

	verb := "Added"
	if found {
		verb = "Updated"
	}

	resp := map[string]interface{}{
		"status":   "ok",
		"action":   verb,
		"model_id": modelID,
		"provider": req.Provider,
		"tier":     tier,
		"active":   true,
		"message":  fmt.Sprintf("%s %s (%s, %s)", verb, modelID, req.Provider, tier),
	}
	if req.APIKeyValue != "" {
		resp["vault_key_stored"] = true
	}
	if req.CreditInfo != "" {
		resp["credit_info"] = req.CreditInfo
	}

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleModelBench(ctx context.Context, w http.ResponseWriter, req *struct {
	Action       string   `json:"action"`
	ModelID      string   `json:"model_id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tier         string   `json:"tier"`
	Role         string   `json:"role"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	APIKeyName   string   `json:"api_key_name"`
	APIKeyValue  string   `json:"api_key_value"`
	CreditInfo   string   `json:"credit_info"`
	Reason       string   `json:"reason"`
	AccessVia    []string `json:"access_via"`
	CreditAmount float64  `json:"credit_amount"`
}) {
	reason := req.Reason
	if reason == "" {
		reason = "benched via admin"
	}

	// Update models.json
	if s.configDir != "" {
		modelsPath := filepath.Join(s.configDir, "models.json")
		data, err := os.ReadFile(modelsPath)
		if err == nil {
			var modelsFile struct {
				Models []map[string]interface{} `json:"models"`
			}
			if json.Unmarshal(data, &modelsFile) == nil {
				for _, m := range modelsFile.Models {
					if m["id"] == req.ModelID {
						m["status"] = "benched"
						m["status_reason"] = reason
						break
					}
				}
				if updated, err := json.MarshalIndent(modelsFile, "", "  "); err == nil {
					os.WriteFile(modelsPath, updated, 0644)
				}
			}
		}
	}

	// Update DB
	if s.db != nil {
		s.db.RPC(ctx, "bench_model", map[string]interface{}{
			"p_id":     req.ModelID,
			"p_reason": reason,
		})
	}

	log.Printf("[AdminModel] Benched model %s: %s", req.ModelID, reason)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"model_id": req.ModelID,
		"action":   "benched",
		"reason":   reason,
	})
}

func (s *Server) handleModelUnbench(ctx context.Context, w http.ResponseWriter, req *struct {
	Action       string   `json:"action"`
	ModelID      string   `json:"model_id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tier         string   `json:"tier"`
	Role         string   `json:"role"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	APIKeyName   string   `json:"api_key_name"`
	APIKeyValue  string   `json:"api_key_value"`
	CreditInfo   string   `json:"credit_info"`
	Reason       string   `json:"reason"`
	AccessVia    []string `json:"access_via"`
	CreditAmount float64  `json:"credit_amount"`
}) {
	// Update models.json
	if s.configDir != "" {
		modelsPath := filepath.Join(s.configDir, "models.json")
		data, err := os.ReadFile(modelsPath)
		if err == nil {
			var modelsFile struct {
				Models []map[string]interface{} `json:"models"`
			}
			if json.Unmarshal(data, &modelsFile) == nil {
				for _, m := range modelsFile.Models {
					if m["id"] == req.ModelID {
						m["status"] = "active"
						delete(m, "status_reason")
						break
					}
				}
				if updated, err := json.MarshalIndent(modelsFile, "", "  "); err == nil {
					os.WriteFile(modelsPath, updated, 0644)
				}
			}
		}
	}

	// Update DB
	if s.db != nil {
		s.db.RPC(ctx, "unbench_model", map[string]interface{}{
			"p_id": req.ModelID,
		})
	}

	log.Printf("[AdminModel] Unbenched model %s", req.ModelID)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"model_id": req.ModelID,
		"action":   "unbenched",
	})
}

func (s *Server) handleModelProbe(ctx context.Context, w http.ResponseWriter, req *struct {
	Action       string   `json:"action"`
	ModelID      string   `json:"model_id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tier         string   `json:"tier"`
	Role         string   `json:"role"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	APIKeyName   string   `json:"api_key_name"`
	APIKeyValue  string   `json:"api_key_value"`
	CreditInfo   string   `json:"credit_info"`
	Reason       string   `json:"reason"`
	AccessVia    []string `json:"access_via"`
	CreditAmount float64  `json:"credit_amount"`
}) {
	// Quick probe: send a minimal request via the connector
	// For now, return model status from DB
	if s.db != nil {
		raw, err := s.db.Query(ctx, "models", map[string]any{
			"filter": map[string]any{"id": req.ModelID},
		})
		if err == nil {
			w.Write(raw)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"model_id": req.ModelID,
		"action":   "probe",
		"message":  "probe requested (runs async via CooldownWatcher)",
	})
}

func (s *Server) handleModelSetCredit(ctx context.Context, w http.ResponseWriter, req *struct {
	Action       string   `json:"action"`
	ModelID      string   `json:"model_id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Tier         string   `json:"tier"`
	Role         string   `json:"role"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	APIKeyName   string   `json:"api_key_name"`
	APIKeyValue  string   `json:"api_key_value"`
	CreditInfo   string   `json:"credit_info"`
	Reason       string   `json:"reason"`
	AccessVia    []string `json:"access_via"`
	CreditAmount float64  `json:"credit_amount"`
}) {
	if req.CreditAmount <= 0 {
		http.Error(w, `{"error":"credit_amount must be > 0"}`, http.StatusBadRequest)
		return
	}

	// Set credit via UsageTracker (persists to DB)
	if s.creditTracker != nil {
		if err := s.creditTracker.SetCredit(ctx, req.ModelID, req.CreditAmount); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}
	}

	// Also update models.json credit_info
	if s.configDir != "" {
		modelsPath := filepath.Join(s.configDir, "models.json")
		data, err := os.ReadFile(modelsPath)
		if err == nil {
			var modelsFile struct {
				Models []map[string]interface{} `json:"models"`
			}
			if json.Unmarshal(data, &modelsFile) == nil {
				for _, m := range modelsFile.Models {
					if m["id"] == req.ModelID {
						m["credit_info"] = fmt.Sprintf("$%.2f credit", req.CreditAmount)
						if m["status"] == "paused" {
							m["status"] = "active"
							delete(m, "status_reason")
						}
						break
					}
				}
				if updated, err := json.MarshalIndent(modelsFile, "", "  "); err == nil {
					os.WriteFile(modelsPath, updated, 0644)
				}
			}
		}
	}

	log.Printf("[AdminModel] Credit set: %s = $%.2f", req.ModelID, req.CreditAmount)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "ok",
		"model_id":         req.ModelID,
		"action":           "credit_set",
		"credit_total":     req.CreditAmount,
		"credit_remaining": req.CreditAmount,
		"message":          fmt.Sprintf("Set $%.2f credit on %s", req.CreditAmount, req.ModelID),
	})
}

// addToRoutingCascade inserts a model into the free_cascade priority list in routing.json.
// Role determines position: "primary" = near front, "backup" = near end, "fallback" = very end.
func (s *Server) addToRoutingCascade(modelID, role string) {
	if s.configDir == "" {
		return
	}

	routingPath := filepath.Join(s.configDir, "routing.json")
	data, err := os.ReadFile(routingPath)
	if err != nil {
		log.Printf("[AdminModel] Warning: could not read routing.json: %v", err)
		return
	}

	var routing map[string]interface{}
	if err := json.Unmarshal(data, &routing); err != nil {
		log.Printf("[AdminModel] Warning: could not parse routing.json: %v", err)
		return
	}

	strategies, ok := routing["strategies"].(map[string]interface{})
	if !ok {
		return
	}
	cascade, ok := strategies["free_cascade"].(map[string]interface{})
	if !ok {
		return
	}
	priority, ok := cascade["priority"].([]interface{})
	if !ok {
		return
	}

	// Check if already in cascade
	for _, p := range priority {
		if p == modelID {
			return // already there
		}
	}

	// Insert based on role
	switch role {
	case "primary":
		// Insert after the first Gemini entries (position 4 or so)
		pos := 4
		if pos > len(priority) {
			pos = len(priority)
		}
		priority = append(priority[:pos], append([]interface{}{modelID}, priority[pos:]...)...)
	case "backup":
		// Insert before the last few fallback entries
		pos := len(priority) - 2
		if pos < 0 {
			pos = len(priority)
		}
		priority = append(priority[:pos], append([]interface{}{modelID}, priority[pos:]...)...)
	case "fallback":
		priority = append(priority, modelID)
	default:
		priority = append(priority, modelID)
	}

	cascade["priority"] = priority

	updated, err := json.MarshalIndent(routing, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(routingPath, updated, 0644); err != nil {
		log.Printf("[AdminModel] Warning: could not write routing.json: %v", err)
		return
	}
	log.Printf("[AdminModel] Added %s to routing cascade as %s", modelID, role)
}
