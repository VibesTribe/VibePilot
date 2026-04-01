# PLAN: Test Simple Fix

## Overview
Create a minimal Go program to verify the governor CLI runner fix is working correctly.

## Tasks

### T001: Create hello_fix.go
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create hello_fix.go

## Context
The governor CLI runner fix needs verification. A simple Go program will confirm the build and run pipeline is functional.

## What to Build
Create a file `hello_fix.go` in the project root with a Go main package that:
1. Has a `main()` function
2. Imports `fmt`
3. Prints exactly: `Fix verified!`
4. Can be executed with `go run hello_fix.go`

## Files
- `hello_fix.go` - The Go program to create
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["hello_fix.go"],
  "tests_written": []
}
```

### T002: Verify Program Execution
**Confidence:** 0.99
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
```
# TASK: T002 - Verify Program Execution

## Context
Need to confirm the Go program created in T001 compiles and runs correctly.

## What to Do
1. Run `go run hello_fix.go`
2. Verify output is exactly `Fix verified!` (with newline)
3. Confirm exit code is 0

## Files
- `hello_fix.go` - The file to test
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": [],
  "verified": true,
  "output": "Fix verified!"
}
```