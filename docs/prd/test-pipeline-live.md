# Test: Hello Package

## Summary
Create a Go package `internal/hello` with a single function.

## Requirements
- Package: `internal/hello`
- Function: `Greet(name string) string` returns "Hello, {name}!"
- Test file: `internal/hello/hello_test.go` with one test case

## Acceptance Criteria
- `go test ./internal/hello` passes
- Handles empty string input
