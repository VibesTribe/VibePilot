# Test PRD - Simple File Creation

## Goal
Verify that the governor can route tasks and create execution sessions.

## Requirements
1. Create a simple text file at `/tmp/test-governor.txt`
2. The file should contain the current date and time
3. Add a brief summary of what was done

## Success Criteria
- File exists at `/tmp/test-governor.txt`
- File contains current date/time
- A summary is included

## Notes
This is a minimal test to verify:
- Router finds internal_cli → glm-5 → claude-code
- Governor spawns second Claude CLI session
- Task execution completes successfully
