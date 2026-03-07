# PLAN: Add Hello Function

## Overview
Create a simple hello function to test the VibePilot flow end-to-end.

## Tasks

### T001: Create Hello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Function

## Context
We need a simple hello function to test the VibePilot flow end-to-end. This serves as a minimal test case for the entire system.

## What to Build
Create a Go file at `governor/cmd/tools/hello.go` that:
1. Defines a `Hello()` function that prints "Hello from VibePilot!" to stdout and returns nil error
2. Contains a `main()` function that calls `Hello()` and exits with status 0

## Files
- `governor/cmd/tools/hello.go` - The hello function implementation
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```