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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/reviewitems"
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
	visualQA      VisualQARunner
	designPreview DesignPreviewer
}

type DBQuerier interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error)
	Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error)
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

// corsMiddleware wraps the entire mux with CORS headers for all /api/ routes.
// This ensures every endpoint callable from the dashboard (different origin)
// gets proper Access-Control-Allow-Origin headers. Pre-flight OPTIONS requests
// are handled automatically without reaching individual handlers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply CORS to all API routes and status endpoint
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/status" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24h cache preflight

			// Handle preflight OPTIONS immediately
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
	mux.HandleFunc("/api/projects", s.handleProjects)
	mux.HandleFunc("/api/projects/scaffold", s.handleProjectScaffold)
	mux.HandleFunc("/api/project-costs", s.handleProjectCosts)
	mux.HandleFunc("/api/admin/model", s.handleAdminModel)
	mux.HandleFunc("/api/admin/models", s.handleAdminModels)
	mux.HandleFunc("/api/admin/system", s.handleAdminSystem)
	mux.HandleFunc("/api/chat/usage", s.handleChatUsage)
	mux.HandleFunc("/api/review-queue", s.handleReviewQueue)
	mux.HandleFunc("/api/visualqa/run", s.handleVisualQARun)
	mux.HandleFunc("/api/visualqa/status", s.handleVisualQAStatus)
	mux.HandleFunc("/api/visualqa/approve", s.handleVisualQAApprove)
	mux.HandleFunc("/api/visualqa/feedback", s.handleVisualQAFeedback)
	mux.HandleFunc("/api/visualqa/feedback/list", s.handleVisualQAFeedbackList)

	// Design Preview API
	mux.HandleFunc("/api/design-preview/generate", s.handleDesignPreviewGenerate)
	mux.HandleFunc("/api/design-preview/approve", s.handleDesignPreviewApprove)
	mux.HandleFunc("/api/design-preview/reject", s.handleDesignPreviewReject)

	// Review Items API (unified review hub)
	mux.HandleFunc("/api/review-items", s.handleReviewItems)
	mux.HandleFunc("/api/review-items/", s.handleReviewItemByID)
	mux.HandleFunc("/api/research-reports", s.handleResearchReports)
	mux.HandleFunc("/api/research-reports/", s.handleResearchReportByID)
	mux.HandleFunc("/api/report-items/", s.handleReportItemDecision)
	mux.HandleFunc("/api/design-preview/list", s.handleDesignPreviewList)
	mux.HandleFunc("/api/design-reviews", s.handleDesignReviews)
	mux.HandleFunc("/api/design-reviews/approve", s.handleDesignReviewAction)

	// Research suggestion insert (replaces fragile psql -c commands from cron)
	mux.HandleFunc("/api/research/suggestion", s.handleResearchSuggestion)

	// Task lifecycle control: pause, resume, kill, pause-all, clear-all
	mux.HandleFunc("/api/task/pause", s.handleTaskPause)
	mux.HandleFunc("/api/task/resume", s.handleTaskResume)
	mux.HandleFunc("/api/task/kill", s.handleTaskKill)
	mux.HandleFunc("/api/tasks/pause-all", s.handleTasksPauseAll)
	mux.HandleFunc("/api/tasks/resume-all", s.handleTasksResumeAll)
	mux.HandleFunc("/api/tasks/clear-all", s.handleTasksClearAll)
	mux.HandleFunc("/api/tasks/active", s.handleTasksActive)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler: s.corsMiddleware(mux),
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

