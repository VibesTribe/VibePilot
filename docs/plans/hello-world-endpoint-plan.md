# PLAN: Hello World Endpoint

## Overview
Add a `/hello` GET endpoint to the governor webhook HTTP server that returns `{"message": "Hello from VibePilot"}` as JSON.

## Architecture Notes

The governor's HTTP server lives in `governor/internal/webhooks/server.go`. The `Start()` method creates a `ServeMux` on line 74 and registers routes on line 75. Currently only one route exists: the webhook path (default `/webhooks`).

The handler and route should both go in the webhooks package since that is where the HTTP server is defined. No new packages needed.

## Tasks

### T001: Add /hello GET handler and register route
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello GET endpoint to governor HTTP server

## Context
The governor runs an HTTP server (in `governor/internal/webhooks/server.go`) for receiving webhooks. We need to add a simple health-check style `/hello` endpoint that returns a JSON greeting. This is the only HTTP server in the governor, so this is where the route goes.

## What to Build

### 1. Add the handler method to `governor/internal/webhooks/server.go`

Add this method to the `Server` struct (after the existing `handleWebhook` method, around line 109):

```go
func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Hello from VibePilot"})
}
```

Note: `encoding/json` is already imported in this file (line 8).

### 2. Register the route in the `Start()` method

In the `Start()` method, after line 75 (`mux.HandleFunc(s.path, s.handleWebhook)`), add:

```go
mux.HandleFunc("/hello", s.handleHello)
```

## Files
- `governor/internal/webhooks/server.go` - Add `handleHello` method and register `/hello` route

## Constraints
- Use only stdlib (`net/http`, `encoding/json`) -- both already imported
- Maximum 10 lines of new code
- No new packages, no new files, no new dependencies
- Do NOT change the webhook path handler or any existing behavior
- The `/hello` route must work alongside the existing `/webhooks` route
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/internal/webhooks/server.go"],
  "files_created": [],
  "tests_written": [],
  "verification": "curl http://localhost:8080/hello returns {\"message\":\"Hello from VibePilot\"} with Content-Type application/json"
}
```

---
