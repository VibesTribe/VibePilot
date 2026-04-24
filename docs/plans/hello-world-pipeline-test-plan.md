# PLAN: Hello World Pipeline Test

## Overview
Add a simple hello function to the VibePilot governor that demonstrates the full pipeline from PRD to merged code.

## Tasks

### T001: Add Hello Function to Governor
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add Hello Function to Governor

## Context
This task is to implement the "Hello Handler" requirement (FR-001) for the "Hello World Pipeline Test" PRD. The goal is to add a simple `hello()` function to the main governor executable that logs a greeting message upon startup. This demonstrates a basic feature addition within the VibePilot governor.

## What to Build
1.  **Create `governor/hello.go`**: Create a new file named `hello.go` inside the `governor/internal/hello` directory.
2.  **Implement `hello()` function**: In `governor/internal/hello/hello.go`, create a `Greet(name string) string` function that returns the string "Hello from VibePilot!".
3.  **Modify `governor/cmd/governor/main.go`**: Add a call to the new `hello()` function after the existing "Governor started" log message. The new log message should be "Hello from VibePilot!".
4.  **Add a test file**: Create `governor/cmd/governor/hello_test.go` with a single test that verifies the `hello.Greet()` function returns the expected string "Hello from VibePilot!".

## Files
- `governor/internal/hello/hello.go` - Add the `Greet` function.
- `governor/cmd/governor/main.go` - Add a log line calling `hello.Greet()` after the "Governor started" message.
- `governor/cmd/governor/hello_test.go` - Add a unit test for the `hello.Greet()` function.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "governor/internal/hello/hello.go",
    "governor/cmd/governor/hello_test.go"
  ],
  "files_modified": [
    "governor/cmd/governor/main.go"
  ],
  "tests_written": [
    "governor/cmd/governor/hello_test.go"
  ]
}
```
