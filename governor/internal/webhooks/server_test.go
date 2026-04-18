package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleStatus_Get(t *testing.T) {
	cfg := &Config{
		Port:    0,
		Path:    "/webhooks",
		Secret:  "",
		Version: "0.9.0",
	}
	s := NewServer(cfg, nil)
	s.startTime = time.Now().Add(-5 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if gov, _ := body["governor"].(string); gov != "vibepilot" {
		t.Errorf("expected governor=vibepilot, got %v", body["governor"])
	}
	if ver, _ := body["version"].(string); ver != "0.9.0" {
		t.Errorf("expected version=0.9.0, got %v", body["version"])
	}
	if st, _ := body["status"].(string); st != "running" {
		t.Errorf("expected status=running, got %v", body["status"])
	}
	uptime, ok := body["uptime_seconds"].(float64)
	if !ok {
		t.Errorf("expected uptime_seconds to be a number, got %v", body["uptime_seconds"])
	}
	if uptime < 1 {
		t.Errorf("expected uptime_seconds >= 1, got %v", uptime)
	}
}

func TestHandleStatus_NonGetReturns405(t *testing.T) {
	cfg := &Config{
		Port:    0,
		Path:    "/webhooks",
		Secret:  "",
		Version: "0.9.0",
	}
	s := NewServer(cfg, nil)
	s.startTime = time.Now()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/status", nil)
		w := httptest.NewRecorder()
		s.handleStatus(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: expected 405, got %d", method, w.Code)
		}
	}
}

func TestHandleStatus_UptimePositive(t *testing.T) {
	cfg := &Config{
		Port:    0,
		Path:    "/webhooks",
		Secret:  "",
		Version: "0.9.0",
	}
	s := NewServer(cfg, nil)
	s.startTime = time.Now().Add(-10 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	uptime, ok := body["uptime_seconds"].(float64)
	if !ok {
		t.Fatalf("uptime_seconds is not a number: %v", body["uptime_seconds"])
	}
	if uptime <= 0 {
		t.Errorf("expected positive uptime_seconds, got %v", uptime)
	}
}
