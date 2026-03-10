# PLAN: Test Simple Hello

## Overview
Create a simple greeting function that takes a name and returns "Hello, {name}!" to verify the V0.4.0 flow works end-to-end.

## Tasks

### T001: Add Greeting Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add Greeting Function

## Context
The PRD requires a greeting function to test the V0.4.0 flow end-to-end with proper task routing and execution.

## What to Build
Add a `Greet(name string) string` function to the existing hello package that takes a name parameter and returns "Hello, {name}!".

## Files
- `pkg/hello/hello.go` - Add the Greet function
- `pkg/hello/hello_test.go` - Add tests for the Greet function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/hello/hello_test.go"],
  "files_modified": ["pkg/hello/hello.go"],
  "tests_written": ["pkg/hello/hello_test.go"]
}
```