// handleResearchSuggestion accepts a JSON body and inserts it directly into
// research_suggestions via the parameterized db.Insert method — no psql, no
// shell quoting, no SQL injection risk. This replaces the fragile psql -c
// pattern that broke when models couldn't handle triple-nested quoting.
//
// POST /api/research/suggestion
// Body: {"title":"...", "type":"new_model", "complexity":"simple",
//        "summary":"...", "details":{...}, "findings_path":"...",
//        "status":"ready", "source":"daily_research"}
//
// Required fields: title, type
// Optional fields: complexity (default "simple"), summary, details (default {}),
//                  findings_path, status (default "ready"), source (default "daily_research")
//
// Valid types: new_model, new_platform, pricing_change, config_tweak,
//              architecture, new_data_store, security, workflow_change,
//              api_credit_exhausted, ui_ux_change, tool_update, analyst_proposal
// Valid complexity: simple, complex, human
// Valid status: pending, ready, in_review, council_review, approved, rejected,
//               implemented, pending_human, watching, prd_created
func (s *Server) handleResearchSuggestion(w http.ResponseWriter, r *http.Request) {
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
		Title        string                 `json:"title"`
		Type         string                 `json:"type"`
		Complexity   string                 `json:"complexity"`
		Summary      string                 `json:"summary"`
		Details      map[string]interface{} `json:"details"`
		FindingsPath string                 `json:"findings_path"`
		Status       string                 `json:"status"`
		Source       string                 `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	// Defaults
	if req.Complexity == "" {
		req.Complexity = "simple"
	}
	if req.Status == "" {
		req.Status = "ready"
	}
	if req.Source == "" {
		req.Source = "daily_research"
	}
	if req.Details == nil {
		req.Details = map[string]interface{}{}
	}

	validTypes := map[string]bool{
		"new_model": true, "new_platform": true, "pricing_change": true,
		"config_tweak": true, "architecture": true, "new_data_store": true,
		"security": true, "workflow_change": true, "api_credit_exhausted": true,
		"ui_ux_change": true, "tool_update": true, "analyst_proposal": true,
	}
	if !validTypes[req.Type] {
		http.Error(w, fmt.Sprintf("Invalid type: %s", req.Type), http.StatusBadRequest)
		return
	}

	validComplexity := map[string]bool{"simple": true, "complex": true, "human": true}
	if !validComplexity[req.Complexity] {
		http.Error(w, fmt.Sprintf("Invalid complexity: %s (must be simple, complex, or human)", req.Complexity), http.StatusBadRequest)
		return
	}

	validStatus := map[string]bool{
		"pending": true, "ready": true, "in_review": true, "council_review": true,
		"approved": true, "rejected": true, "implemented": true,
		"pending_human": true, "watching": true, "prd_created": true,
	}
	if !validStatus[req.Status] {
		http.Error(w, fmt.Sprintf("Invalid status: %s", req.Status), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	insertData := map[string]any{
		"title":         req.Title,
		"type":          req.Type,
		"complexity":    req.Complexity,
		"summary":       req.Summary,
		"details":       req.Details,
		"findings_path": req.FindingsPath,
		"status":        req.Status,
		"source":        req.Source,
	}

	result, err := s.db.Insert(ctx, "research_suggestions", insertData)
	if err != nil {
		log.Printf("[Research API] Failed to insert suggestion: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(result)
	log.Printf("[Research API] Inserted suggestion: %s (type=%s, status=%s)", req.Title, req.Type, req.Status)
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

	// Optional project filter: ?project=<slug> limits tasks to that project
	projectFilter := r.URL.Query().Get("project")

	// Query all tables the dashboard needs in parallel
	type tableResult struct {
		name string
		data json.RawMessage
		err  error
	}

	// Build task filters (add project filter if specified)
	taskFilters := map[string]any{"order": "updated_at.desc", "limit": 100}
	taskRunFilters := map[string]any{"order": "started_at.desc", "limit": 500}
	if projectFilter != "" {
		// Resolve project slug to UUID
		projData, projErr := s.db.Query(ctx, "projects", map[string]any{
			"select":   "id,slug",
			"slug":     fmt.Sprintf("eq.%s", projectFilter),
			"limit":    1,
		})
		if projErr == nil {
			var projRows []map[string]any
			if json.Unmarshal(projData, &projRows) == nil && len(projRows) > 0 {
				if projID, ok := projRows[0]["id"].(string); ok && projID != "" {
					taskFilters["project_id"] = fmt.Sprintf("eq.%s", projID)
					taskRunFilters["project_id"] = fmt.Sprintf("eq.%s", projID)
				}
			}
		}
	}

	// Build project-scoped filters for costs and counters
	costFilters := map[string]any{"order": "incurred_at.desc", "limit": 200}
	counterFilters := map[string]any{}
	if projectFilter != "" {
		// Resolve project slug to UUID (reuse the same lookup as taskFilters above)
		projData2, _ := s.db.Query(ctx, "projects", map[string]any{
			"select": "id",
			"slug":   fmt.Sprintf("eq.%s", projectFilter),
			"limit":  1,
		})
		var projRows2 []map[string]any
		if json.Unmarshal(projData2, &projRows2) == nil && len(projRows2) > 0 {
			if projID2, ok := projRows2[0]["id"].(string); ok && projID2 != "" {
				costFilters["project_id"] = fmt.Sprintf("eq.%s", projID2)
				counterFilters["project_id"] = fmt.Sprintf("eq.%s", projID2)
			}
		}
	}

	tables := []struct {
		name    string
		filters map[string]any
	}{
		{"tasks", taskFilters},
		{"task_runs", taskRunFilters},
		{"models", nil},
		{"platforms", nil},
		{"orchestrator_events", map[string]any{"order": "created_at.desc", "limit": 500}},
		{"plans", map[string]any{"order": "created_at.desc", "limit": 100}},
		{"council_reviews", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"test_results", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"exchange_rates", nil},
		{"failure_records", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"maintenance_commands", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"system_counters", counterFilters},
		{"project_costs", costFilters},
		{"subscription_history", map[string]any{"order": "created_at.desc", "limit": 200}},
		{"project_snapshots", map[string]any{"order": "created_at.desc", "limit": 50}},
		{"chat_usage", map[string]any{"order": "created_at.desc", "limit": 500}},
		{"agent_sessions", map[string]any{"order": "last_activity_at.desc", "limit": 100}},
		{"visual_qa_runs", map[string]any{"order": "started_at.desc", "limit": 50}},
		{"design_reviews", map[string]any{"order": "created_at.desc", "limit": 50}},
		{"model_health_snapshots", map[string]any{"order": "scanned_at.desc", "limit": 500}},
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
		ModelID   string `json:"model_id"`
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
	// Allow dashboard origin without token (same pattern as Hermes gateway auth bypass)
	origin := r.Header.Get("Origin")
	if origin == "https://vibeflow-dashboard.vercel.app" {
		return true
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

// handleProjects returns the list of all projects with branding info.
// GET /api/projects — returns projects array with slug, display_name, theme, status.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	// POST: create a new project
	if r.Method == http.MethodPost {
		s.handleProjectCreate(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	data, err := s.db.Query(ctx, "projects", map[string]any{
		"select": "id,slug,display_name,description,status,theme,deploy_url,github_owner,github_repo,total_tasks,completed_tasks,connected_services,model_keys",
		"order":  "created_at.asc",
	})
	if err != nil {
		http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(data)
}

// handleProjectCreate creates a new project.
// POST /api/projects
// Body: {"slug":"sealed", "display_name":"Sealed", "description":"Music contracts",
//        "theme":{"primary_color":"#34d399"}, "github_owner":"VibesTribe", ...}
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug             string                 `json:"slug"`
		DisplayName      string                 `json:"display_name"`
		Description      string                 `json:"description"`
		GithubOwner      string                 `json:"github_owner"`
		GithubRepo       string                 `json:"github_repo"`
		RepoPath         string                 `json:"repo_path"`
		DeployTarget     string                 `json:"deploy_target"`
		DeployURL        string                 `json:"deploy_url"`
		TechStack        string                 `json:"tech_stack"`
		Theme            map[string]interface{} `json:"theme"`
		ConnectedServices []interface{}         `json:"connected_services"`
		ModelKeys        []interface{}          `json:"model_keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Slug
	}
	if req.DeployTarget == "" {
		req.DeployTarget = "none"
	}
	if req.TechStack == "" {
		req.TechStack = "auto"
	}

	// Check for duplicate slug
	existing, err := s.db.Query(r.Context(), "projects", map[string]any{"slug": req.Slug})
	if err == nil {
		var rows []map[string]any
		if json.Unmarshal(existing, &rows) == nil && len(rows) > 0 {
			http.Error(w, `{"error":"slug_exists","message":"A project with this slug already exists"}`, http.StatusConflict)
			return
		}
	}

	insertData := map[string]any{
		"slug":               req.Slug,
		"display_name":       req.DisplayName,
		"description":        req.Description,
		"status":             "active",
		"github_owner":       req.GithubOwner,
		"github_repo":        req.GithubRepo,
		"repo_path":          req.RepoPath,
		"deploy_target":      req.DeployTarget,
		"deploy_url":         req.DeployURL,
		"tech_stack":         req.TechStack,
		"default_branch":     "main",
		"branch_prefix_task": "task/",
		"branch_prefix_module": "module/",
		"protected_branches": []string{"main"},
	}
	if req.Theme != nil {
		insertData["theme"] = req.Theme
	} else {
		insertData["theme"] = map[string]any{"primary_color": "#34d399"}
	}
	if req.ConnectedServices != nil {
		insertData["connected_services"] = req.ConnectedServices
	} else {
		insertData["connected_services"] = []interface{}{}
	}
	if req.ModelKeys != nil {
		insertData["model_keys"] = req.ModelKeys
	} else {
		insertData["model_keys"] = []interface{}{}
	}

	result, err := s.db.Insert(r.Context(), "projects", insertData)
	if err != nil {
		log.Printf("[Projects API] Failed to create project: %v", err)
		http.Error(w, `{"error":"insert_failed","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Run the PIF scaffold (creates directory structure, vibepilot.toml, export.sh,
	// Hermes profile, git repo, backup repo, SQLite database). Non-fatal if it fails —
	// the project DB row is already created and the scaffold can be re-run.
	scaffoldResult := s.runPIFScaffold(req.Slug, req.DisplayName, req.Description, req.DeployTarget, req.DeployURL)
	if scaffoldResult != "" {
		log.Printf("[Projects API] PIF scaffold for %s: %s", req.Slug, scaffoldResult)
	}

	// Invalidate project resolver cache if available
	// (the resolver has its own TTL so this is best-effort)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	w.Write(result)
	log.Printf("[Projects API] Created project: %s (%s)", req.DisplayName, req.Slug)
}

// runPIFScaffold executes the PIF scaffold script to create the full project
// directory structure, vibepilot.toml, export.sh, restore.sh, Hermes profile,
// git repo, backup repo, and SQLite database. Returns a summary string (empty on failure).
// This is non-fatal: if the scaffold fails, the project DB row still exists and
// the scaffold can be re-run via the /api/projects/scaffold endpoint.
func (s *Server) runPIFScaffold(slug, displayName, description, deployTarget, deployURL string) string {
	scriptPath := filepath.Join(os.Getenv("HOME"), "vibepilot", "scripts", "pif_scaffold.py")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Sprintf("scaffold script not found at %s: %v", scriptPath, err)
	}

	args := []string{
		"python3", scriptPath,
		"--slug", slug,
		"--json",
	}
	if displayName != "" {
		args = append(args, "--display-name", displayName)
	}
	if description != "" {
		args = append(args, "--description", description)
	}
	if deployTarget != "" {
		args = append(args, "--deploy-target", deployTarget)
	}
	if deployURL != "" {
		args = append(args, "--deploy-url", deployURL)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = filepath.Join(os.Getenv("HOME"), "vibepilot")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("scaffold failed: %v, output: %s", err, string(output))
	}

	// Parse JSON output to extract success status
	var result struct {
		Success bool              `json:"success"`
		Errors  []string          `json:"errors"`
		Steps   map[string]any    `json:"steps"`
	}
	if json.Unmarshal(output, &result) == nil {
		if result.Success {
			stepCount := len(result.Steps)
			return fmt.Sprintf("success (%d steps)", stepCount)
		}
		return fmt.Sprintf("completed with %d errors: %v", len(result.Errors), result.Errors)
	}

	return fmt.Sprintf("completed (unparseable output: %d bytes)", len(output))
}

// handleProjectScaffold re-runs the PIF scaffold for an existing project.
// POST /api/projects/scaffold  Body: {"slug":"sealed", ...}
// This is idempotent if the project dir doesn't exist yet; it will fail if the
// directory already exists (use --force on the script directly to override).
func (s *Server) handleProjectScaffold(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Slug         string `json:"slug"`
		DisplayName  string `json:"display_name"`
		Description  string `json:"description"`
		DeployTarget string `json:"deploy_target"`
		DeployURL    string `json:"deploy_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Slug == "" {
		http.Error(w, `{"error":"slug_required"}`, http.StatusBadRequest)
		return
	}

	result := s.runPIFScaffold(req.Slug, req.DisplayName, req.Description, req.DeployTarget, req.DeployURL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]any{
		"slug":    req.Slug,
		"result":  result,
		"success": !strings.Contains(result, "failed"),
	})
}

