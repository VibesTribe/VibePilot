# PRD: Add /health endpoint

## Summary
Add a simple health check endpoint to the webhook server.

## Requirements
- Add GET /health to `governor/internal/webhooks/server.go`
- Return JSON: `{"status": "ok", "timestamp": "<unix timestamp>"}`
- Status code 200

## Acceptance Criteria
- `curl http://localhost:8080/health` returns `{"status": "ok", "timestamp": "..."}`
- No external dependencies
