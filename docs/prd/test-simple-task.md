# PRD: Simple Hello World Task

Priority: Low
Complexity: Simple
Category: coding
Module: general

## Context
Test task to validate VibePilot governor workflow. This is a simple "hello world" task to measure timing and identify any issues in the pipeline.

## What to Build
Create a simple "Hello from VibePilot" function in Go that prints a message.

## Files
- `governor/cmd/tools/hello_vibepilot.go`

## Expected Output
- Go file created with working hello function
- Function prints "Hello from VibePilot!" when run
- File is syntactically correct Go code
- Task flows through complete workflow: PRD → plan → tasks → code → merge

## Success Criteria
- Governor detects PRD from GitHub
- Planner creates plan with breakdown
- Tasks created and assigned
- Code generated
- Committed to git
- Dashboard shows progress at each stage