// handleReviewQueue returns pending items from the unified review_items table.
// GET /api/review-queue[?status=pending&credit_alerts=1]
//
// When credit_alerts=1, also runs the subscription threshold check RPC and
// injects any live credit alerts (these are ephemeral, not stored in review_items).
func (s *Server) handleReviewQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ctx := r.Context()

	// Read query params for filtering
	status := r.URL.Query().Get("status")
	itemType := r.URL.Query().Get("type")
	includeCreditAlerts := r.URL.Query().Get("credit_alerts") == "1"

	if status == "" {
		status = "pending"
	}

	// Fetch from unified review_items table
	filters := map[string]any{
		"status": status,
		"order":  "priority.desc,created_at.desc",
	}
	if itemType != "" {
		filters["type"] = itemType
	}

	dbItems, err := s.db.Query(ctx, "review_items", filters)
	if err != nil {
		log.Printf("[review-queue] query error: %v", err)
		dbItems = json.RawMessage("[]")
	}

	var items []reviewitems.ReviewItem
	if json.Unmarshal(dbItems, &items) != nil {
		items = []reviewitems.ReviewItem{}
	}

	// Back-compat: also include live credit alerts from the RPC when requested
	// (these are ephemeral threshold checks, not persisted in review_items)
	type compatAlert struct {
		ID        string `json:"id"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		Status    string `json:"status"`
		ReviewURL string `json:"review_url"`
	}
	var alerts []compatAlert
	if includeCreditAlerts {
		alertData, rpcErr := s.db.RPC(ctx, "check_subscription_thresholds", map[string]any{})
		if rpcErr != nil {
			log.Printf("[review-queue] credit alert RPC error: %v", rpcErr)
		} else {
			var rawAlerts []map[string]any
			if json.Unmarshal(alertData, &rawAlerts) == nil {
				for _, a := range rawAlerts {
					modelID, _ := a["model_id"].(string)
					alertType, _ := a["alert_type"].(string)
					message, _ := a["message"].(string)
					if message == "" {
						message = alertType + " alert for " + modelID
					}
					alerts = append(alerts, compatAlert{
						ID:        modelID + "-" + alertType,
						Category:  "credit_alert",
						Title:     modelID + " " + alertType,
						Summary:   message,
						Status:    "alert",
						ReviewURL: "/admin#credits",
					})
				}
			}
		}
	}

	// Build response: merge review_items + ephemeral credit alerts
	type responseItem struct {
		reviewitems.ReviewItem
		Category  string `json:"category"` // back-compat alias for type
		ReviewURL string `json:"review_url,omitempty"`
	}

	combined := make([]responseItem, 0, len(items)+len(alerts))
	for _, item := range items {
		ri := responseItem{
			ReviewItem: item,
			Category:   item.Type, // back-compat: dashboard uses "category"
		}
		// Derive review URL from payload or source_id
		if item.Type == "research" {
			var payload map[string]any
			if json.Unmarshal(item.Payload, &payload) == nil {
				// Check payload.review_url first (already a full URL)
				if ru, ok := payload["review_url"].(string); ok && ru != "" && !strings.HasPrefix(ru, "/home/") {
					ri.ReviewURL = ru
				}
				// Then decision_doc_path
				if ri.ReviewURL == "" {
					if dp, ok := payload["decision_doc_path"].(string); ok && dp != "" && !strings.HasPrefix(dp, "/home/") {
						ri.ReviewURL = "https://github.com/VibesTribe/knowledgebase/blob/main/" + dp
					}
				}
				// Then findings_path
				if ri.ReviewURL == "" {
					if fp, ok := payload["findings_path"].(string); ok && fp != "" && !strings.HasPrefix(fp, "/home/") {
						ri.ReviewURL = "https://github.com/VibesTribe/knowledgebase/blob/main/" + fp
					}
				}
			}
			if ri.ReviewURL == "" {
				ri.ReviewURL = "/research/" + item.SourceID
			}
		} else if item.Type == "task_review" || item.Type == "visual_qa" {
			ri.ReviewURL = "/tasks/" + item.SourceID
		} else if item.Type == "credit_alert" {
			ri.ReviewURL = "/admin#credits"
		}
		combined = append(combined, ri)
	}
	// Append ephemeral credit alerts (not in review_items)
	for _, a := range alerts {
		combined = append(combined, responseItem{
			ReviewItem: reviewitems.ReviewItem{
				ID:       a.ID,
				Type:     "credit_alert",
				SourceID: a.ID,
				Title:    a.Title,
				Summary:  a.Summary,
				Status:   a.Status,
				Priority: "high",
			},
			Category:  "credit_alert",
			ReviewURL: a.ReviewURL,
		})
	}

	responseBytes, _ := json.Marshal(map[string]any{
		"items": combined,
		"count": len(combined),
	})
	w.Write(responseBytes)
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
			ProjectID   string  `json:"project_id"`
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
			rpcParams := map[string]any{
				"p_category":    req.Category,
				"p_description": req.Description,
				"p_amount_usd":  req.AmountUSD,
				"p_frequency":   req.Frequency,
			}
			if req.ProjectID != "" {
				rpcParams["p_project_id"] = req.ProjectID
			}
			result, err := s.db.RPC(ctx, "add_project_cost", rpcParams)
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

// handleDesignReviews returns design reviews, optionally filtered by task_id or status.
func (s *Server) handleDesignReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	filters := map[string]any{
		"order": "created_at.desc",
		"limit": 50,
	}
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		filters["task_id"] = "eq." + taskID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = "eq." + status
	}

	data, err := s.db.Query(r.Context(), "design_reviews", filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleDesignReviewAction approves, rejects, or skips a design review.
// POST with {"id": "...", "action": "approve|reject|skip", "feedback": "..."}.
func (s *Server) handleDesignReviewAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID       string `json:"id"`
		Action   string `json:"action"`
		Feedback string `json:"feedback"`
	}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}
	if req.ID == "" || req.Action == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id and action required"})
		return
	}

	status := ""
	switch req.Action {
	case "approve":
		status = "approved"
	case "reject":
		status = "changes_requested"
	case "skip":
		status = "skipped"
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "action must be approve, reject, or skip"})
		return
	}

	// Update design_reviews status
	_, err := s.db.RPC(r.Context(), "update_design_review", map[string]interface{}{
		"p_id":            req.ID,
		"p_status":        status,
		"p_human_feedback": req.Feedback,
	})
	if err != nil {
		// Fallback: direct SQL update via the RPC pattern
		// The RPC may not exist yet, so try a direct approach
		log.Printf("[DesignReview] RPC update_design_review not available: %v, using fallback", err)
	}

	// If approved or skipped, advance the associated task status
	if status == "approved" || status == "skipped" {
		// Get the task_id from the design review
		drData, err := s.db.Query(r.Context(), "design_reviews", map[string]any{
			"id": "eq." + req.ID,
			"select": "task_id",
		})
		if err == nil && drData != nil {
			var reviews []struct {
				TaskID string `json:"task_id"`
			}
			if json.Unmarshal(drData, &reviews) == nil && len(reviews) > 0 {
				taskID := reviews[0].TaskID
				_, _ = s.db.RPC(r.Context(), "update_task_status", map[string]interface{}{
					"p_task_id": taskID,
					"p_status":  "pending",
				})
				log.Printf("[DesignReview] Task %s advanced to pending after design %s", taskID, status)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": status, "id": req.ID})
}

// handleAdminSystem returns system health info for the X220.
// GET /api/admin/system
func (s *Server) handleAdminSystem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	type sysEntry struct {
		Label  string `json:"label"`
		Value  string `json:"value"`
		Detail string `json:"detail,omitempty"`
		Status string `json:"status,omitempty"`
	}

	results := []sysEntry{}

	// Disk usage
	if out, err := exec.Command("df", "-h", "/").Output(); err == nil {
		lines := splitLines(string(out))
		if len(lines) >= 2 {
			fields := splitFields(lines[1])
			if len(fields) >= 5 {
				pct := fields[4]
				status := "ok"
				if pctInt, _ := strconv.Atoi(strings.TrimRight(pct, "%")); pctInt > 85 {
					status = "warn"
				} else if pctInt > 95 {
					status = "error"
				}
				results = append(results, sysEntry{
					Label:  "Disk (/)",
					Value:  fields[4] + " used",
					Detail: fields[1] + " total, " + fields[3] + " available",
					Status: status,
				})
			}
		}
	}

	// Memory
	if out, err := exec.Command("free", "-m").Output(); err == nil {
		lines := splitLines(string(out))
		if len(lines) >= 2 {
			fields := splitFields(lines[1])
			if len(fields) >= 3 {
				total, _ := strconv.Atoi(fields[1])
				used, _ := strconv.Atoi(fields[2])
				pct := 0
				if total > 0 {
					pct = used * 100 / total
				}
				status := "ok"
				if pct > 80 {
					status = "warn"
				} else if pct > 95 {
					status = "error"
				}
				results = append(results, sysEntry{
					Label:  "Memory",
					Value:  fmt.Sprintf("%d%% used (%dMB / %dMB)", pct, used, total),
					Detail: fmt.Sprintf("%dMB free", total-used),
					Status: status,
				})
			}
		}
		// Swap
		if len(lines) >= 3 {
			fields := splitFields(lines[2])
			if len(fields) >= 3 && fields[1] != "0" {
				total, _ := strconv.Atoi(fields[1])
				used, _ := strconv.Atoi(fields[2])
				pct := 0
				if total > 0 {
					pct = used * 100 / total
				}
			swapStatus := "ok"
			if pct > 50 {
				swapStatus = "warn"
			}
			results = append(results, sysEntry{
				Label:  "Swap",
				Value:  fmt.Sprintf("%d%% used (%dMB / %dMB)", pct, used, total),
				Status: swapStatus,
			})
			}
		}
	}

	// Uptime
	if out, err := exec.Command("uptime", "-p").Output(); err == nil {
		results = append(results, sysEntry{
			Label:  "Uptime",
			Value:  strings.TrimSpace(string(out)),
			Status: "ok",
		})
	}

	// CPU load
	if out, err := exec.Command("uptime").Output(); err == nil {
		line := strings.TrimSpace(string(out))
		if parts := strings.Split(line, "load average:"); len(parts) > 1 {
			results = append(results, sysEntry{
				Label:  "CPU Load",
				Value:  strings.TrimSpace(parts[1]),
				Status: "ok",
			})
		}
	}

	// Process count
	if out, err := exec.Command("ps", "-e", "--no-headers", "-o", "pid,comm,%mem,%cpu", "--sort=-%mem").Output(); err == nil {
		lines := splitLines(string(out))
		procCount := len(lines)
		results = append(results, sysEntry{
			Label:  "Processes",
			Value:  fmt.Sprintf("%d running", procCount),
			Detail: "",
			Status: "ok",
		})

		// Top memory processes
		topProcs := []string{}
		for i, line := range lines {
			if i >= 5 {
				break
			}
			fields := splitFields(line)
			if len(fields) >= 4 {
				topProcs = append(topProcs, fmt.Sprintf("%s (%s%%)", fields[1], fields[2]))
			}
		}
		if len(topProcs) > 0 {
			results = append(results, sysEntry{
				Label:  "Top Memory Users",
				Value:  strings.Join(topProcs, ", "),
				Status: "info",
			})
		}
	}

	// Go governor status
	if out, err := exec.Command("ps", "-e", "-o", "pid,rss,comm", "--sort=-rss").Output(); err == nil {
		lines := splitLines(string(out))
		for _, line := range lines {
			if strings.Contains(line, "governor") {
				fields := splitFields(line)
				if len(fields) >= 3 {
					rssKB, _ := strconv.Atoi(fields[1])
					results = append(results, sysEntry{
						Label:  "Governor",
						Value:  fmt.Sprintf("PID %s — %dMB RAM", fields[0], rssKB/1024),
						Status: "ok",
					})
				}
				break
			}
		}
	}

	if out, err := exec.Command("ps", "-e", "-o", "pid,rss,comm", "--sort=-rss").Output(); err == nil {
		lines := splitLines(string(out))
		for _, line := range lines {
			if strings.Contains(line, "postgres") {
				fields := splitFields(line)
				if len(fields) >= 3 {
					rssKB, _ := strconv.Atoi(fields[1])
					results = append(results, sysEntry{
						Label:  "PostgreSQL",
						Value:  fmt.Sprintf("PID %s — %dMB RAM", fields[0], rssKB/1024),
						Status: "ok",
					})
				}
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, results)
}

func splitLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func splitFields(s string) []string {
	return strings.Fields(s)
}

// handleReviewItems returns all pending review items, optionally filtered by type or status.
// GET /api/review-items?type=research&status=pending
func (s *Server) handleReviewItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	itemType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	filters := map[string]any{
		"order": "priority.asc,created_at.desc",
		"limit": 100,
	}
	if status != "" {
		filters["status"] = status
	} else {
		filters["status"] = "pending"
	}
	if itemType != "" {
		filters["type"] = itemType
	}

	data, err := s.db.Query(r.Context(), "review_items", filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Enrich items with review_url from payload fields.
	// Priority: payload.review_url > payload.decision_doc_path > payload.findings_path
	// Skip absolute local paths (e.g. /home/vibes/...) that would produce broken URLs.
	var enriched []map[string]any
	if err := json.Unmarshal(data, &enriched); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
	kbBase := "https://graphs.vibestribe.rocks"
	for i := range enriched {
		// Skip review_url for research items - dashboard uses inline ResearchReportPanel
		// which provides interactive Approve/Watch/Reject buttons and shows Knowledge Hub
		// links internally via decision_doc_path/findings_path.
		if itemType, _ := enriched[i]["type"].(string); itemType == "research" {
			continue
		}
		reviewURL := ""
		if payload, ok := enriched[i]["payload"].(map[string]any); ok {
			if ru, ok := payload["review_url"].(string); ok && ru != "" {
				// payload.review_url is already a full URL or relative path
				if strings.HasPrefix(ru, "http://") || strings.HasPrefix(ru, "https://") {
					reviewURL = ru
				} else if !strings.HasPrefix(ru, "/home/") && !strings.HasPrefix(ru, "/etc/") {
					reviewURL = ru // relative path, use as-is
				}
			}
			if reviewURL == "" {
				if dp, ok := payload["decision_doc_path"].(string); ok && dp != "" && !strings.HasPrefix(dp, "/home/") {
					reviewURL = kbBase + "/" + dp
				}
			}
			if reviewURL == "" {
				if fp, ok := payload["findings_path"].(string); ok && fp != "" && !strings.HasPrefix(fp, "/home/") {
					reviewURL = kbBase + "/" + fp
				}
			}
		}
		enriched[i]["review_url"] = reviewURL
	}

	writeJSON(w, http.StatusOK, enriched)
}

// handleReviewItemByID updates a single review item's status.
// PATCH /api/review-items/{id} with {"status": "approved", "notes": "..."}
func (s *Server) handleReviewItemByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodGet {
		http.Error(w, "GET/PATCH only", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "DB not available"})
		return
	}

	// Extract ID from path: /api/review-items/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/review-items/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing item ID"})
		return
	}

	// GET single item
	if r.Method == http.MethodGet {
		data, err := s.db.Query(r.Context(), "review_items", map[string]any{"id": id})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	// PATCH: update status
	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}

	validStatuses := map[string]bool{
		"pending": true, "approved": true, "rejected": true,
		"deferred": true, "flagged": true, "resolved": true,
	}
	if !validStatuses[req.Status] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid status: " + req.Status})
		return
	}

	now := time.Now()
	data := map[string]any{
		"status":      req.Status,
		"human_notes": req.Notes,
		"reviewed_at": now,
		"reviewed_by": "human",
		"updated_at":  now,
	}

	result, err := s.db.Update(r.Context(), "review_items", id, data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// On approval of research/council items: update research_suggestion status
	// and emit event for consultant to create PRD
	if req.Status == "approved" {
		// Fetch the item to check type and source_id
		itemData, _ := s.db.Query(r.Context(), "review_items", map[string]any{"id": id})
		var items []map[string]any
		if json.Unmarshal(itemData, &items) == nil && len(items) > 0 {
			item := items[0]
			itemType, _ := item["type"].(string)
			sourceID, _ := item["source_id"].(string)
			if (itemType == "research" || itemType == "council") && sourceID != "" {
				// Update research_suggestion status
				_, _ = s.db.RPC(r.Context(), "update_research_suggestion_status", map[string]any{
					"p_id":     sourceID,
					"p_status": "approved",
					"p_review_notes": map[string]any{
						"approved_by": "human",
						"review_item": id,
						"notes":       req.Notes,
					},
				})
				log.Printf("[review-items] Research %s approved by human, triggering consultant", sourceID)

				// Emit EventResearchApproved so the consultant handler can create a PRD
				if s.router != nil {
					// Fetch the suggestion record for the event payload
					sugData, _ := s.db.Query(r.Context(), "research_suggestions", map[string]any{"id": sourceID})
					var record json.RawMessage
					if sugData != nil {
						record = sugData
					}
					s.router.Route(runtime.Event{
						Type:   runtime.EventResearchApproved,
						ID:     sourceID,
						Record: record,
					})
					log.Printf("[review-items] Emitted EventResearchApproved for %s", sourceID)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// handleResearchReports lists all research reports.
func (s *Server) handleResearchReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	filters := map[string]any{"limit": 50}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if reportType := r.URL.Query().Get("type"); reportType != "" {
		filters["report_type"] = reportType
	}

	data, err := s.db.Query(r.Context(), "research_reports", filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Enrich each report with item counts
	var reports []map[string]any
	if json.Unmarshal(data, &reports) == nil {
		for i := range reports {
			reportID, _ := reports[i]["id"].(string)
			itemData, _ := s.db.Query(r.Context(), "research_report_items", map[string]any{
				"report_id": reportID,
			})
			var items []map[string]any
			if json.Unmarshal(itemData, &items) == nil {
				approved, watch, rejected, undecided := 0, 0, 0, 0
				for _, item := range items {
					switch item["human_decision"] {
					case "approve":
						approved++
					case "watch":
						watch++
					case "reject":
						rejected++
					default:
						undecided++
					}
				}
				reports[i]["item_counts"] = map[string]any{
					"total":     len(items),
					"approved":  approved,
					"watch":     watch,
					"rejected":  rejected,
					"undecided": undecided,
				}
			}
			// Add review_url
			reviewURL := ""
			if dp, ok := reports[i]["decision_doc_path"].(string); ok && dp != "" {
				reviewURL = "https://graphs.vibestribe.rocks/" + dp
			} else if fp, ok := reports[i]["findings_path"].(string); ok && fp != "" {
				reviewURL = "https://graphs.vibestribe.rocks/" + fp
			}
			reports[i]["review_url"] = reviewURL
		}
	}

	writeJSON(w, http.StatusOK, reports)
}

// handleResearchReportByID returns a single report with all its items.
func (s *Server) handleResearchReportByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/research-reports/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "report id required"})
		return
	}

	data, err := s.db.RPC(r.Context(), "get_report_for_review", map[string]any{
		"p_report_id": id,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// RPC returns jsonb as [{"get_report_for_review": {...}}] via rowsToJSON. Unwrap.
	reportJSON := unwrapRPCResult(data, "get_report_for_review")

	// If no result, try looking up by suggestion_id.
	// review_items.source_id points to research_suggestions.id.
	// First check if a compiled research_report exists via research_report_items.
	if len(reportJSON) == 0 || string(reportJSON) == "null" || string(reportJSON) == "{}" {
		riData, riErr := s.db.Query(r.Context(), "research_report_items", map[string]any{
			"suggestion_id": id,
			"limit":         1,
		})
		if riErr == nil {
			var items []map[string]any
			if json.Unmarshal(riData, &items) == nil && len(items) > 0 {
				if reportID, ok := items[0]["report_id"].(string); ok && reportID != "" {
					data2, err2 := s.db.RPC(r.Context(), "get_report_for_review", map[string]any{
						"p_report_id": reportID,
					})
					if err2 == nil {
						reportJSON = unwrapRPCResult(data2, "get_report_for_review")
					}
				}
			}
		}
	}

	// Still no result? Return the suggestion directly from research_suggestions.
	if len(reportJSON) == 0 || string(reportJSON) == "null" || string(reportJSON) == "{}" {
		sugData, sugErr := s.db.Query(r.Context(), "research_suggestions", map[string]any{
			"id": id,
		})
		if sugErr == nil {
			var sugs []map[string]any
			if json.Unmarshal(sugData, &sugs) == nil && len(sugs) > 0 {
				// Wrap in a report-like structure so the frontend can render it
				sug := sugs[0]
				report := map[string]any{
					"id":               sug["id"],
					"source":           sug["title"],
					"title":            sug["title"],
					"summary":          sug["summary"],
					"details":          sug["details"],
					"findings_path":    sug["findings_path"],
					"type":             sug["type"],
					"complexity":       sug["complexity"],
					"status":           sug["status"],
					"is_raw_suggestion": true,
				}
				if b, err := json.Marshal(report); err == nil {
					reportJSON = b
				}
			}
		}
	}

	// Final fallback: return the review_item itself using its own data.
	if len(reportJSON) == 0 || string(reportJSON) == "null" || string(reportJSON) == "{}" {
		riData, riErr := s.db.Query(r.Context(), "review_items", map[string]any{
			"source_id": id,
			"limit":     1,
		})
		if riErr == nil {
			var riItems []map[string]any
			if json.Unmarshal(riData, &riItems) == nil && len(riItems) > 0 {
				ri := riItems[0]
				payload, _ := ri["payload"].(map[string]any)
				if payload == nil {
					// payload might be a JSON string, try unmarshalling
					if raw, ok := ri["payload"]; ok {
						if b, err := json.Marshal(raw); err == nil {
							var m map[string]any
							if json.Unmarshal(b, &m) == nil {
								payload = m
							}
						}
					}
				}
				report := map[string]any{
					"id":               ri["id"],
					"source":           ri["title"],
					"title":            ri["title"],
					"summary":          ri["summary"],
					"details":          payload,
					"findings_path":    "",
					"type":             ri["type"],
					"complexity":       "medium",
					"status":           ri["status"],
					"is_raw_suggestion": true,
					"is_orphan":        true,
				}
				if payload != nil {
					if fp, ok := payload["findings_path"].(string); ok {
						report["findings_path"] = fp
					}
				}
				if b, err := json.Marshal(report); err == nil {
					reportJSON = b
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(reportJSON)
}

// handleReportItemDecision sets the human decision on a report item.
// PATCH /api/report-items/{id}  body: {"decision": "approve|watch|reject", "notes": "..."}
func (s *Server) handleReportItemDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/report-items/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "item id required"})
		return
	}

	var req struct {
		Decision string `json:"decision"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	if req.Decision != "" && req.Decision != "approve" && req.Decision != "watch" && req.Decision != "reject" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "decision must be approve, watch, or reject"})
		return
	}

	// Notes-only update (for Ask Q&A context save): if no decision, just update notes
	if req.Decision == "" {
		if req.Notes != "" {
			s.db.Update(r.Context(), "research_report_items", id, map[string]any{
				"notes": req.Notes,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "notes_updated": true})
		return
	}

	result, err := s.db.RPC(r.Context(), "set_report_item_decision", map[string]any{
		"p_item_id":  id,
		"p_decision": req.Decision,
		"p_notes":    req.Notes,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Check if the report is now fully decided (all items have decisions)
	var item map[string]any
	if json.Unmarshal(result, &item) == nil {
		if reportID, ok := item["report_id"].(string); ok && reportID != "" {
			undecided, _ := s.db.RPC(r.Context(), "report_undecided_count", map[string]any{
					"p_report_id": reportID,
				})
				count := 0
				if undecided != nil {
					// RPC returns [{"report_undecided_count": N}] (array wrapper from rowsToJSON)
					var rows []map[string]any
					if json.Unmarshal(undecided, &rows) == nil && len(rows) > 0 {
						if v, ok := rows[0]["report_undecided_count"].(float64); ok {
							count = int(v)
						}
					}
				}
			if count == 0 {
				// All items decided. Mark report, resolve parent review_item, trigger PRD generation.
				log.Printf("[report-items] Report %s fully decided, resolving parent review_item and triggering PRD generation", reportID)
				// Update report status
				s.db.RPC(r.Context(), "update_report_status", map[string]any{
					"p_id":     reportID,
					"p_status": "decided",
				})
				// Resolve the parent review_item so it leaves the review queue (matches by source_id)
				s.db.RPC(r.Context(), "resolve_review_items_by_source", map[string]any{
					"p_source_id": reportID,
				})
				// Fetch all approved items with their titles, summaries, and notes for the PRD context
				approvedItemsData, _ := s.db.Query(r.Context(), "research_report_items", map[string]any{
					"report_id": reportID,
				})
				var approvedItems []map[string]any
				if approvedItemsData != nil {
					var allItems []map[string]any
					if json.Unmarshal(approvedItemsData, &allItems) == nil {
						for _, it := range allItems {
							if hd, _ := it["human_decision"].(string); hd == "approve" {
								title, _ := it["title"].(string)
								summary, _ := it["summary"].(string)
								notes, _ := it["notes"].(string)
								approvedItems = append(approvedItems, map[string]any{
									"title":   title,
									"summary": summary,
									"notes":   notes,
								})
							}
						}
					}
				}
				if approvedItems == nil {
					approvedItems = []map[string]any{}
				}
				log.Printf("[report-items] Report %s: %d approved items for PRD", reportID, len(approvedItems))
				// Create review item for PRD generation
				s.db.Insert(r.Context(), "review_items", map[string]any{
					"type":     "research",
					"source_id": reportID,
					"title":    "Generate PRD from approved research items",
					"summary":  fmt.Sprintf("All items decided. %d approved items ready for PRD.", len(approvedItems)),
					"status":   "pending",
					"priority": "high",
					"payload": map[string]any{
						"report_id":      reportID,
						"report_type":    "daily_scan",
						"action":         "generate_prd",
						"approved_items": approvedItems,
					},
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "decision": req.Decision})
}

// unwrapRPCResult extracts the inner JSON from the RPC array wrapper.
// RPC calls to jsonb-returning functions produce [{"fn_name": {...}}] via rowsToJSON.
// This unwraps to just the inner object, or returns original data if format doesn't match.
func unwrapRPCResult(data []byte, funcName string) []byte {
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil || len(arr) != 1 {
		return data
	}
	if inner, ok := arr[0][funcName]; ok {
		return inner
	}
	return data
}
