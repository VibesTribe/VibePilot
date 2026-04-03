# PLAN: Hello World

## Overview
Create a simple hello world function in Go to test the VibePilot end-to-end flow.

## Tasks

### T001: Create Hello Package with Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Package with Tests

## Context
This task tests the VibePilot end-to-end flow by creating a simple Go package with a function and tests.

## What to Build
1. Create a new Go package at `governor/cmd/hello/`
2. Create `hello.go` with a function `Hello()` that returns the string "Hello, VibePilot!"
3. Create `hello_test.go` with a test function `TestHello()` that verifies `Hello()` returns the correct string

The function signature should be:
```go
package hello

func Hello() string {
    return "Hello, VibePilot!"
}
```

The test should use standard Go testing:
```go
package hello

import "testing"

func TestHello(t *testing.T) {
    got := Hello()
    want := "Hello, VibePilot!"
    if got != want {
        t.Errorf("Hello() = %q, want %q", got, want)
    }
}
```

## Files
- `governor/cmd/hello/hello.go` - Main package file with Hello() function
- `governor/cmd/hello/hello_test.go` - Test file for Hello() function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/hello/hello.go", "governor/cmd/hello/hello_test.go"],
  "tests_written": ["governor/cmd/hello/hello_test.go"]
}
```
