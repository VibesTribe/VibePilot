# PLAN: Hello VibePilot

## Overview
Create a simple Go program that prints "Hello VibePilot!" to stdout.

## Tasks

### T001: Create Hello VibePilot Tool
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Hello VibePilot Tool

## Context
A simple hello world program to verify the VibePilot tooling setup works correctly.

## What to Build
Create a Go file at `governor/cmd/tools/hello.go` with a main function that prints "Hello VibePilot!" to stdout.

## Files
- `governor/cmd/tools/hello.go` - Main entry point that prints the message
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["governor/cmd/tools/hello.go"],
  "tests_written": []
}
```