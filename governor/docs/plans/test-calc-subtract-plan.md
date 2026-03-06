# PLAN: Test Calc Subtract

## Overview
Implement a subtraction calculator function for testing the VibePilot planning system.

## Tasks

### T001: Create Subtract Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Subtract Function

## Context
Create a simple subtraction function to test the VibePilot planning and execution pipeline.

## What to Build
Implement a subtract(a, b) function that returns a - b.

## Files
- `calc/subtract.go` - Subtraction function implementation
- `calc/subtract_test.go` - Unit tests

## Implementation
1. Create calc/subtract.go with Subtract(a, b int) int function
2. Create calc/subtract_test.go with test cases for positive, negative, and zero values

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["calc/subtract.go", "calc/subtract_test.go"],
  "tests_written": ["calc/subtract_test.go"]
}
```
```