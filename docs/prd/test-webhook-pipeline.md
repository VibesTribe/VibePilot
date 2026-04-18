# Test: Hello World Package

## Summary
Create a simple Go package `internal/hello` with a `Greet(name string) string` function that returns "Hello, {name}!".

## Requirements
- Package path: `internal/hello`
- Single function: `Greet(name string) string`
- Returns "Hello, {name}!" where {name} is the input
- Include a basic test file

## Acceptance Criteria
- `go build ./internal/hello` passes
- `go test ./internal/hello` passes
- Function handles empty string (returns "Hello, !")
