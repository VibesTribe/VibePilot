# PLAN: Hello World Endpoint

## Overview
Add a `/hello` GET endpoint to the governor HTTP server that returns `{"message":"Hello from VibePilot"}`. The HTTP server lives in `governor/internal/webhooks/server.go` and uses `net/http.ServeMux`. Only one file needs modification.

## Tasks

### T001: Add /hello GET endpoint
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello GET endpoint

## Context
The governor runs an HTTP server (in governor/internal/webhooks/server.go) for receiving Supabase webhooks. It uses a standard net/http.ServeMux. Currently only the webhook path is registered. We need a simple /hello healthcheck endpoint that returns a JSON greeting.

## What to Build

In file governor/internal/webhooks/server.go, make these changes:

1. In the Start() method (around line 74), after the mux is created and the webhook handler is registered:
   - Add: mux.HandleFunc("/hello", s.handleHello)

2. Add a new method on *Server:

   func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
       if r.Method != http.MethodGet {
           http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
           return
       }
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(map[string]string{"message": "Hello from VibePilot"})
   }

   Note: encoding/json is already imported in this file.

3. That is it. Two lines in Start() (one blank line separator, one HandleFunc), one new method (~7 lines). Total ~10 lines of new code. No new imports needed.

## Files
- governor/internal/webhooks/server.go - Add route registration in Start() and new handleHello method
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/internal/webhooks/server.go"],
  "lines_added": 10,
  "tests_written": [],
  "verify": "curl http://localhost:8080/hello returns {\"message\":\"Hello from VibePilot\"}"
}
```
