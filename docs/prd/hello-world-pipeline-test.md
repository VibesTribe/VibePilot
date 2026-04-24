# PRD: Hello World Pipeline Test

Priority: Low
Complexity: Simple
Category: coding

## Context
Pipeline validation test. The goal is to verify the full VibePilot pipeline works end-to-end: PRD → plan → task → execute → test → complete. This task should produce a single output file with no modifications to any existing project files.

## What to Build
Create a single file `output/hello.json` containing a JSON object with:
- A `greeting` field set to `Hello from VibePilot!`
- A `status` field set to `success`
- A `generated_at` field with the current ISO 8601 timestamp
- A `pipeline` field set to `e2e-test-passed`

## Files
- `output/hello.json` - The output artifact

## Expected Output
A valid JSON file at `output/hello.json` that parses correctly and contains all four fields.

## Constraints
- Do NOT modify any existing files
- Do NOT touch any Go code, vibepilot internals, or configuration files
- Only create the single file specified above
- The file must be valid JSON

