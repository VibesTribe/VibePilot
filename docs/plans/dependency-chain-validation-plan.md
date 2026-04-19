# PLAN: Dependency Chain Validation

## Overview
Validate task dependency handling by creating a `/ping` endpoint and corresponding test.

## Tasks

### T001: Create /api/ping Endpoint
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create /api/ping Endpoint

## Context
Implement a health-check endpoint to verify API responsiveness.

## What to Build
1. Add a new HTTP handler for `GET /api/ping` in `internal/handlers/ping.go`.
2. The handler should return JSON: `{"status":"ok","timestamp":"<ISO8601>"}`.
3. Register the route in `cmd/api/server.go`.

## Files
- `internal/handlers/ping.go` - New handler implementation
- `cmd/api/server.go` - Route registration
```

#### Expected Output
```json
{
  "files_created": ["internal/handlers/ping.go", "cmd/api/server.go"],
  "tests_required": []
}
```

### T002: Write /api/ping Test
**Confidence:** 0.98
**Category:** coding
**Dependencies:** ["T001"]

#### Prompt Packet
```
# TASK: T002 - Write /api/ping Test

## Context
Verify the `/api/ping` endpoint returns correct data before deployment.

## What to Build
1. Create a test in `internal/handlers/ping_test.go`.
2. Use `net/http/httptest` to mock a request to `/api/ping`.
3. Assert response status is 200, JSON contains "ok" and ISO8601 timestamp.

## Files
- `internal/handlers/ping_test.go` - Test implementation
```

#### Expected Output
```json
{
  "files_created": ["internal/handlers/ping_test.go"],
  "tests_required": ["internal/handlers/ping_test.go"]
}
```

## Dependencies
- T002 requires T001 completion

