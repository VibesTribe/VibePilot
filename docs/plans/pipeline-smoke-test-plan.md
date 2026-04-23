# PLAN: Pipeline Smoke Test

## Overview
Add a simple greeting print to verify the VibePilot pipeline works end-to-end with a trivial change.

## Tasks

### T001: Add Greeting Output to Hello Package
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add Greeting Output to Hello Package

## Context
This is a pipeline smoke test. The goal is to make a trivial code change to verify the full VibePilot pipeline (PRD → Plan → Task → Execute → Test → Merge) works correctly. The existing `governor/internal/hello/hello.go` already has a `Greet` function.

## What to Build
Modify the `Greet` function in `governor/internal/hello/hello.go` so that when called with the argument `"There"`, it returns the exact string:

`Hi There. Guess What? I'm working!`

The function should format the greeting as: `Hi <name>. Guess What? I'm working!`

If the file already has the correct implementation, leave it unchanged.

## Files
- `governor/internal/hello/hello.go` - Modify the Greet function
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [],
  "files_modified": ["governor/internal/hello/hello.go"],
  "tests_written": []
}
```
