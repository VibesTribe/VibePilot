# PLAN: Hello World

## Overview
Create a simple hello function that returns a greeting message.

## Tasks

### T001: Create SayHello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create SayHello Function

## Context
A reusable greeting function is needed for the governor tools package.

## What to Build
Create a Go file with a SayHello function:
- File: `governor/cmd/tools/hello.go`
- Function signature: `func SayHello(name string) string`
- If name is empty, return "Hello, World!"
- Otherwise, return "Hello, {name}!"

## Files
- `governor/cmd/tools/hello.go` - The hello function implementation
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```
