# PLAN: Test Simple Automation

## Overview
Create a simple Python test script to validate the fully automated VibePilot pipeline and measure stage timings.

## Tasks

### T001: Create Automation Test Script
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Create Automation Test Script

## Context
We need a simple Python test script to validate that the VibePilot pipeline works end-to-end. This script serves as a minimal smoke test for the automation pipeline.

## What to Build
Create a Python file at `tests/test_automation.py` that:
1. Defines a function called `test_automation`
2. Inside the function, prints "Automation test passed!" to stdout
3. Has a `if __name__ == "__main__"` block that calls `test_automation()`
4. Can be executed directly with `python tests/test_automation.py`

The script should be simple, self-contained, and require no external dependencies.

## Files
- `tests/test_automation.py` - The test script
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["tests/test_automation.py"],
  "tests_written": []
}
```
