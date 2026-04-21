# PRD: Greeting and Farewell Endpoints

## Summary
Add two simple HTTP endpoints to the governor: a `/greeting` endpoint that returns a configurable greeting message, and a `/farewell` endpoint that returns a farewell message using the greeting module's response format.

## Technical Specification

### Endpoint 1: GET /greeting
1. Add an HTTP handler that responds to GET `/greeting`
2. Returns JSON: `{"message": "Hello from VibePilot", "timestamp": "<current UTC ISO 8601>"}`
3. Content-Type: `application/json`
4. Register route in the existing HTTP router

### Endpoint 2: GET /farewell
1. Add an HTTP handler that responds to GET `/farewell`
2. Returns JSON: `{"message": "Goodbye from VibePilot", "timestamp": "<current UTC ISO 8601>"}`
3. Must use the same response format and helper function as the `/greeting` endpoint (shared utility, no duplication)
4. Register route in the existing HTTP router

### Constraints
- Standard library only (`net/http`, `encoding/json`, `time`)
- No new external dependencies
- Shared response helper between both endpoints
- Maximum 40 lines of new code across all files
- Each endpoint is a single handler function
- Both routes registered in the same place as existing routes

### Acceptance Criteria
- `curl http://localhost:8080/greeting` returns valid JSON with "Hello from VibePilot" and a timestamp
- `curl http://localhost:8080/farewell` returns valid JSON with "Goodbye from VibePilot" and a timestamp
- Both responses have Content-Type `application/json`
- Server starts without errors
- Existing routes still work

### Files to modify
- Route registration file (wherever existing routes are defined)
- New file for handlers or extend existing handlers file

### Confidence Target
Two simple endpoints sharing a helper. Expected confidence: 95%+. No external dependencies.
