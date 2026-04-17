# PRD: Add /status endpoint to webhook server

## Summary
Add a GET /status endpoint to the webhook server that returns basic governor runtime info as JSON. This is for operational monitoring and debugging.

## Requirements
- Add GET /status route to the webhook server (`governor/internal/webhooks/server.go`)
- Return JSON with the following fields:
  - `governor`: string, always "vibepilot"
  - `version`: string, from the Version variable set at build time
  - `status`: string, "running"
  - `uptime_seconds`: integer, seconds since the server started
- HTTP status code 200
- The server should track its own start time to calculate uptime
- No external dependencies

## Constraints
- Modify only `governor/internal/webhooks/server.go`
- Keep it simple — no database queries, no external calls
- Must not break existing webhook handling on the same mux

## Acceptance Criteria
- `curl http://localhost:8080/status` returns JSON with all four fields
- Response is valid JSON with status 200
- Existing webhook endpoint continues to work
