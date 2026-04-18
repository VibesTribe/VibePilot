# PLAN: Add /status Endpoint to Webhook Server

## Overview
Add a GET /status route to the webhook server that returns governor runtime info as JSON. Requires three changes: (1) add startTime and version fields to the Server struct, (2) register the /status route on the mux alongside the existing webhook route, (3) wire the version string from main.go into the server constructor.

## Tasks

### T001: Add /status Endpoint to Webhook Server
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /status Endpoint to Webhook Server

## Context
The webhook server needs a health/status endpoint for operational monitoring. Currently it only handles POST requests on the webhook path. We need to add a GET /status route that returns runtime information without disrupting existing webhook handling.

## Current Architecture
- Server struct is in `governor/internal/webhooks/server.go`
- Server has fields: port, path, secret, router, github, server, handlers
- Server is constructed in `governor/cmd/governor/main.go` at line 214 via `webhooks.NewServer()`
- The version variable `version` (lowercase, unexported) lives in `main.go` line 28, set to "2.0.0" as default, overridden at build time via ldflags
- The mux is created in `Start()` method (line 74) — currently only registers `s.path` (webhook handler)
- The `time` package is already imported in server.go

## What to Build

### 1. Add fields to Server struct (in `governor/internal/webhooks/server.go`)
Add two new fields:
- `startTime time.Time` — records when the server started
- `version string` — the governor version string, passed in at construction time

### 2. Update Config struct
Add a `Version string` field to the `Config` struct so main.go can pass the version in.

### 3. Update NewServer constructor
In `NewServer()`, store `cfg.Version` into `s.version` and set a default of `"unknown"` if empty. Do NOT set startTime here — set it in Start().

### 4. Add handleStatus method
Add a new method `handleStatus(w http.ResponseWriter, r *http.Response) *http.Request` with signature:
```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
```
It should:
- Only accept GET, reject anything else with 405
- Set `Content-Type: application/json`
- Return status 200 with JSON body:
```json
{
  "governor": "vibepilot",
  "version": "<the version string>",
  "status": "running",
  "uptime_seconds": <int64, calculated as time.Since(s.startTime).Seconds() rounded>
}
```
Use `json.NewEncoder(w).Encode()` for the response. Define a local struct for marshaling.

### 5. Register the route in Start()
In the `Start()` method, after `mux.HandleFunc(s.path, s.handleWebhook)` (line 75), add:
- Set `s.startTime = time.Now()` before creating the mux
- Add `mux.HandleFunc("/status", s.handleStatus)` after the existing webhook route registration

### 6. Wire version in main.go
In `governor/cmd/governor/main.go`, update the `NewServer` call (around line 214) to include:
```go
Version: version,
```
This passes the build-time version variable (from ldflags) into the webhook server.

## Files to Modify
- `governor/internal/webhooks/server.go` — Add startTime/version fields, Config.Version field, NewServer update, handleStatus method, route registration in Start()
- `governor/cmd/governor/main.go` — Pass version string to NewServer Config

## Important Constraints
- Do NOT change the existing webhook handler behavior at all
- Do NOT add any external dependencies
- The /status endpoint must NOT require authentication (it's for health checks)
- uptime_seconds must be an integer (use int64, truncate/round from float64)
- Keep the /status handler simple — no database queries, no external calls
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
  "acceptance_criteria": [
    "curl http://localhost:8080/status returns JSON with governor, version, status, uptime_seconds fields",
    "Response HTTP status is 200",
    "Content-Type is application/json",
    "uptime_seconds is a positive integer",
    "Existing POST /webhooks endpoint still works unchanged"
  ]
}
```

---

### T002: Add Unit Tests for /status Endpoint
**Confidence:** 0.97
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Add Unit Tests for /status Endpoint

## Context
T001 adds a GET /status endpoint to the webhook server. We need tests to verify it returns correct JSON and doesn't break existing webhook handling.

## What to Build
Create `governor/internal/webhooks/server_test.go` with the following test cases:

### Test 1: TestStatusEndpoint_ReturnsCorrectJSON
- Create a Server with `Config{Port: 0, Version: "test-1.0.0"}`
- Set `s.startTime` to a known time (e.g., 5 seconds ago)
- Use `httptest.NewRequest("GET", "/status", nil)` and `httptest.NewRecorder()`
- Call `s.handleStatus(recorder, request)`
- Assert status 200
- Assert Content-Type contains "application/json"
- Parse JSON response body, verify:
  - `governor` == "vibepilot"
  - `version` == "test-1.0.0"
  - `status` == "running"
  - `uptime_seconds` >= 5 (approximately)

### Test 2: TestStatusEndpoint_RejectsNonGET
- Send a POST request to handleStatus
- Assert status 405

### Test 3: TestStatusEndpoint_DefaultVersion
- Create Server with empty Version in Config
- Verify response `version` field is "unknown"

## Files
- `governor/internal/webhooks/server_test.go` — New file with all tests
- Uses standard library: `net/http/httptest`, `encoding/json`, `testing`, `time`
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [
    "governor/internal/webhooks/server_test.go"
  ],
  "tests_written": [
    "TestStatusEndpoint_ReturnsCorrectJSON",
    "TestStatusEndpoint_RejectsNonGET",
    "TestStatusEndpoint_DefaultVersion"
  ]
}
```

---

## Summary
- **T001**: Core implementation (struct changes + handler + route + wiring) — Confidence 0.98
- **T002**: Unit tests for the new endpoint — Confidence 0.97
- Total: 2 tasks, 0 external dependencies, modifies only 3 files
