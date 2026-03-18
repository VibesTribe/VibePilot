# PLAN: Test Slice Numbering

## Overview
Create a simple utility function `SayHello` in Go that returns a greeting message.

## Tasks

### T001: Create Greet Utility Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Greet Utility Function

## Context
A simple utility function is needed to demonstrate proper slice numbering in the planner.

## What to Build
Create `utils/greet.go` with:
- Package `utils`
- Function `SayHello(name string) string` that returns "Hello, {name}!"

Create `utils/greet_test.go` with:
- Basic test for `SayHello` function
- Test that `SayHello("World")` returns "Hello, World!"

## Files
- `utils/greet.go` - The greet function implementation
- `utils/greet_test.go` - Unit tests for the greet function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["utils/greet.go", "utils/greet_test.go"],
  "tests_written": ["utils/greet_test.go"]
}
```
