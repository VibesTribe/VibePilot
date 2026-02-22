package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/pkg/types"
)

//go:embed dist
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	cfg     *config.ServerConfig
	db      *db.DB
	server  *http.Server
	clients map[*websocket.Conn]bool
}

func New(cfg *config.ServerConfig, database *db.DB) *Server {
	return &Server{
		cfg:     cfg,
		db:      database,
		clients: make(map[*websocket.Conn]bool),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/task/", s.handleTask)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/platforms", s.handlePlatforms)
	mux.HandleFunc("/api/roi", s.handleROI)
	mux.HandleFunc("/ws", s.handleWebSocket)

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

	for client := range s.clients {
		client.Close()
	}

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
<li>GET /api/tasks - List tasks</li>
<li>GET /api/task/{id} - Get task details</li>
<li>GET /api/models - List models</li>
<li>GET /api/platforms - List platforms</li>
<li>GET /api/roi - ROI report</li>
<li>WS /ws - WebSocket for real-time updates</li>
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
	tasks, err := s.db.GetAvailableTasks(ctx)
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

	models := []map[string]interface{}{
		{"id": "opencode", "name": "OpenCode (GLM-5)", "status": "active"},
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

	roi := map[string]interface{}{
		"total_tasks":     0,
		"total_tokens":    0,
		"theoretical_cost": 0.0,
		"actual_cost":     0.0,
		"savings":         0.0,
	}

	s.jsonResponse(w, roi)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.clients[conn] = true
	defer delete(s.clients, conn)

	log.Printf("WebSocket client connected")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) Broadcast(task types.Task, event string) {
	msg := map[string]interface{}{
		"event":   event,
		"task_id": task.ID,
		"status":  task.Status,
		"title":   task.Title,
	}

	data, _ := json.Marshal(msg)

	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
