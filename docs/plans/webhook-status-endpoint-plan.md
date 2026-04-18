# PLAN: Add /status Endpoint to Webhook Server

## Overview
Add a GET /status endpoint to the webhook server that returns governor runtime info as JSON for operational monitoring and debugging.

## Tasks

### T001: Add /status endpoint with version and uptime tracking
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet

# TASK: T001 - Add /status endpoint to webhook server

## Context
The webhook server needs a health/monitoring endpoint that returns basic governor runtime info as JSON. This is for operational monitoring and debugging — operators need to verify the governor is alive and see its version and uptime at a glance.

## What to Build

### 1. Add fields to the Server struct in `governor/internal/webhooks/server.go`

Add two new unexported fields:
- `startTime time.Time` — set when `Start()` is called
- `version string` — governor version string

### 2. Add a SetVersion method

```go
func (s *Server) SetVersion(v string) {
	s.version = v
}
```

### 3. Track start time in Start() method

At the beginning of the `Start()` method body (after the mux is created), add:
```go
s.startTime = time.Now()
```

### 4. Add /status route in Start() method

Right after `mux.HandleFunc(s.path, s.handleWebhook)`, add:
```go
mux.HandleFunc("/status", s.handleStatus)
```

Note: Use `"/status"` as a literal path, not a field.

### 5. Add handleStatus handler method

Add this new method to the Server:

```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := struct {
		Governor      string `json:"governor"`
		Version       string `json:"version"`
		Status        string `json:"status"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}{
		Governor:      "vibepilot",
		Version:       s.version,
		Status:        "running",
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
```

### 6. Wire version in main.go (minimal 1-line addition)

In `governor/cmd/governor/main.go`, after the `webhookServer.SetGitHubHandler(githubHandler)` call (line 221), add:
```go
webhookServer.SetVersion(version)
```

This is a single line to pass the build-time version variable to the webhook server.

## Constraints
- The `/status` route must NOT conflict with the existing webhook path (default `/webhooks`)
- `handleStatus` must only accept GET requests
- Must use `application/json` content type
- No external dependencies, no database queries
- `time` is already imported in server.go
- `encoding/json` is already imported in server.go

## Files
- `governor/internal/webhooks/server.go` — primary changes: add fields, handler, route
- `governor/cmd/governor/main.go` — 1-line addition: `webhookServer.SetVersion(version)` after line 221

## Verification
After changes, run:
```bash
cd ~/VibePilot/governor && go build ./...
```
Then start the governor and test:
```bash
curl http://localhost:8080/status
```
Expected response:
```json
{"governor":"vibepilot","version":"2.0.0","status":"running","uptime_seconds":42}
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
  "verification": "curl http://localhost:8080/status returns 200 with governor, version, status, uptime_seconds fields"
}
```
