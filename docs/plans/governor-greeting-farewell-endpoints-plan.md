# PLAN: Governor Greeting and Farewell Endpoints
## Overview
Add two simple HTTP endpoints to the governor: a `/greeting` endpoint that returns a configurable greeting message, and a `/farewell` endpoint that returns a farewell message using the greeting module's response format.
## Tasks
### T001: Create Shared Response Helper
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none
#### Prompt Packet
# TASK: T001 - Create Shared Response Helper
## Context
The governor needs a shared helper function to generate JSON responses for both the `/greeting` and `/farewell` endpoints.
## What to Build
Create a new file `helpers/response.go` with a function `GenerateResponse(message string) (string, error)` that returns a JSON string with the given message and a timestamp.
## Files
- `helpers/response.go` - New file for the shared response helper function
#### Expected Output
{
  "task_id": "T001",
  "files_created": ["helpers/response.go"],
  "tests_written": []
}
### T002: Implement Greeting Endpoint
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001
#### Prompt Packet
# TASK: T002 - Implement Greeting Endpoint
## Context
The `/greeting` endpoint should return a JSON response with a greeting message and a timestamp.
## What to Build
Create a new file `handlers/greeting.go` with a handler function `GreetingHandler(w http.ResponseWriter, r *http.Request)` that calls the shared response helper function to generate the response.
## Files
- `handlers/greeting.go` - New file for the greeting endpoint handler
#### Expected Output
{
  "task_id": "T002",
  "files_created": ["handlers/greeting.go"],
  "tests_written": []
}
### T003: Implement Farewell Endpoint
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001
#### Prompt Packet
# TASK: T003 - Implement Farewell Endpoint
## Context
The `/farewell` endpoint should return a JSON response with a farewell message and a timestamp.
## What to Build
Create a new file `handlers/farewell.go` with a handler function `FarewellHandler(w http.ResponseWriter, r *http.Request)` that calls the shared response helper function to generate the response.
## Files
- `handlers/farewell.go` - New file for the farewell endpoint handler
#### Expected Output
{
  "task_id": "T003",
  "files_created": ["handlers/farewell.go"],
  "tests_written": []
}
### T004: Register Endpoints
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T002, T003
#### Prompt Packet
# TASK: T004 - Register Endpoints
## Context
The `/greeting` and `/farewell` endpoints need to be registered in the existing HTTP router.
## What to Build
Modify the existing route registration file to add handlers for the `/greeting` and `/farewell` endpoints.
## Files
- `main.go` - Modify the existing route registration file
#### Expected Output
{
  "task_id": "T004",
  "files_modified": ["main.go"],
  "tests_written": []
}
