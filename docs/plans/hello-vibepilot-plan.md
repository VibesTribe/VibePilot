# PLAN: Hello VibePilot

## Overview
Modify `governor/cmd/tools/hello.go` to print exactly "Hello VibePilot!" when executed.

## Tasks

### T001: Update Hello Output
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Update Hello Output

## Context
The hello.go tool should output exactly "Hello VibePilot!" as specified in the PRD.

## What to Build
Modify `governor/cmd/tools/hello.go` to print only "Hello VibePilot!" (without comma) when run.

## Files
- `governor/cmd/tools/hello.go` - Update main() to print exactly "Hello VibePilot!"
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/cmd/tools/hello.go"],
  "tests_written": [],
  "verification": "go run ./governor/cmd/tools/hello.go outputs: Hello VibePilot!"
}
```