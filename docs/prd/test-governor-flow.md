# Test PRD - Governor Execution Flow

## Goal
Verify the governor can route and execute tasks using internal_cli agent.

## Task
Create a simple test file at `/tmp/governor-test.txt` containing:
- Current date and time
- A brief "Governor test successful" message

## Success Criteria
- File exists at `/tmp/governor-test.txt`
- File contains timestamp
- Task completes without errors

## Notes
This is a minimal test to verify:
1. Router finds internal_cli → glm-5 → claude-code
2. Governor spawns second Claude CLI session
3. Task executes and returns results
