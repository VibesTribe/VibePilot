# PLAN: Simple Hello

## Overview
Create a hello function that returns "Hello!".

## Tasks

### T001: Create Hello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Function

## Context
Need a simple hello function as a basic greeting utility.

## What to Build
Create a Go package with a Hello() function that returns the string "Hello!".

## Files
- `pkg/hello/hello.go` - Main implementation with Hello() function
- `pkg/hello/hello_test.go` - Unit tests for Hello() function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/hello/hello.go", "pkg/hello/hello_test.go"],
  "tests_written": ["pkg/hello/hello_test.go"]
}
```