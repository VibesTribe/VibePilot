# PRD: Hello World Endpoint

## Summary
Add a `/hello` GET endpoint to the governor HTTP server that returns `{"message": "Hello from VibePilot"}`.

## Technical Specification

### Requirements
1. Add a single HTTP handler function in `cmd/governor/` that responds to GET `/hello`
2. Returns JSON: `{"message": "Hello from VibePilot"}`
3. Content-Type header: `application/json`
4. Register the route in the existing HTTP router wherever other routes are registered
5. No new dependencies. Use only the standard library (`net/http`, `encoding/json`)

### Constraints
- One file change for the handler, one file change for route registration
- No CLI framework changes
- No new packages
- Maximum 30 lines of new code

### Acceptance Criteria
- `curl http://localhost:8080/hello` returns `{"message":"Hello from VibePilot"}`
- Server starts without errors
- Existing routes still work

### Files to modify
- `cmd/governor/main.go` or wherever routes are registered — add the `/hello` route
- Optionally a new `cmd/governor/handlers_hello.go` for the handler function

### Confidence Target
This is a single-route addition with no dependencies. Expected confidence: 98%+. Expected completion: under 60 seconds.
