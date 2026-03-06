# PRD: Final Model Assignment Test

Priority: Low
Complexity: Simple
Category: coding

## Context
Final test of model assignment and token tracking after implementing fixes.

## What to Build
Create `pkg/math/modulo.go` with:
- Function `Modulo(a, b int) (int, error)` that returns a % b
- Return error when dividing by zero
- Include package documentation

Create `pkg/math/modulo_test.go` with:
- Table-driven tests for positive, negative, and zero values
- Division by zero error test

## Files
- `pkg/math/modulo.go` - Modulo function implementation
- `pkg/math/modulo_test.go` - Unit tests

## Expected Output
- Two files created
- All tests passing
- Dashboard shows model assignment and token counts
