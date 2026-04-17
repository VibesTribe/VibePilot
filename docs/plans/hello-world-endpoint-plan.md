# PLAN: Hello World Endpoint

## Overview
Add a GET /hello endpoint to the governor webhook HTTP server (port 8080) that returns `{"message":"Hello from VibePilot"}`.

## Architecture Notes
The webhook server lives in `governor/internal/webhooks/server.go`. It creates an `http.NewServeMux()` in `Start()` (line 74) and registers routes via `mux.HandleFunc`. The package already imports `encoding/json` and `net/http` -- no new dependencies needed. The handler and route registration both go in this single file.

## Tasks

### T001: Add /hello GET handler and route
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /hello GET handler and route

## Context
The governor webhook server (port 8080) needs a health/readiness endpoint that external callers can hit to confirm the service is alive. This is a single GET route returning static JSON.

## What to Build

1. Add a handler method `handleHello` on the `Server` struct in `governor/internal/webhooks/server.go`:
   - Accept only GET. For any other method, return 405.
   - Set header `Content-Type: application/json`.
   - Write HTTP 200 with body `{"message":"Hello from VibePilot"}`.
   - Use `json.Marshal` on a struct or map (not raw string concatenation).
   - ~12 lines of code.

2. Register the route in the `Start()` method, right after the existing `mux.HandleFunc(s.path, s.handleWebhook)` line (line 75):
   ```go
   mux.HandleFunc("/hello", s.handleHello)
   ```

3. No new imports. `encoding/json` and `net/http` are already in the import block.

## Files
- `governor/internal/webhooks/server.go` -- add `handleHello` method + one route registration line
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
