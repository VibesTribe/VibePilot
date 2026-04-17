# PLAN: Hello World Endpoint

## Overview
Add a GET /hello endpoint to the governor webhook HTTP server that returns `{"message":"Hello from VibePilot"}` with Content-Type application/json. The webhook server in `governor/internal/webhooks/server.go` already runs an HTTP server on port 8080 with a ServeMux. This task adds one additional route to that existing mux.

## Tasks

### T001: Add /hello handler and register route
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello handler and register route

## Context
The governor runs an HTTP server via `governor/internal/webhooks/server.go`. In the `Start()` method (line 73), it creates a ServeMux and registers the webhook route:

```go
mux := http.NewServeMux()
mux.HandleFunc(s.path, s.handleWebhook)
```

We need to add a simple `/hello` GET endpoint to this same mux for health-checking and basic connectivity testing.

## What to Build

1. Add a new method `handleHello` on the `Server` struct in `governor/internal/webhooks/server.go`:

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

2. Register it in the `Start()` method, right after the existing `mux.HandleFunc(s.path, s.handleWebhook)` line (line 75):

```go
mux.HandleFunc("/hello", s.handleHello)
```

3. No new imports needed -- `encoding/json` and `net/http` are already imported.

## Files
- `governor/internal/webhooks/server.go` -- add `handleHello` method and register `/hello` route in `Start()`
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
