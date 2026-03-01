# PRD: Echo Test

## Problem Statement
Verify the basic VibePilot planning flow works end-to-end.

## Target Users
VibePilot system testing.

## Success Criteria
1. A plan is generated from this PRD
2. Plan contains exactly one task
3. Task creates file `test-echo.txt` with content "Echo successful"

## Core Features
1. Create a simple greeting file

## Technical Constraints
- Single file: `test-echo.txt`
- Content: "Echo successful"
- No dependencies
- Must complete in one shot

## Dependencies
None.

## Out of Scope
- Multiple files
- Complex logic
- Tests (this is a simple smoke test)
