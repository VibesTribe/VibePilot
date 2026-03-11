# PLAN: Hello VibePilot v4

## Overview
Create/modify `governor/cmd/tools/hello.go` to print "Hello VibePilot!" when run.

## Tasks

### T001: Create Hello Tool
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Tool

## Context
A simple CLI tool that demonstrates VibePilot's ability to create and test Go programs.

## What to Build
Create or modify `governor/cmd/tools/hello.go` to print exactly "Hello VibePilot!" (without comma) when executed.

The file must:
1. Be in package main
2. Have a main() function
3. Print "Hello VibePilot!" to stdout

## Files
- `governor/cmd/tools/hello.go` - The main program file
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```

### T002: Verify Output
**Confidence:** 0.99
**Category:** testing
**Dependencies:** ["T001"]

#### Prompt Packet
```
# TASK: T002 - Verify Output

## Context
Verify the hello tool produces the correct output.

## What to Build
Run the following command and verify output:

```bash
go run ./governor/cmd/tools/hello.go
```

Expected output: Hello VibePilot!

## Files
- No new files needed
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": [],
  "verified": true
}
```