# PLAN: Governor Greeting and Farewell Endpoints

## Overview
Add two simple HTTP endpoints to the governor: a `/greeting` endpoint that returns a configurable greeting message, and a `/farewell` endpoint that returns a farewell message using the greeting module's response format.

## Tasks

### T001: Create Greeting Endpoint Handler
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
# TASK: T001 - Create Greeting Endpoint Handler
## Context
The governor needs a new HTTP handler that responds to GET `/greeting` with a JSON message containing a greeting and a timestamp.
## What to Build
Create a new handler function that returns `{"message": "Hello from VibePilot", "timestamp": "<current UTC ISO 8601>"}` in JSON format with Content-Type `application/json`.
## Files
- `handlers/http_handlers.go` - New handler function for `/greeting` endpoint

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["handlers/http_handlers.go"],
  "tests_written": []
}

### T002: Create Farewell Endpoint Handler
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
# TASK: T002 - Create Farewell Endpoint Handler
## Context
The governor needs a new HTTP handler that responds to GET `/farewell` with a JSON message containing a farewell and a timestamp, using the same response format as the `/greeting` endpoint.
## What to Build
Create a new handler function that returns `{"message": "Goodbye from VibePilot", "timestamp": "<current UTC ISO 8601>"}` in JSON format with Content-Type `application/json`, sharing the response helper with the `/greeting` endpoint.
## Files
- `handlers/http_handlers.go` - New handler function for `/farewell` endpoint

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": []
}

### T003: Register Endpoints with HTTP Router
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001, T002

#### Prompt Packet
# TASK: T003 - Register Endpoints with HTTP Router
## Context
The `/greeting` and `/farewell` endpoints need to be registered with the existing HTTP router.
## What to Build
Modify the route registration file to include the new `/greeting` and `/farewell` endpoints.
## Files
- `routes/router.go` - Register new endpoints

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "tests_written": []
}

### T004: Write Unit Tests for Endpoints
**Confidence:** 0.96
**Category:** testing
**Dependencies:** T001, T002, T003

#### Prompt Packet
# TASK: T004 - Write Unit Tests for Endpoints
## Context
Unit tests are needed to verify the correctness of the `/greeting` and `/farewell` endpoints.
## What to Build
Write unit tests for the new endpoints using the `net/http/httptest` package.
## Files
- `tests/http_handlers_test.go` - Unit tests for `/greeting` and `/farewell` endpoints

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["tests/http_handlers_test.go"],
  "tests_written": ["TestGreetingEndpoint", "TestFarewellEndpoint"]
}

