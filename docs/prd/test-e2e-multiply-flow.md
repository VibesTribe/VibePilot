# PRD: Test End-to-End Flow with Model assignment and token tracking

Priority: Low
Complexity: Simple
Category: coding

## Context
Test the complete VibePilot flow from PRD to task execution with the new migration 064 for model assignment and token tracking.

## What to Build
Create a simple `pkg/math/multiply.go` with:
- Function `Multiply(a, b int) int` that returns a * b
- Include package documentation
- Handle edge cases (negative numbers, zero, overflow)

Create `pkg/math/multiply_test.go` with:
- Test cases for:
  - Positive numbers
  - Negative numbers
  - Zero values
  - Mixed positive/negative
  - Overflow handling
- Use table-driven tests
- Include test descriptions

## Files
- `pkg/math/multiply.go` - Multiply function implementation
- `pkg/math/multiply_test.go` - Unit tests

## Expected Output
- Two files created
- All tests passing
- Code committed to task branch
- Dashboard shows model assignment and token counts
