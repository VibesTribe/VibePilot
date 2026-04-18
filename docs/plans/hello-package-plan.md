# PLAN: Hello Package

## Overview
Create a Go package `internal/hello` with a Greet function and tests.

## Tasks

### T001: Implement hello package with Greet function and tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
# TASK: T001 - Implement hello package with Greet function and tests

## Context
A new internal package is needed that provides a simple greeting function. This is a foundational package that other packages may import.

## What to Build
Create the following files:

1. `internal/hello/hello.go` - Package hello with function `Greet(name string) string`. If name is empty string, return `"Hello, !"` (no special handling required — just concatenate). The function must return `"Hello, " + name + "!"`.

2. `internal/hello/hello_test.go` - Test file containing:
   - `TestGreet` with a subtest for normal input: `Greet("World")` must return `"Hello, World!"`
   - `TestGreetEmpty` or a subtest for empty string: `Greet("")` must return `"Hello, !"`

## Constraints
- Package must be in directory `internal/hello`
- Go package name: `hello`
- Function signature: `func Greet(name string) string`
- Must pass `go test ./internal/hello/...` with no errors
- No external dependencies

## Files
- `internal/hello/hello.go` - Greet function implementation
- `internal/hello/hello_test.go` - Test cases

#### Expected Output
{"task_id":"T001","files_created":["internal/hello/hello.go","internal/hello/hello_test.go"],"tests_written":["internal/hello/hello_test.go"],"verify_command":"go test ./internal/hello/..."}