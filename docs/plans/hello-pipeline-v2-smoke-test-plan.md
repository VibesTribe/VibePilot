# PLAN: Hello Pipeline v2 - Smoke Test

## Overview
This plan implements a basic smoke test by creating a shell script and verifying its output to ensure the environment is functioning correctly.

## Tasks

### T001: Create hello.sh script
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create hello.sh script

## Context
We need a simple shell script to act as a smoke test for the pipeline, verifying file creation and execution capabilities.

## What to Build
Create a shell script named `hello.sh` in the repository root. The script should use `#!/bin/sh` and print the exact string "Hello from VibePilot!" followed by a newline.

## Files
- `hello.sh` - Script that prints the smoke test message
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["hello.sh"],
  "tests_written": []
}
```

---

### T002: Verify hello.sh execution and output
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```markdown
# TASK: T002 - Verify hello.sh execution and output

## Context
This task ensures that the script created in T001 actually performs as expected in the current environment.

## What to Build
Execute the `hello.sh` script. Capture the output and verify it matches "Hello from VibePilot!" exactly. 

## Files
- `hello.sh` - Script to be executed
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "execution_results": [
    {
      "command": "sh hello.sh",
      "expected_stdout": "Hello from VibePilot!\n"
    }
  ]
}
```