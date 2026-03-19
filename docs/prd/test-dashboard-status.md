# PRD: Test Dashboard Status Flow

Priority: Low
Complexity: Simple
Category: coding
Module: general

## Context
Test to verify dashboard shows task status correctly through the full flow after recent fixes.

## What to Build
Create a simple Go function in `governor/cmd/tools/` that prints "Hello from test task" and exits.

## Files
- governor/cmd/tools/hello_test.go

## Expected Output
- File created with working Go code
- Task flows through: pending → in_progress → review → testing → complete → merged
- Dashboard shows correct status at each stage
