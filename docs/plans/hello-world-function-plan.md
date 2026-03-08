# PLAN: Hello World Function

## Overview
Create a simple hello world function to test the complete VibePilot flow from PRD to completion.

## Tasks

### T001: Create Hello World Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello World Function

## Context
Testing the complete VibePilot flow by creating a simple greeting function.

## What to Build
Create a Go function in `governor/cmd/tools/hello.go`:
- Function signature: `SayHello(name string) string`
- If name is empty string, return "Hello, World!"
- Otherwise return "Hello, {name}!"

## Files
- `governor/cmd/tools/hello.go` - The hello world function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```