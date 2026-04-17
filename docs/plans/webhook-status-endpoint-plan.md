# PLAN: Add /status Endpoint to Webhook Server

## Overview
Add a GET /status endpoint to the webhook server that returns governor runtime info as JSON. Two files modified: server.go (core logic) and main.go (wiring).

## Tasks

### T001: Add /status Endpoint to Webhook Server
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /status Endpoint to Webhook Server

## Context
The governor webhook server has no health/status endpoint. For operational monitoring and debugging, we need GET /status to return basic runtime info as JSON without any external dependencies or database queries.

## What to Build

### 1. Modify `governor/internal/webhooks/server.go`

Add a `startedAt time.Time` field and a `version string` field to the `Server` struct:
```go
type Server struct {
	port     int
	path     string
	secret   string
	router   *runtime.EventRouter
	github   *GitHubWebhookHandler
	server   *http.Server
	handlers map[string]EventHandler
	startedAt time.Time
	version   string
}
```

Add `Version string` to the `Config` struct (after `Secret`):
```go
type Config struct {
	Port   int
	Path   string
	Secret string
	Version string
}
```

In `NewServer`, store the version and set `startedAt` to `time.Now()`:
```go
return &Server{
	port:     cfg.Port,
	path:     cfg.Path,
	secret:   cfg.Secret,
	router:   router,
	handlers: make(map[string]EventHandler),
	startedAt: time.Now(),
	version:   cfg.Version,
}
```

In the `Start` method (around line 75), add the /status route BEFORE the existing webhook route:
```go
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc(s.path, s.handleWebhook)
	// ... rest unchanged
```

Add a new handler method:
```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"governor":        "vibepilot",
		"version":         s.version,
		"status":          "running",
		"uptime_seconds":  int(time.Since(s.startedAt).Seconds()),
	})
}
```

Note: `encoding/json` and `time` are already imported in server.go, so no import changes needed.

### 2. Modify `governor/cmd/governor/main.go`

In the `NewServer` call (around line 214), add the Version field:
```go
webhookServer := webhooks.NewServer(&webhooks.Config{
	Port:    cfg.GetWebhooksConfig().Port,
	Path:    cfg.GetWebhooksConfig().Path,
	Secret:  webhookSecret,
	Version: version,
}, eventRouter)
```

Note: `version` is already a package-level var in main.go (line 28: `version = "2.0.0"`, overridden at build time via ldflags). No other changes needed.

## Constraints
- Only modify these two files
- No new dependencies
- No database queries or external calls
- Must not break existing webhook handling
- The /status route must be registered on the same mux as webhooks

## Files
- `governor/internal/webhooks/server.go` - Add startedAt/version fields, handleStatus handler, /status route
- `governor/cmd/governor/main.go` - Pass version string to webhook server config
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": [
    "governor/internal/webhooks/server.go",
    "governor/cmd/governor/main.go"
  ],
  "tests_written": [],
  "verification": "curl http://localhost:8080/status returns {\"governor\":\"vibepilot\",\"version\":\"2.0.0\",\"status\":\"running\",\"uptime_seconds\":<number>} with HTTP 200"
}
```
