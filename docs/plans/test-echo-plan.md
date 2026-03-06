# PLAN: Test Echo

## Overview
Create a simple echo function that returns its input string unchanged.

## Tasks

### T001: Create Echo Function and Tests
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create Echo Function and Tests

## Context
A simple echo function is needed for testing purposes. This function should accept a string and return it unchanged.

## What to Build
1. Create `pkg/echo/echo.go` with function `Echo(s string) string` that returns the input unchanged
2. Create `pkg/echo/echo_test.go` with tests covering:
   - Returns input unchanged
   - Handles empty string

## Files
- `pkg/echo/echo.go` - Echo function implementation
- `pkg/echo/echo_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/echo/echo.go", "pkg/echo/echo_test.go"],
  "tests_written": ["pkg/echo/echo_test.go"]
}
```
```

#### Expected Output
```json
{
  "files_created": ["pkg/echo/echo.go", "pkg/echo/echo_test.go"],
  "tests_required": ["pkg/echo/echo_test.go"]
}
```
