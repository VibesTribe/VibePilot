# PLAN: Hello VibePilot v4

## Overview
Modify `governor/cmd/tools/hello.go` to print only "Hello VibePilot!" when executed.

## Tasks

### T001: Update Hello Tool Output
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Update Hello Tool Output

## Context
The hello.go tool currently prints two lines. The PRD requires it to print only "Hello VibePilot!" when run.

## What to Build
Modify `governor/cmd/tools/hello.go` so that `go run ./cmd/tools/hello.go` outputs exactly:

Hello VibePilot!

The main() function should print only this single line. You may keep or remove the SayHello helper function as needed.

## Files
- `governor/cmd/tools/hello.go` - Modify to print only "Hello VibePilot!"
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/cmd/tools/hello.go"],
  "tests_written": [],
  "verification": "go run ./cmd/tools/hello.go outputs: Hello VibePilot!"
}
```