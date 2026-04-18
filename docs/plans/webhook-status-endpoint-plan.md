# PLAN: Add /status Endpoint to Webhook Server

## Overview
Add a GET /status endpoint to the webhook server that returns governor runtime info as JSON for operational monitoring. Single-file change to `server.go` plus a test file.

## Tasks

### T001: Add /status endpoint to webhook server
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /status endpoint to webhook server

## Context
The webhook server currently only handles POST requests on its webhook path. Operators need a simple health/runtime endpoint to confirm the governor is alive and see basic info without hitting Supabase or checking logs.

## What to Build

Modify `governor/internal/webhooks/server.go` to add a GET /status endpoint. Changes required:

### 1. Add fields to Server struct

Add two new fields to the `Server` struct:
- `version string` — the governor version string
- `startTime time.Time` — set when Start() is called

The `startTime` field already has `time` imported. No new imports needed.

### 2. Add Version to Config struct

Add `Version string` to the `Config` struct so callers can pass in the build-time version.

### 3. Store version in NewServer

In `NewServer`, add:
```go
version: cfg.Version,
```

to the returned Server literal. The `startTime` will be set in Start().

### 4. Set startTime in Start()

At the top of the `Start()` method, before creating the mux, add:
```go
s.startTime = time.Now()
```

### 5. Register /status route on the mux

In `Start()`, after `mux.HandleFunc(s.path, s.handleWebhook)`, add:
```go
mux.HandleFunc("/status", s.handleStatus)
```

### 6. Add handleStatus method

Add a new method on Server:
```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Seconds()
	resp := struct {
		Governor      string  `json:"governor"`
		Version       string  `json:"version"`
		Status        string  `json:"status"`
		UptimeSeconds float64 `json:"uptime_seconds"`
	}{
		Governor:      "vibepilot",
		Version:       s.version,
		Status:        "running",
		UptimeSeconds: uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
```

Notes:
- Use `float64` for uptime_seconds (time.Since().Seconds() returns float64). If the PRD strictly wants an integer, use `int64(uptime)` — but float is more useful and standard. The PRD says "integer" so use `int64(time.Since(s.startTime).Seconds())` with `json:"uptime_seconds"` typed as `int64`.

### 7. Add test file

Create `governor/internal/webhooks/server_test.go` with:

```go
package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatusEndpoint(t *testing.T) {
	cfg := &Config{
		Port:    0, // will default to 8080
		Path:    "/webhooks",
		Secret:  "",
		Version: "test-1.0.0",
	}
	srv := NewServer(cfg, nil)
	srv.startTime = time.Now()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body struct {
		Governor      string `json:"governor"`
		Version       string `json:"version"`
		Status        string `json:"status"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Governor != "vibepilot" {
		t.Errorf("expected governor=vibepilot, got %s", body.Governor)
	}
	if body.Version != "test-1.0.0" {
		t.Errorf("expected version=test-1.0.0, got %s", body.Version)
	}
	if body.Status != "running" {
		t.Errorf("expected status=running, got %s", body.Status)
	}
	if body.UptimeSeconds < 0 {
		t.Errorf("expected non-negative uptime, got %d", body.UptimeSeconds)
	}
}

func TestStatusMethodNotAllowed(t *testing.T) {
	cfg := &Config{Version: "1.0.0"}
	srv := NewServer(cfg, nil)
	srv.startTime = time.Now()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/status", nil)
		w := httptest.NewRecorder()
		srv.handleStatus(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /status: expected 405, got %d", method, w.Code)
		}
	}
}
```

## Files
- `governor/internal/webhooks/server.go` — Add startTime+version fields, Config.Version, /status route, handleStatus handler
- `governor/internal/webhooks/server_test.go` — NEW: tests for /status endpoint
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/internal/webhooks/server_test.go"],
  "files_modified": ["governor/internal/webhooks/server.go"],
  "tests_written": ["governor/internal/webhooks/server_test.go"],
  "acceptance": "curl http://localhost:8080/status returns {\"governor\":\"vibepilot\",\"version\":\"...\",\"status\":\"running\",\"uptime_seconds\":N} with HTTP 200"
}
```
