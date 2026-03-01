# PLAN: Hello World Test

## Overview
Verify the complete VibePilot execution flow works end-to-end by creating a simple text file. This is a smoke test to validate the system from PRD → Plan → Task → Execution → Completion.

## Success Criteria
- Plan generated and approved
- Task created and assigned to executable destination
- File `hello.txt` created in project root
- Task completes successfully

## Tasks

### T001: Create Hello World File
**Confidence:** 1.0
**Dependencies:** none
**Type:** feature
**Category:** coding
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T001 - Create Hello World File

## CONTEXT
This is a smoke test task to verify the complete VibePilot execution pipeline works end-to-end. The task is intentionally simple - create a single text file with a timestamp - to validate that tasks can be received, executed, and completed successfully without any complex dependencies or logic.

## DEPENDENCIES
None.

## WHAT TO BUILD
Create a text file named `hello.txt` in the project root directory. The file should contain a greeting message with the current timestamp.

## FILES TO CREATE
- `hello.txt` - A simple text file in the project root with the content specified below

## TECHNICAL SPECIFICATIONS

### File Location
- Path: `hello.txt` (project root, i.e., `/home/mjlockboxsocial/vibepilot/hello.txt`)

### File Content
The file must contain exactly:
```
Hello from VibePilot! [YYYY-MM-DD HH:MM:SS]
```
Where `[YYYY-MM-DD HH:MM:SS]` is replaced with the current timestamp at execution time.

Example:
```
Hello from VibePilot! 2026-03-01 16:30:45
```

### Timestamp Format
- Use ISO 8601 format without timezone: `YYYY-MM-DD HH:MM:SS`
- Use UTC time
- Example in Python: `datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S')`

## ACCEPTANCE CRITERIA
- [ ] File `hello.txt` exists in project root
- [ ] File contains text "Hello from VibePilot!" followed by a timestamp
- [ ] Timestamp is in correct format (YYYY-MM-DD HH:MM:SS)
- [ ] File is plain text (not binary, not JSON, no extra formatting)

## TESTS REQUIRED
None (explicitly out of scope per PRD).

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": ["hello.txt"],
  "files_modified": [],
  "summary": "Created hello.txt with timestamp greeting",
  "tests_written": [],
  "notes": "File created successfully at [timestamp]"
}
```

## DO NOT
- Create multiple files
- Add tests
- Call external APIs
- Add any features beyond creating the single text file
- Modify any existing files
- Add dependencies or imports to existing code
```

#### Expected Output
```json
{
  "files_created": ["hello.txt"],
  "files_modified": [],
  "tests_required": []
}
```

---

## Critical Path
T001 (only task)

## Estimated Context
- T001: ~2,000 tokens
- Total: ~2,000 tokens

## Risk Assessment
- **Complexity:** Minimal
- **Dependencies:** None
- **Risk Level:** Very Low

## Notes
- This is a smoke test to validate the execution pipeline
- Single task with no dependencies ensures easy debugging if execution fails
- Success validates: task receipt → file creation → completion reporting