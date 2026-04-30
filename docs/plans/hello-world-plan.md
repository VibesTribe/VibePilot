# PLAN: Hello World

## Overview
E2E pipeline smoke test. Minimal task to verify the full pipeline works: planning, execution, review, testing, merge.

## Tasks

### T001: Create Hello World JSON
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none
**Target Files:** output/hello.json

#### Prompt Packet
```
# TASK: T001 - Create Hello World JSON

## Context
This is a pipeline validation task. The goal is to produce a single JSON output file to verify the pipeline works end-to-end.

## What to Build
Create the file `output/hello.json` with valid JSON containing:
- A "greeting" field set to "Hello from VibePilot"
- A "timestamp" field set to the current date in ISO 8601 format (YYYY-MM-DD)

Do NOT modify any existing files. Only create `output/hello.json`.

## Files
- `output/hello.json` - The output artifact
```

#### Expected Output
```json
{
  "files_created": ["output/hello.json"],
  "tests_written": []
}
```
",
  "tasks": [
    {
      "task_number": "T001",
      "title": "Create Hello World JSON",
      "category": "coding",
      "confidence": 0.99,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Create Hello World JSON

## Context
This is a pipeline validation task. The goal is to produce a single JSON output file to verify the pipeline works end-to-end.

## What to Build
Create the file `output/hello.json` with valid JSON containing:
- A "greeting" field set to "Hello from VibePilot"
- A "timestamp" field set to the current date in ISO 8601 format (YYYY-MM-DD)

Do NOT modify any existing files. Only create `output/hello.json`.

## Files
- `output/hello.json` - The output artifact",
      "expected_output": {
        "files_created": [
          "output/hello.json"
        ],
        "tests_written": []
      }
    }
  ],
  "total_tasks": 1,
  "status": "review