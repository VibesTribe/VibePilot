# PLAN: Simple Math Utility

## Overview
Create a multiply function that returns the product of two integers, with comprehensive test coverage.

## Tasks

### T001: Implement Multiply Function with Tests
**Confidence:** 0.98
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Implement Multiply Function with Tests

## Context
We need a simple math utility function to multiply two integers. This is a foundational utility that will be used across the codebase.

## What to Build
1. Create a `multiply(a, b int) int` function that returns the product of two integers
2. Write comprehensive tests covering:
   - Positive numbers
   - Zero multiplication
   - Negative numbers
   - Mixed positive/negative

## Files
- `pkg/math/multiply.go` - Implementation of multiply function
- `pkg/math/multiply_test.go` - Unit tests

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["pkg/math/multiply.go", "pkg/math/multiply_test.go"],
  "tests_written": ["pkg/math/multiply_test.go"]
}
```
```

#### Expected Output
```json
{
  "files_created": ["pkg/math/multiply.go", "pkg/math/multiply_test.go"],
  "tests_required": ["pkg/math/multiply_test.go"]
}
```