# PLAN: Test Simple Math Utility

## Overview
Create a simple Go package with a math utility function to verify the full VibePilot flow works correctly.

## Tasks

### T001: Create Add Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Add Function

## Context
Create a simple math utility function that adds two integers. This is a test task to verify the VibePilot planning and execution flow works correctly.

## What to Build
Create a Go package at pkg/math/ with an Add function that takes two integers and returns their sum.

## Files
- pkg/math/add.go - The Add function implementation

## Implementation Details
- Package name: math
- Function signature: func Add(a, b int) int
- Return the sum of a and b

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/add.go"],
  "tests_written": []
}
```
```

---

### T002: Create Add Function Tests
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Add Function Tests

## Context
Write comprehensive tests for the Add function created in T001.

## What to Build
Create table-driven tests for the Add function covering positive numbers, negative numbers, and zero.

## Files
- pkg/math/add_test.go - Test file

## Test Cases Required
1. Positive numbers: Add(2, 3) = 5
2. Negative numbers: Add(-2, -3) = -5
3. Mixed: Add(-2, 3) = 1
4. Zero: Add(0, 0) = 0
5. Zero with positive: Add(0, 5) = 5

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/math/add_test.go"],
  "tests_written": ["pkg/math/add_test.go"]
}
```
```
