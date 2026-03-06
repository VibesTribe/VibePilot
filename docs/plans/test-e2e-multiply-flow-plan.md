# PLAN: Test End-to-End Flow with Model Assignment and Token Tracking

## Overview
Create a simple multiply function with comprehensive tests to validate the complete VibePilot flow from PRD to task execution, including model assignment and token tracking.

## Tasks

### T001: Create Multiply Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Multiply Function

## Context
Create the core multiply function that will be tested in the e2e flow validation. This function demonstrates basic Go package structure and handles edge cases.

## What to Build
Create `pkg/math/multiply.go` with:
- Package declaration: `package math`
- Function signature: `func Multiply(a, b int) int`
- Implementation: return a * b
- Package-level documentation comment explaining the function's purpose
- Add a comment noting that integer overflow behavior is undefined (Go's default behavior)
- No error handling needed - Go's standard integer multiplication behavior is acceptable

## Files
- `pkg/math/multiply.go` - Multiply function implementation

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/multiply.go"],
  "tests_written": []
}
```
```

#### Expected Output
```json
{
  "files_created": ["pkg/math/multiply.go"],
  "tests_required": []
}
```

---

### T002: Create Multiply Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Create Multiply Tests

## Context
Create comprehensive unit tests for the Multiply function to validate test execution in the e2e flow and demonstrate table-driven testing patterns.

## What to Build
Create `pkg/math/multiply_test.go` with:
- Package declaration: `package math_test`
- Import statements for `testing` and the math package
- Table-driven test function `TestMultiply` with test cases for:
  - Two positive numbers (e.g., 3 * 4 = 12)
  - Two negative numbers (e.g., -3 * -4 = 12)
  - One positive, one negative (e.g., 3 * -4 = -12)
  - Zero as first operand (e.g., 0 * 5 = 0)
  - Zero as second operand (e.g., 5 * 0 = 0)
  - Both zeros (e.g., 0 * 0 = 0)
  - Identity (e.g., 7 * 1 = 7)
- Each test case should have a descriptive name field
- Use `t.Run` for subtests
- Add a comment explaining the table-driven test pattern

## Files
- `pkg/math/multiply_test.go` - Unit tests for Multiply function

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["pkg/math/multiply_test.go"],
  "tests_written": ["pkg/math/multiply_test.go"]
}
```
```

#### Expected Output
```json
{
  "files_created": ["pkg/math/multiply_test.go"],
  "tests_required": ["pkg/math/multiply_test.go"]
}
```

---

### T003: Verify Tests Pass
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T002

#### Prompt Packet
```
# TASK: T003 - Verify Tests Pass

## Context
Run the tests to verify the implementation is correct and the e2e flow captures test results properly.

## What to Build
Execute tests and verify:
- Run `go test ./pkg/math/... -v` in the project root
- Confirm all tests pass
- Report the test output
- Note: Do not commit yet - that's handled separately

## Files
- No new files created

## Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "tests_written": [],
  "test_results": "All tests passing"
}
```
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": []
}
```
