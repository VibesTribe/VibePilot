# PLAN: Test Hello World

## Overview
Simple hello world function to test VibePilot flow.

## Tasks

### T001: Create Hello World Tool
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello World Tool

## Context
Create a simple hello world program to verify the VibePilot autonomous execution flow works end-to-end.

## What to Build
Create a Go file with:
1. A `Hello()` function that prints "Hello, VibePilot!"
2. A `main()` function that calls `Hello()`

## Files
- `governor/cmd/tools/hello.go` - The hello world program
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```
