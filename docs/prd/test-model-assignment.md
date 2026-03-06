# PRD: Test Model Assignment and Token Tracking

Priority: Low
Complexity: Simple
Category: coding

## Context
Test the complete VibePilot flow from PRD to task execution with migration 064 for model assignment and token tracking. This validates the dashboard can show which model is working on each task and track token usage.

## What to Build
Create `pkg/math/divide.go` with:
- Function `Divide(a, b float64) (float64, error)` that returns a / b
- Return error when dividing by zero
- Include package documentation
- Handle edge cases (negative numbers, zero dividend)

Create `pkg/math/divide_test.go` with:
- Test cases for:
  - Positive numbers
  - Negative numbers
  - Zero dividend
  - Division by zero (should error)
  - Mixed positive/negative
- Use table-driven tests
- Include test descriptions

## Files
- `pkg/math/divide.go` - Divide function implementation
- `pkg/math/divide_test.go` - Unit tests

## Expected Output
- Two files created
- All tests passing
- Code committed to task branch
- Dashboard shows:
  - Model ID in `assigned_to` field
  - Token counts in `task_runs` table
  - Task in progress with model info
