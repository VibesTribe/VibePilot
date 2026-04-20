# PLAN: VibePilot Dependency Chain Validation

## Overview
Add a /ping endpoint and validation test with enforced task dependencies.

## Tasks

### T001: Create Ping Endpoint
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Ping Endpoint

## Context
Build a health-check endpoint for the VibePilot API to confirm system availability.

## What to Build
1. Add a GET handler at `/api/ping` returning `{"status":"ok","timestamp":"[ISO8601]"}`.
2. Include the route in the existing API server initialization.

## Files
- `api/handlers/ping.go` - New handler implementation
- `api/server/router.go` - Register the `/api/ping` route
```

#### Expected Output
```json
{
  "files_created": ["api/handlers/ping.go", "api/server/router.go"],
  "tests_required": []
}
```

### T002: Write Ping Test
**Confidence:** 0.95
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Write Ping Test

## Context
Validate the `/api/ping` endpoint returns the correct response structure.

## What to Build
1. Create a test that sends a GET request to `/api/ping`.
2. Assert the response status is "ok" and timestamp matches ISO8601 format.

## Files
- `api/handlers/ping_test.go` - Test implementation
```

#### Expected Output
```json
{
  "files_created": ["api/handlers/ping_test.go"],
  "tests_required": ["api/handlers/ping_test.go"]
}
```
