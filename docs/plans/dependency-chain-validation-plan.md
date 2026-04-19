# PLAN: Dependency Chain Validation

## Overview
Add a GET /api/ping endpoint to the VibePilot webhook server that returns JSON with status and timestamp, then write a test verifying the response. This two-task plan validates that task dependencies work correctly in the governor pipeline.

## Slice
general

## Tasks

### T001: Add /api/ping HTTP Handler
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add /api/ping HTTP Handler

## Context
The VibePilot webhook server (governor/internal/webhooks/server.go) already serves routes on an HTTP mux (/webhooks and /status). We need a simple health-check endpoint at GET /api/ping that returns a JSON response. This handler will be used by downstream tests to validate the dependency chain feature of the task pipeline.

## What to Build

1. Add a new handler method `handlePing` on the `Server` struct in `governor/internal/webhooks/server.go`.

2. The handler must:
   - Only accept GET requests. Return 405 for any other method.
   - Return HTTP 200 with Content-Type application/json.
   - Response body must be: {"status":"ok","timestamp":"<ISO8601>"} where <ISO8601> is the current UTC time formatted as time.RFC3339 (e.g. "2026-04-19T15:30:00Z").

3. Register the route in the `Start` method of `Server` by adding:
   `mux.HandleFunc("/api/ping", s.handlePing)`
   Place it right after the existing `mux.HandleFunc("/status", s.handleStatus)` line.

## Files
- `governor/internal/webhooks/server.go` - Add handlePing method and register route in Start()

## Code Reference
The existing handleStatus method (line 296-312) is the exact pattern to follow:
- Check method is GET, return 405 otherwise
- Build a map[string]any response
- Set Content-Type to application/json
- Use json.NewEncoder(w).Encode(resp)

For the ping handler, use `time.Now().UTC().Format(time.RFC3339)` for the timestamp value.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/internal/webhooks/server.go"],
  "tests_written": []
}
```

---

### T002: Write Test for /api/ping Handler
**Confidence:** 0.97
**Category:** coding
**Dependencies:** ["T001"]

#### Prompt Packet
```
# TASK: T002 - Write Test for /api/ping Handler

## Context
T001 added a handlePing method to the webhook Server in governor/internal/webhooks/server.go. This task writes comprehensive tests for that handler, following the exact same testing patterns used in the existing server_test.go file.

## What to Build

Add tests to `governor/internal/webhooks/server_test.go` (the file already exists with TestHandleStatus_* tests). Append the following test functions:

1. `TestHandlePing_Get` - Tests the happy path:
   - Create a Server with NewServer using a Config{Port:0, Path:"/webhooks", Secret:"", Version:"test"}
   - Create a GET request to "/api/ping" using httptest.NewRequest
   - Call s.handlePing(w, req) directly
   - Assert status code is 200
   - Assert Content-Type is "application/json"
   - Parse response body as JSON
   - Assert body["status"] == "ok"
   - Assert body["timestamp"] is a non-empty string
   - Assert body["timestamp"] can be parsed with time.Parse(time.RFC3339, ...)

2. `TestHandlePing_NonGetReturns405` - Tests method rejection:
   - Try POST, PUT, DELETE, PATCH on "/api/ping"
   - Assert each returns 405

3. `TestHandlePing_TimestampIsCurrent` - Tests timestamp freshness:
   - Record time.Now().UTC() before the call
   - Call handlePing
   - Parse the timestamp from response
   - Assert the response timestamp is within 2 seconds of the recorded before-time

Follow the exact code style from the existing tests:
- Use `httptest.NewRequest` and `httptest.NewRecorder`
- Use `t.Fatalf` for setup errors, `t.Errorf` for assertion failures
- Same struct literal style for Config
- Package is `webhooks` (same package, no import needed for Server)

## Files
- `governor/internal/webhooks/server_test.go` - Append test functions to existing file
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_modified": ["governor/internal/webhooks/server_test.go"],
  "tests_written": ["governor/internal/webhooks/server_test.go"]
}
```

---
