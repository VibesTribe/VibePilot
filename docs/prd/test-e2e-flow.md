# PRD: Test End-to-End Flow

Priority: Low
Complexity: Simple
Category: coding

## Context
Test the complete VibePilot flow from PRD to task execution with model assignment and token tracking.

## What to Build
Create a simple `pkg/math/subtract.go` with:
- Function `Subtract(a, b int) int` that returns a - b
- Include package documentation
- Handle edge cases (negative numbers, zero)

Create `pkg/math/subtract_test.go` with:
- Test cases for positive, negative, zero, and mixed values
- Use table-driven tests
- Include test descriptions

## Files
- `pkg/math/subtract.go` - Subtract function implementation
- `pkg/math/subtract_test.go` - Unit tests

## Expected Output
- Two files created
- All tests passing
- Code committed to task branch
