# PLAN: Hello VibePilot v3

## Overview
Fix the output string in hello.go to print "Hello, VibePilot!" correctly (single exclamation mark).

## Tasks

### T001: Fix Hello Output
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Fix Hello Output

## Context
The file governor/cmd/tools/hello.go currently passes "VibePilot!" to SayHello(), which adds another "!", resulting in "Hello, VibePilot!!" (double exclamation). We need exactly "Hello, VibePilot!" (single exclamation).

## What to Build
Update governor/cmd/tools/hello.go line 14:
- Change `fmt.Println(SayHello("VibePilot!"))` to `fmt.Println(SayHello("VibePilot"))`
- The SayHello function already adds "!" at the end, so passing "VibePilot" will produce the correct output.

## Files
- `governor/cmd/tools/hello.go` - Update line 14 in main function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_modified": ["governor/cmd/tools/hello.go"],
  "tests_written": [],
  "verification": "go run ./cmd/tools/hello.go should output:\nHello, World!\nHello, VibePilot!"
}
```