# Test PRD - Calculator Divide

## Summary
Create a division function for the calculator module.

## Requirements
- Function: `divide(a, b float64) (float64, error)`
- Returns quotient of two numbers
- Returns error when dividing by zero

## Acceptance Criteria
- Returns correct quotient
- Handles division by zero error
- Unit tests with edge cases

## Technical Notes
- Place in `internal/calc/divide.go`
- Tests in `internal/calc/divide_test.go`
