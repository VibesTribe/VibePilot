# PLAN: VibePilot Ping Endpoint

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
Add a new HTTP handler at `GET /api/ping` to the VibePilot API.

## What to Build
Create a new HTTP handler that returns `{"status":"ok","timestamp":"\u003cISO8601\u003e"}`.

## Files
- `internal/api/server.go` - Add the new route to the API server setup
- `internal/api/handlers/ping.go` - Create a new file for the ping handler

#### Expected Output
{
  "files_created": ["internal/api/handlers/ping.go"],
  "tests_written": []
}

### T002: Write Ping Test
**Confidence:** 0.96
**Category:** testing
**Dependencies:** ["T001"]

#### Prompt Packet
# TASK: T002 - Write Ping Test

## Context
Write a Go test file that makes an HTTP request to `/api/ping` and verifies the response.

## What to Build
Create a new test file that verifies the `/api/ping` endpoint returns `{"status":"ok","timestamp":"\u003cISO8601\u003e"}`.

## Files
- `internal/api/tests/ping_test.go` - Create a new test file

#### Expected Output
{
  "files_created": ["internal/api/tests/ping_test.go"],
  "tests_written": ["internal/api/tests/ping_test.go"]
}

