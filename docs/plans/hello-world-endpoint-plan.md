# PLAN: Hello World Endpoint

## Overview
Add a `/hello` GET endpoint to the governor HTTP server that returns `{"message": "Hello from VibePilot"}`.

## Architecture Notes
The governor's HTTP server lives in `internal/webhooks/server.go`. It uses `http.ServeMux` created in `Start()` (line 74). Currently only one route is registered: the webhook path. The `/hello` route should be added alongside it in that same `Start()` method, with the handler as a method on `*Server`.

## Tasks

### T001: Add /hello endpoint to webhook server
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello endpoint to webhook server

## Context
The governor exposes an HTTP server via `internal/webhooks/server.go` using `http.ServeMux`. Currently only the webhook path is registered. A health/status endpoint at `/hello` is needed for lightweight liveness checks.

## What to Build
1. Add a `handleHello` method on `*Server` in `internal/webhooks/server.go` that:
   - Only accepts GET requests (return 405 for other methods)
   - Sets `Content-Type: application/json`
   - Returns HTTP 200 with body `{"message": "Hello from VibePilot"}`
   - Uses `encoding/json` (already imported) to marshal the response

2. Register the `/hello` route in the `Start()` method (line 74-75 area), right after the existing `mux.HandleFunc(s.path, s.handleWebhook)` line:
   ```go
   mux.HandleFunc("/hello", s.handleHello)
   ```

## Implementation Details

### handleHello method (add after handleWebhook method, around line 109)
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

### Route registration (in Start(), after line 75)
```go
mux.HandleFunc("/hello", s.handleHello)
```

## Files
- `internal/webhooks/server.go` - Add handleHello method and register route in Start()

## Constraints
- No new dependencies (encoding/json and net/http already imported)
- No new files needed
- Maximum ~15 lines of new code
- Do NOT change any existing behavior
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