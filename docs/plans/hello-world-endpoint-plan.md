# PLAN: Hello World Endpoint

## Overview
Add a `/hello` GET endpoint to the governor webhook HTTP server that returns `{"message":"Hello from VibePilot"}`. The governor's HTTP server is `internal/webhooks/server.go` — it creates an `http.NewServeMux()` in `Start()` and currently only registers the webhook path. We add a second route on the same mux.

## Tasks

### T001: Add `/hello` GET handler and register route
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello GET endpoint

## Context
The governor HTTP server runs in `internal/webhooks/server.go`. It uses `http.NewServeMux()` (line 74) and currently registers only one route: `mux.HandleFunc(s.path, s.handleWebhook)`. We need to add a simple `/hello` GET endpoint on the same mux for health-check / greeting purposes.

## What to Build

1. In `governor/internal/webhooks/server.go`:

   a. Add a new handler method on `*Server`:
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

   b. In the `Start()` method, after the existing `mux.HandleFunc(s.path, s.handleWebhook)` line (line 75), add:
   ```go
   mux.HandleFunc("/hello", s.handleHello)
   ```

2. The `encoding/json` import already exists in the file (line 8). No new imports needed.

3. No new files. No new packages. No CLI changes. Maximum ~10 lines of new code.

## Files
- `governor/internal/webhooks/server.go` — add handleHello method + register route in Start()

## Verification
After rebuilding:
```bash
curl http://localhost:8080/hello
# Expected: {"message":"Hello from VibePilot"}

curl -X POST http://localhost:8080/hello
# Expected: Method not allowed

curl http://localhost:8080/webhooks
# Existing webhook route must still work
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/internal/webhooks/server.go"],
  "files_created": [],
  "tests_written": [],
  "lines_added": 10
}
```
