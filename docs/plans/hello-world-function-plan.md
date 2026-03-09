# PLAN: Hello World Function

## Overview
Create a simple Hello World function to test the VibePilot flow end-to-end.

## Tasks

### T001: Create SayHello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create SayHello Function

## Context
Implement a simple Hello World function in Go to validate the VibePilot development workflow.

## What to Build
Create `pkg/hello/hello.go` with a function `SayHello(name string) string` that returns "Hello, {name}!".

## Files
- `pkg/hello/hello.go` - The Hello World function implementation
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/hello/hello.go"],
  "tests_written": []
}
```

### T002: Create Tests for SayHello
**Confidence:** 0.99
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Tests for SayHello

## Context
Write unit tests to verify the SayHello function works correctly.

## What to Build
Create `pkg/hello/hello_test.go` with tests that verify:
- SayHello("World") returns "Hello, World!"
- SayHello("") returns "Hello, !"
- SayHello("Alice") returns "Hello, Alice!"

## Files
- `pkg/hello/hello_test.go` - Unit tests for SayHello
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/hello/hello_test.go"],
  "tests_written": ["pkg/hello/hello_test.go"]
}
```