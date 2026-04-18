# PLAN: Add /status endpoint to webhook server

## Overview
Add a GET /status endpoint to the webhook server that returns governor runtime info as JSON for operational monitoring.

## Tasks

### T001: Add /status endpoint with uptime tracking
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /status endpoint to webhook server

## Context
The webhook server needs a health/status endpoint for operational monitoring. Currently the mux only handles the webhook path. We need to add a /status route that returns runtime info as JSON.

## What to Build

Modify ONLY `governor/internal/webhooks/server.go`:

1. Add a `startTime time.Time` field to the `Server` struct (after the `server` field on line 25).

2. Add a `version string` field to the `Server` struct (after `startTime`).

3. Add a `version` parameter (string) to the `Config` struct (after `Secret`).

4. In `NewServer`, set `startTime: time.Now()` and `version: cfg.Version` in the returned struct. If cfg.Version is empty, default to "dev".

5. In the `Start` method, add a new route BEFORE the webhook route:
   ```go
   mux.HandleFunc("/status", s.handleStatus)
   ```
   This goes right after `mux := http.NewServeMux()` and before `mux.HandleFunc(s.path, s.handleWebhook)`.

6. Add a new handler method `handleStatus`:
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
           "status":           "running",
           "uptime_seconds": int(time.Since(s.startTime).Seconds()),
       })
   }
   ```

7. In `governor/cmd/governor/main.go`, pass the version to the webhook config. Change the `webhooks.NewServer` call (around line 214-218) to:
   ```go
   webhookServer := webhooks.NewServer(&webhooks.Config{
       Port:    cfg.GetWebhooksConfig().Port,
       Path:    cfg.GetWebhooksConfig().Path,
       Secret:  webhookSecret,
       Version: version,
   }, eventRouter)
   ```
   The `version` variable is already declared as `var version = "2.0.0"` in the same file (line 28).

## Important Notes
- The `/status` path is hardcoded — it is NOT configurable and should NOT be. It must always be available.
- Do NOT add any new imports beyond what already exists (time and encoding/json are already imported in server.go).
- The `int()` cast on uptime_seconds is intentional — the PRD specifies integer.
- The existing webhook handler (`handleWebhook`) only accepts POST, so /status (GET) on a different path won't interfere.

## Files
- `governor/internal/webhooks/server.go` — Add startTime/version fields, Config.Version, handleStatus handler, register /status route in Start()
- `governor/cmd/governor/main.go` — Pass `version` variable to webhooks.Config
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": [
    "governor/internal/webhooks/server.go",
    "governor/cmd/governor/main.go"
  ],
  "tests_required": [],
  "verification": "curl http://localhost:8080/status returns {\"governor\": \"vibepilot\", \"version\": \"2.0.0\", \"status\": \"running\", \"uptime_seconds\": <int>} with HTTP 200"
}
```