# PLAN: Test Dashboard Status Flow

## Overview
Create a simple Go function in governor/cmd/tools/ that prints "Hello from test task" and exits. This task will verify the dashboard shows task status correctly through the full flow.

## Tasks

### T001: Create Hello Test Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Test Function

## Context
A simple test function is needed to verify the dashboard status flow works correctly. This function will be executed by the task runner and should complete successfully to demonstrate the full flow: pending → in_progress → review → testing → complete → merged.

## What to Build
Create a Go file with a main function that prints "Hello from test task" and exits with status 0.

## Files
- `governor/cmd/tools/hello_test.go` - Simple Go program with main function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello_test.go"],
  "tests_written": []
}
```