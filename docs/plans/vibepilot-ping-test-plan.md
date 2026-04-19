# PLAN: VibePilot Ping Test

## Overview
Add a simple `/ping` endpoint to the VibePilot API and a test to verify it works.

## Tasks

### T001: Create Ping Endpoint
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
# TASK: T001 - Create Ping Endpoint

## Context
The goal is to add a simple `/ping` endpoint to the VibePilot API that returns a JSON response with status and timestamp.

## What to Build
Create a new HTTP handler at `GET /api/ping` that returns `{\"status\":\"ok\",\"timestamp\":\"\u003cISO8601\u003e\"}`. Add the route to the existing API server setup.

## Files
- `vibepilot/api/server.go` - Add the `/api/ping` handler
- `vibepilot/api/routes.go` - Add the route to the API server setup

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["vibepilot/api/server.go", "vibepilot/api/routes.go"],
  "tests_written": []
}

### T002: Write Ping Test
**Confidence:** 0.96
**Category:** testing
**Dependencies:** [\"T001\"]

#### Prompt Packet
# TASK: T002 - Write Ping Test

## Context
The `/ping` endpoint must be tested to ensure it works as expected.

## What to Build
Write a Go test file that makes an HTTP request to `/api/ping` and verifies the response has status \"ok\" and a valid timestamp.

## Files
- `vibepilot/tests/ping_test.go` - Test file for the `/api/ping` endpoint

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["vibepilot/tests/ping_test.go"],
  "tests_written": ["vibepilot/tests/ping_test.go"]
}

