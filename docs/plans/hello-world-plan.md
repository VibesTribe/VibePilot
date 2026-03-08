# PLAN: Hello World

## Overview
Create a simple hello function in Go that returns personalized greetings.

## Tasks

### T001: Create Hello Function
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello Function

## Context
Need a reusable greeting function for the governor tools that can return personalized or default greetings.

## What to Build
Create file `governor/cmd/tools/hello.go` with:
- Package declaration: `package main`
- Function: `SayHello(name string) string`
- Logic: If name is empty string, return "Hello, World!". Otherwise return "Hello, {name}!"

## Files
- `governor/cmd/tools/hello.go` - Create this file with the SayHello function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```
