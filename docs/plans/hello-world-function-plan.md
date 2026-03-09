# PLAN: Hello World Function

## Overview
Create a simple Hello World function to test the VibePilot flow end-to-end.

## Tasks

### T001: Create Hello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Function

## Context
Create a simple Go function to test the VibePilot flow end-to-end.

## What to Build
Create a Go function `SayHello(name string) string` in `pkg/hello/hello.go` that returns "Hello, {name}!".

## Files
- `pkg/hello/hello.go` - Main function implementation
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/hello/hello.go"],
  "tests_written": []
}
```

### T002: Create Hello Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Hello Tests

## Context
Add tests for the SayHello function to verify it works correctly.

## What to Build
Create `pkg/hello/hello_test.go` with tests for the SayHello function. Test with various inputs including empty string.

## Files
- `pkg/hello/hello_test.go` - Test file
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/hello/hello_test.go"],
  "tests_written": ["pkg/hello/hello_test.go"]
}
```