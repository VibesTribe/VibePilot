# PLAN: Hello VibePilot v2

## Overview
Update the hello.go file to output exactly "Hello VibePilot!" (without comma) as specified in the PRD.

## Tasks

### T001: Update Hello Output
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Update Hello Output

## Context
The file governor/cmd/tools/hello.go currently outputs "Hello, VibePilot!" (with comma) but the PRD requires exactly "Hello VibePilot!" (no comma).

## What to Build
Update governor/cmd/tools/hello.go to:
- Keep a main function
- Print exactly "Hello VibePilot!" to stdout (no comma)

## Files
- `governor/cmd/tools/hello.go` - Update to print exact required output
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [],
  "files_modified": ["governor/cmd/tools/hello.go"],
  "tests_written": [],
  "verification": "go run ./cmd/tools/hello.go outputs Hello VibePilot!"
}
```