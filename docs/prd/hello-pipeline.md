# Hello Pipeline Test

## Summary
Minimal test PRD to verify the full VibePilot pipeline is working end-to-end. This is a hello-world style task — write a simple script that prints "Hello from VibePilot!" to stdout.

## Requirements
1. Create a single Python file: `hello.py`
2. The script should print exactly: `Hello from VibePilot!`
3. Include a basic self-test that validates the output matches the expected string

## Acceptance Criteria
- `python3 hello.py` outputs `Hello from VibePilot!`
- Exit code is 0
- No external dependencies

## Priority
Low — this is a smoke test for the pipeline itself, not production code.
