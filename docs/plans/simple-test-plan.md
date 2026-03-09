# PLAN: Simple Test

## Overview
Create a file called `test-output.txt` with content "Hello from VibePilot" to verify the flow works end-to-end.

## Tasks

### T001: Create Test Output File
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Test Output File

## Context
Simple test to verify the flow works end-to-end.

## What to Build
Create a file called `test-output.txt` with content "Hello from VibePilot"

## Files
- `test-output.txt` - The output file to create
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["test-output.txt"],
  "tests_written": []
}
```