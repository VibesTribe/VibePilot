package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/courier"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/pkg/types"
)

//go:embed dist
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		host := r.Header.Get("Origin")
		if host == "" {
			return true
		}
		allowedOrigins := []string{
			"localhost",
			"127.0.0.1",
			".vercel.app",
		}
		for _, allowed := range allowedOrigins {
			if strings.HasSuffix(host, allowed) {
				return true
			}
		}
		return false
	},
}

type Server struct {
	cfg          *config.ServerConfig
	db           *db.DB
	server       *http.Server
	hub          *Hub
	courierCb    func(courier.Result)
	moduleCounts func() map[string]int
}

func New(cfg *config.ServerConfig, database *db.DB) *Server {
	return &Server{
		cfg: cfg,
		db:  database,
		hub: NewHub(),
	}
}

func (s *Server) SetCourierCallback(cb func(courier.Result)) {
	s.courierCb = cb
}

func (s *Server) SetModuleCountsGetter(fn func() map[string]int) {
	s.moduleCounts = fn
}

func (s *Server) Hub() *Hub {
	return s.hub
}

func (s *Server) Start() error {
	go s.hub.Run()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/task/", s.handleTask)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/platforms", s.handlePlatforms)
	mux.HandleFunc("/api/roi", s.handleROI)
	mux.HandleFunc("/api/limits", s.handleLimits)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/webhook/courier", s.handleCourierWebhook)

	distFS, err := fs.Sub(staticFS, "dist")
	if err != nil {
		log.Printf("Warning: could not load embedded dist: %v", err)
		mux.HandleFunc("/", s.handleIndex)
	} else {
		mux.Handle("/", http.FileServer(http.FS(distFS)))
	}

	s.server = &http.Server{
		Addr:         s.cfg.Addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Server starting on %s", s.cfg.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.server != nil {
		s.server.Shutdown(ctx)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>VibePilot Governor</title></head>
<body>
<h1>VibePilot Governor</h1>
<p>API endpoints:</p>
<ul>
<li>GET /health - Health check</li>
<li>GET /api/stats - Quick stats overview</li>
<li>GET /api/tasks?status={status} - List tasks (default: available)</li>
<li>GET /api/task/{id} - Get task details</li>
<li>GET /api/models - List active runners</li>
<li>GET /api/platforms - List web platforms</li>
<li>GET /api/roi - ROI summary</li>
<li>GET /api/limits - Per-module limits</li>
<li>WS /ws - WebSocket for real-time updates</li>
<li>POST /webhook/courier - Courier completion callback</li>
</ul>
</body>
</html>`)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "available"
	}

	limit := 50
	tasks, err := s.db.GetTasksByStatus(ctx, status, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, tasks)
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Path[len("/api/task/"):]
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	packet, err := s.db.GetTaskPacket(ctx, taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if packet == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, packet)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	runners, err := s.db.GetRunners(ctx)
	if err != nil {
		s.jsonResponse(w, []interface{}{})
		return
	}

	models := make([]map[string]interface{}, len(runners))
	for i, r := range runners {
		models[i] = map[string]interface{}{
			"id":          r.ModelID,
			"runner_id":   r.ID,
			"tool":        r.ToolID,
			"priority":    r.CostPriority,
			"daily_used":  r.DailyUsed,
			"daily_limit": r.DailyLimit,
			"status":      "active",
		}
	}

	s.jsonResponse(w, models)
}

func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	platforms := []map[string]interface{}{
		{"id": "chatgpt", "name": "ChatGPT", "status": "active"},
		{"id": "claude", "name": "Claude", "status": "active"},
		{"id": "gemini", "name": "Gemini", "status": "active"},
	}

	s.jsonResponse(w, platforms)
}

func (s *Server) handleROI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	summary, err := s.db.GetROISummary(ctx)
	if err != nil {
		summary = &db.ROISummary{}
	}

	roi := map[string]interface{}{
		"total_runs":       summary.TotalRuns,
		"total_tokens_in":  summary.TotalTokensIn,
		"total_tokens_out": summary.TotalTokensOut,
		"total_tokens":     summary.TotalTokensIn + summary.TotalTokensOut,
		"theoretical_cost": summary.TheoreticalCost,
		"actual_cost":      summary.ActualCost,
		"savings":          summary.Savings,
	}

	s.jsonResponse(w, roi)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	s.hub.register <- client

	go client.writePump()
	client.readPump()
}

func (s *Server) Broadcast(task types.Task, event string) {
	s.hub.BroadcastTaskEvent(event, task.ID, string(task.Status), task.Title, task.SliceID)
}

func (s *Server) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limits := map[string]interface{}{
		"max_per_module": 8,
		"active_counts":  map[string]int{},
		"ws_clients":     s.hub.ClientCount(),
	}

	if s.moduleCounts != nil {
		limits["active_counts"] = s.moduleCounts()
	}

	s.jsonResponse(w, limits)
}

func (s *Server) handleCourierWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.courierCb == nil {
		http.Error(w, "Courier not configured", http.StatusServiceUnavailable)
		return
	}

	var result courier.Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if result.TaskID == "" {
		http.Error(w, "Missing task_id", http.StatusBadRequest)
		return
	}

	log.Printf("Server: courier webhook received for %s: %s", result.TaskID[:8], result.Status)

	s.courierCb(result)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	availableTasks, _ := s.db.GetTasksByStatus(ctx, "available", 50)
	inProgressTasks, _ := s.db.GetTasksByStatus(ctx, "in_progress", 50)
	reviewTasks, _ := s.db.GetTasksByStatus(ctx, "review", 50)

	roi, _ := s.db.GetROISummary(ctx)

	runners, _ := s.db.GetRunners(ctx)

	stats := map[string]interface{}{
		"tasks": map[string]int{
			"available":   len(availableTasks),
			"in_progress": len(inProgressTasks),
			"review":      len(reviewTasks),
		},
		"roi": map[string]interface{}{
			"total_runs":   roi.TotalRuns,
			"total_tokens": roi.TotalTokensIn + roi.TotalTokensOut,
			"savings":      roi.Savings,
		},
		"runners":        len(runners),
		"ws_clients":     s.hub.ClientCount(),
		"max_per_module": 8,
	}

	if s.moduleCounts != nil {
		stats["module_counts"] = s.moduleCounts()
	}

	s.jsonResponse(w, stats)
}
