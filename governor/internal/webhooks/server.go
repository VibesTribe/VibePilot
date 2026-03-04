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

type Server struct {
	port     int
	path     string
	secret   string
	router   *runtime.EventRouter
	github   *GitHubWebhookHandler
	server   *http.Server
	handlers map[string]EventHandler
}

type EventHandler func(ctx context.Context, payload *Payload) error

type Config struct {
	Port   int
	Path   string
	Secret string
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
		port:     cfg.Port,
		path:     cfg.Path,
		secret:   cfg.Secret,
		router:   router,
		handlers: make(map[string]EventHandler),
	}
}

func (s *Server) SetGitHubHandler(handler *GitHubWebhookHandler) {
	s.github = handler
}

func (s *Server) RegisterHandler(eventType string, handler EventHandler) {
	s.handlers[eventType] = handler
	log.Printf("[Webhooks] Registered handler for: %s", eventType)
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWebhook)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("[Webhooks] Server starting on port %d at %s", s.port, s.path)

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
		signature := r.Header.Get("X-Supabase-Signature")
		if !s.verifySignature(body, signature) {
			log.Printf("[Webhooks] Invalid signature")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
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
		case status == "available" && action == "INSERT":
			return string(runtime.EventTaskAvailable)
		case status == "available" && action == "UPDATE":
			if oldStatus, _ := payload.OldRecord["status"].(string); oldStatus != "available" {
				return string(runtime.EventTaskAvailable)
			}
		case status == "review":
			return string(runtime.EventTaskReview)
		case status == "testing" || status == "approval":
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

func (s *Server) GetPort() int {
	return s.port
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
