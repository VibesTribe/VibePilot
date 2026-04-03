# PLAN: Hello World

## Overview
Create a simple hello world function in Go to test the VibePilot end-to-end flow.

## Tasks

### T001: Create Hello Package
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
# TASK: T001 - Create Hello Package

## Context
Create the foundational hello package to demonstrate the VibePilot development workflow with a simple, well-defined Go package.

## What to Build
Create a Go package at `governor/cmd/hello/` with:
- Package name: `hello`
- Function `Hello() string` that returns "Hello, VibePilot!"

The function should be exported (capital H) and return a constant string.

## Files
- `governor/cmd/hello/hello.go` - Package implementation

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/hello/hello.go"],
  "tests_written": []
}
```

### T002: Add Unit Tests
**Confidence:** 0.99
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
# TASK: T002 - Add Unit Tests

## Context
Verify the Hello function works correctly with a unit test that confirms it returns the expected string.

## What to Build
Create a test file at `governor/cmd/hello/hello_test.go` with:
- Test function `TestHello` that calls `Hello()` and asserts it equals "Hello, VibePilot!"
- Use standard Go testing package

The test should pass when `go test` is run in the hello package directory.

## Files
- `governor/cmd/hello/hello_test.go` - Test implementation

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/cmd/hello/hello_test.go"],
  "tests_written": ["TestHello"]
}
```