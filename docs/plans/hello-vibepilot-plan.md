# PLAN: Hello VibePilot

## Overview
Create a simple Go program that outputs "Hello VibePilot!" when executed.

## Tasks

### T001: Create Hello VibePilot Tool
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello VibePilot Tool

## Context
Need a simple hello world tool that demonstrates VibePilot's ability to create and run Go programs.

## What to Build
Create or modify `governor/cmd/tools/hello.go` to print exactly "Hello VibePilot!" (with exclamation mark) when run.

The file should:
- Be in package main
- Use fmt.Println to output the message
- Output exactly: Hello VibePilot!

## Files
- `governor/cmd/tools/hello.go` - The main Go file
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": [],
  "verification": "go run ./governor/cmd/tools/hello.go outputs: Hello VibePilot!"
}
```