# PLAN: Final Model Assignment Test

## Overview
Create a simple modulo function with tests to verify model assignment and token tracking work correctly.

## Tasks

### T001: Create Modulo Function and Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create Modulo Function and Tests

## Context
Create a simple Go package to test that model assignment and token tracking work correctly after recent fixes.

## What to Build

Create `pkg/math/modulo.go`:
- Function `Modulo(a, b int) (int, error)` that returns a % b
- Return error with message "division by zero" when b is 0
- Include package documentation explaining the modulo function

Create `pkg/math/modulo_test.go`:
- Table-driven tests covering:
  - Positive numbers: Modulo(10, 3) = 1
  - Negative numbers: Modulo(-10, 3) = -1
  - Zero dividend: Modulo(0, 5) = 0
  - Division by zero: Modulo(10, 0) returns error
- Use standard Go testing conventions

## Files
- `pkg/math/modulo.go` - Modulo function implementation
- `pkg/math/modulo_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/modulo.go", "pkg/math/modulo_test.go"],
  "tests_written": ["pkg/math/modulo_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/modulo.go", "pkg/math/modulo_test.go"],
  "tests_written": ["pkg/math/modulo_test.go"]
}
```
