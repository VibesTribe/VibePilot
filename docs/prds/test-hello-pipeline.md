# PRD: Pipeline Smoke Test - Hello Function

## Summary
Minimal smoke test to verify the full VibePilot pipeline works end-to-end: planner → executor → tester → reviewer → merger. No external dependencies, no complex logic.

## Requirements
Create a single Go file `governor/internal/hello/hello.go` with:
- A package `hello`
- A function `Greet(name string) string` that returns `"Hello, " + name + "!"`
- A test file `governor/internal/hello/hello_test.go` that tests:
  - `Greet("World")` returns `"Hello, World!"`
  - `Greet("")` returns `"Hello, !"`

## Constraints
- Only create files under `governor/internal/hello/`
- No changes to any existing files
- No external dependencies
- Must pass `go test ./...`

## Acceptance Criteria
- Both files exist in `governor/internal/hello/`
- `go test ./internal/hello/...` passes
- `go build ./...` passes
- No modifications to existing code
