# PLAN: Test Model Assignment and Token Tracking

## Overview
Test the complete VibePilot flow from PRD to task execution by creating a simple math package with model assignment and token tracking validation.

## Tasks

### T001: Create Divide Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Divide Function

## Context
Create a simple Divide function to test the VibePilot flow with model assignment and token tracking. This validates the dashboard can show which model is working on each task.

## What to Build
Create `pkg/math/divide.go` with:
- Function `Divide(a, b float64) (float64, error)` that returns a / b
- Return error when dividing by zero with message "division by zero"
- Include package documentation explaining the math package purpose
- Handle edge cases:
  - Negative numbers (should work normally)
  - Zero dividend (should return 0, nil)
  - Zero divisor (should return error)

## Files
- `pkg/math/divide.go` - Divide function implementation

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/divide.go"],
  "tests_written": []
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/divide.go"],
  "tests_written": []
}
```

---

### T002: Create Divide Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Divide Tests

## Context
Create comprehensive tests for the Divide function to ensure all edge cases are handled correctly.

## What to Build
Create `pkg/math/divide_test.go` with table-driven tests covering:
- Positive numbers (e.g., 10.0 / 2.0 = 5.0)
- Negative numbers (e.g., -10.0 / 2.0 = -5.0)
- Zero dividend (e.g., 0.0 / 5.0 = 0.0)
- Division by zero (should return error)
- Mixed positive/negative (e.g., 10.0 / -2.0 = -5.0)
- Decimal results (e.g., 7.0 / 2.0 = 3.5)

Each test case should have:
- Name/description
- Input values (a, b)
- Expected result
- Expected error state

## Files
- `pkg/math/divide_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/math/divide_test.go"],
  "tests_written": ["pkg/math/divide_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/math/divide_test.go"],
  "tests_written": ["pkg/math/divide_test.go"]
}
```

---

### T003: Run Tests and Verify
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T002

#### Prompt Packet
```
# TASK: T003 - Run Tests and Verify

## Context
Execute the tests to verify the Divide function works correctly and all edge cases pass.

## What to Build
Run `go test ./pkg/math/... -v` and verify:
- All tests pass
- No compilation errors
- Coverage is reasonable (if reported)

If tests fail, report the failure details.

## Files
- No new files created

## Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "tests_written": [],
  "test_results": {
    "passed": true,
    "output": "test output summary"
  }
}
```
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "tests_written": [],
  "test_results": {
    "passed": true,
    "output": "All tests passing"
  }
}
```
