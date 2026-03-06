# PLAN: Test Calculator Multiply Function

## Overview
Create a simple multiply function for testing the VibePilot planning and execution flow.

## Tasks

### T001: Create Multiply Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Multiply Function

## Context
Create a simple multiply function to test the VibePilot autonomous flow from PRD to task execution.

## What to Build
Create a simple Multiply function in a new file that takes two integers and returns their product.

## Files
- `internal/calc/multiply.go` - Multiply function implementation
- `internal/calc/multiply_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["internal/calc/multiply.go", "internal/calc/multiply_test.go"],
  "tests_written": ["internal/calc/multiply_test.go"]
}
```
```

---

### T002: Verify Tests Pass
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Verify Tests Pass

## Context
Ensure the multiply function works correctly by running the tests.

## What to Build
Run the tests for the multiply function and verify they pass.

## Files
- `internal/calc/multiply_test.go` - Tests to run

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": [],
  "tests_passed": true
}
```
```

---