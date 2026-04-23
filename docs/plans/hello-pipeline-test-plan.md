# PLAN: Hello Pipeline Test

## Overview
Minimal end-to-end pipeline smoke test. Create a Python script that prints "Hello from VibePilot!" with a built-in self-test.

## Tasks

### T001: Create hello.py with self-test
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create hello.py with self-test

## Context
This is a smoke test for the VibePilot pipeline. The goal is to prove the full PRD-to-task-to-execution path works by producing a trivial but verifiable artifact.

## What to Build
Create a single Python file at the repository root called `hello.py` that:

1. Defines a function `greet()` that returns the string `Hello from VibePilot!`
2. When run as `__main__`, calls `greet()`, prints the result, and runs a self-test
3. The self-test asserts that `greet()` returns exactly `Hello from VibePilot!`
4. Exit code must be 0 on success
5. No external dependencies — only the Python standard library

## Files
- `hello.py` — the script with greet() function, __main__ block, and self-test
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["hello.py"],
  "tests_written": ["hello.py (inline self-test)"]
}
```