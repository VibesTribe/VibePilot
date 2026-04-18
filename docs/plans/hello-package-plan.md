# PLAN: Hello Package

## Overview
Create a Go package `internal/hello` with a `Greet` function that returns a greeting string, plus a passing test file.

## Tasks

### T001: Implement internal/hello package with Greet function and tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Implement internal/hello package with Greet function and tests

## Context
A new Go package is needed under `internal/hello` providing a single exported function `Greet`. This is a self-contained package with no external dependencies.

## What to Build
1. Create directory `internal/hello/` if it does not exist.
2. Create `internal/hello/hello.go`:
   - Package declaration: `package hello`
   - Exported function: `func Greet(name string) string`
   - If `name` is an empty string, return `"Hello, !"` (no special casing needed — the format string handles it naturally)
   - Otherwise return `"Hello, {name}!"` using `fmt.Sprintf`
3. Create `internal/hello/hello_test.go`:
   - Package: `package hello` (white-box test)
   - Import `testing`
   - Test function `TestGreet` containing:
     - A case where name is `"World"` asserting the result equals `"Hello, World!"`
     - A case where name is empty string asserting the result equals `"Hello, !"`
   - Use `t.Errorf` for assertion failures with clear messages showing expected vs actual.

## Files
- `internal/hello/hello.go` - Package implementation with Greet function
- `internal/hello/hello_test.go` - Test file with TestGreet
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "internal/hello/hello.go",
    "internal/hello/hello_test.go"
  ],
  "tests_written": [
    "internal/hello/hello_test.go"
  ]
}
```
