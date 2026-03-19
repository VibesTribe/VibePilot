# PLAN: Test Dashboard Status Flow

## Overview
Create a simple Go function to test that the dashboard correctly shows task status through the full flow.

## Tasks

### T001: Create Hello Test Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Test Function

## Context
Need to verify the dashboard shows task status correctly through the full flow after recent fixes. This simple test task will flow through: pending → in_progress → review → testing → complete → merged.

## What to Build
Create a simple Go file in `governor/cmd/tools/` that prints "Hello from test task" and exits with code 0.

## Files
- `governor/cmd/tools/hello_test.go` - Go file with main function that prints message
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello_test.go"],
  "tests_written": []
}
```