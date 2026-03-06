# PRD: Test Hello Feature

## Summary
Add a simple hello world test to verify the planner agent workflow.

## Requirements
- Create a simple test file that outputs "Hello, VibePilot!"
- The test should be runnable with `go test`
- No external dependencies

## Acceptance Criteria
- Test file exists at `internal/tests/hello_test.go`
- Test passes when run with `go test ./internal/tests/`
- Test outputs "Hello, VibePilot!" to confirm the system is working
