# PLAN: Test Add Function

## Overview
Create a simple Add function in Go with comprehensive tests to validate the full VibePilot flow.

## Tasks

### T001: Create Add Function with Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create Add Function with Tests

## Context
Create a simple Add function to test the full VibePilot flow from PRD to execution. This validates the system can handle basic coding tasks.

## What to Build
1. Create `pkg/math/add.go` with:
   - Function `Add(a, b int) int` that returns the sum of two integers
   - Include package documentation
   - Handle edge cases (negative numbers, zero, max/min int)

2. Create `pkg/math/add_test.go` with:
   - Test cases for:
     - Positive numbers
     - Negative numbers
     - Zero values
     - Mixed positive/negative
     - Edge cases (max/min int if applicable)
   - Use table-driven tests
   - Include test descriptions

## Files
- `pkg/math/add.go` - Add function implementation
- `pkg/math/add_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "pkg/math/add.go",
    "pkg/math/add_test.go"
  ],
  "tests_written": ["pkg/math/add_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "pkg/math/add.go",
    "pkg/math/add_test.go"
  ],
  "tests_written": ["pkg/math/add_test.go"]
}
```
