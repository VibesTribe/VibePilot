# PLAN: Hello Pipeline v2 Smoke Test

## Overview
Create and verify a simple shell script to ensure pipeline functionality.

## Tasks

### T001: Create Hello Script
**Confidence:** 1.0
**Category:** coding
**Dependencies:** none

#### Prompt Packet
# TASK: T001 - Create Hello Script

## Context
Creating a base script to verify the execution pipeline.

## What to Build
Create a shell script at `scripts/hello.sh` that prints exactly "Hello from VibePilot!" to stdout.

## Files
- `scripts/hello.sh` - New file

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["scripts/hello.sh"],
  "tests_written": []
}
```

### T002: Verify Hello Script Execution
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T001

#### Prompt Packet
# TASK: T002 - Verify Hello Script Execution

## Context
Verifying that the script created in T001 executes correctly within the environment.

## What to Build
Execute `scripts/hello.sh` using `os/exec`. Verify the output matches exactly "Hello from VibePilot!".

## Files
- `scripts/hello.sh` - Existing file to verify
- `governor/internal/tools/sandbox_tools.go` - Reference implementation for execution

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": ["scripts/hello_test.sh"]
}
```