# PLAN: Calculator Modulo Function

## Overview
Implement a modulo function for the calculator module that returns the remainder of division with proper error handling for division by zero.

## Tasks

### T001: Implement Modulo Function with Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Implement Modulo Function with Tests

## Context
The calculator module needs a modulo function that safely computes the remainder of integer division. This function must handle the edge case of division by zero by returning an error.

## What to Build
Create a modulo function in Go with the following specification:
- Function signature: `func Modulo(a, b int) (int, error)`
- Returns the remainder of a divided by b
- Returns error when b is zero (division by zero)
- Include comprehensive unit tests covering:
  - Normal cases (positive and negative numbers)
  - Edge cases (division by zero, zero dividend)
  - Various combinations of positive/negative operands

## Files
- `internal/calc/modulo.go` - Implementation of the Modulo function
- `internal/calc/modulo_test.go` - Unit tests for the Modulo function

## Implementation Details
1. Create `internal/calc/modulo.go` with:
   - Package declaration `package calc`
   - Exported function `Modulo(a, b int) (int, error)`
   - Return error if b == 0 using `errors.New("division by zero")`
   - Return a % b for valid inputs

2. Create `internal/calc/modulo_test.go` with:
   - Test cases for: positive/positive, positive/negative, negative/positive, negative/negative
   - Test case for division by zero
   - Test case for zero dividend
   - Use table-driven tests pattern

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["internal/calc/modulo.go", "internal/calc/modulo_test.go"],
  "tests_written": ["internal/calc/modulo_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["internal/calc/modulo.go", "internal/calc/modulo_test.go"],
  "tests_written": ["internal/calc/modulo_test.go"]
}
```
