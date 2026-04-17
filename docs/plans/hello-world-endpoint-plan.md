# PLAN: Hello World Endpoint

## Overview
Add a `/hello` GET endpoint to the governor webhook HTTP server that returns `{"message":"Hello from VibePilot"}` with `application/json` content type. One file change, ~15 lines of code.

## Architecture Notes
- The governor's HTTP server lives in `governor/internal/webhooks/server.go`
- It uses `net/http.NewServeMux()` — routes are registered via `mux.HandleFunc`
- Currently only one route is registered: the webhook path
- The `/hello` route will be added alongside it in the `Start()` method
- No new packages, no new files, no new dependencies

## Tasks

### T001: Add /hello GET endpoint to webhook server
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello GET endpoint to webhook server

## Context
The governor exposes an HTTP server via `governor/internal/webhooks/server.go` for receiving webhooks. A simple health/identity endpoint at `/hello` is needed so operators can verify the server is running and identify it as VibePilot. This is a single-file change adding ~12 lines.

## What to Build

In `governor/internal/webhooks/server.go`, in the `Start()` method (line 73), add a second route registration to the mux:

1. After line 75 (`mux.HandleFunc(s.path, s.handleWebhook)`), add:
   ```go
   mux.HandleFunc("/hello", s.handleHello)
   ```

2. Add a new handler method on the Server struct (place it after the `handleWebhook` method, around line 189):
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

No other files need to change. `encoding/json` is already imported. `net/http` is already imported.

## Files
- `governor/internal/webhooks/server.go` — add `/hello` route in `Start()`, add `handleHello` method
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/internal/webhooks/server.go"],
  "tests_written": [],
  "verification": "curl http://localhost:8080/hello returns {\"message\":\"Hello from VibePilot\"} with Content-Type application/json"
}
```

---

## Dependency Graph

```
T001 (no dependencies)
```

## Risk Assessment
- **Complexity:** Trivial — 12 lines in one file
- **Dependencies:** None (json and net/http already imported)
- **Blast radius:** Zero — new route, no existing code touched
- **Rollback:** Remove 2 lines from Start() and the handleHello method