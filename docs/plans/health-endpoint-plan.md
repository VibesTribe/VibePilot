# PLAN: Add /health Endpoint

## Overview
Add a GET /health endpoint to the webhook server that returns JSON with status and timestamp. Simple liveness probe for monitoring and load balancers.

## Tasks

### T001: Add /health handler and route to webhook server
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /health handler and route to webhook server

## Context
The webhook server (`governor/internal/webhooks/server.go`) currently only exposes the webhook path. We need a lightweight health check endpoint for monitoring, load balancers, and Kubernetes probes.

## What to Build

1. In `governor/internal/webhooks/server.go`, in the `Start()` method (line 73), add a second route on the mux BEFORE the webhook route:

   mux.HandleFunc("/health", s.handleHealth)

   This goes right after `mux := http.NewServeMux()` (line 74) and before the existing `mux.HandleFunc(s.path, s.handleWebhook)` (line 75).

2. Add a new handler method `handleHealth` on the Server struct:

   func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
       if r.Method != http.MethodGet {
           http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
           return
       }
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(map[string]any{
           "status":    "ok",
           "timestamp": time.Now().Unix(),
       })
   }

   Place this method right after the `Shutdown` method (after line 107) to keep handler methods grouped.

## Constraints
- No new imports needed -- `encoding/json`, `net/http`, and `time` are already imported.
- No external dependencies.
- Response must be HTTP 200 with Content-Type application/json.
- Timestamp is Unix epoch (int64), not ISO string.
- Handler only accepts GET. Return 405 for anything else.

## Files
- `governor/internal/webhooks/server.go` - Add handleHealth method + register route in Start()
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [],
  "files_modified": ["governor/internal/webhooks/server.go"],
  "tests_written": [],
  "tests_required": ["governor/internal/webhooks/server_test.go"],
  "verification": "curl http://localhost:8080/health returns {\"status\": \"ok\", \"timestamp\": <unix int>}"
}
```