# PLAN: Simple Test Function

## Overview
Create a simple Go function to verify the full flow works end-to-end.

## Tasks

### T001: Create Greet Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Greet Function

## Context
Verify the full VibePilot flow works end-to-end by creating a simple Go function.

## What to Build
Create a `Greet(name string) string` function in `governor/cmd/tools/greet.go` that returns "Hello, {name}!".

## Files
- `governor/cmd/tools/greet.go` - Create this file with the Greet function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/greet.go"],
  "tests_written": []
}
```
