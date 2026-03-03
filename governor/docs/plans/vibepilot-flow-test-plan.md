# PLAN: VibePilot Flow Test

## Overview
Test the VibePilot planning and execution flow with a minimal feature. This validates that the planner receives PRDs, creates atomic tasks, and the executor can run them successfully.

## Tasks

### T001: Create Flow Test PRD
**Confidence:** 0.99
**Category:** setup
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create Flow Test PRD

## Context
The PRD file referenced by the planner doesn't exist yet. Create it to establish the feature requirements for testing the VibePilot flow.

## What to Build
Create a markdown file at `docs/prd/vibepilot-flow-test.md` with the following content:

```markdown
# PRD: VibePilot Flow Test

## Objective
Verify the VibePilot planning and execution pipeline works end-to-end with a minimal, verifiable feature.

## Feature
Add a simple `/ready` endpoint to the governor that returns a JSON response confirming the system is operational.

## Requirements
1. Endpoint at GET `/ready`
2. Returns JSON: `{"ready": true, "service": "governor"}`
3. Returns HTTP 200 status
4. No external dependencies required

## Acceptance Criteria
1. Endpoint responds correctly
2. Response is valid JSON
3. Response time under 50ms
4. Unit tests pass
```

## Files
- `docs/prd/vibepilot-flow-test.md` - The PRD document

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["docs/prd/vibepilot-flow-test.md"],
  "tests_written": []
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["docs/prd/vibepilot-flow-test.md"],
  "tests_written": []
}
```

---

### T002: Add Ready Endpoint Handler
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```markdown
# TASK: T002 - Add Ready Endpoint Handler

## Context
The governor needs a simple ready endpoint to verify the system is operational. This is used by the VibePilot flow test to confirm execution works.

## What to Build
Add a `/ready` HTTP endpoint to the governor that returns:
```json
{
  "ready": true,
  "service": "governor"
}
```

Implementation details:
1. Create new file `governor/internal/api/ready.go`
2. Add `ReadyHandler` function that returns the JSON response
3. Return status code 200
4. Content-Type: application/json
5. Register the endpoint in main.go at `/ready`

## Files
- `governor/internal/api/ready.go` - Ready endpoint handler
- `governor/cmd/governor/main.go` - Register the endpoint

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/internal/api/ready.go"],
  "files_modified": ["governor/cmd/governor/main.go"],
  "tests_written": []
}
```
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/internal/api/ready.go"],
  "files_modified": ["governor/cmd/governor/main.go"],
  "tests_written": []
}
```

---

### T003: Write Ready Endpoint Tests
**Confidence:** 0.97
**Category:** testing
**Dependencies:** T002

#### Prompt Packet
```markdown
# TASK: T003 - Write Ready Endpoint Tests

## Context
Verify the ready endpoint works correctly and meets the acceptance criteria.

## What to Build
Write unit tests for the ready endpoint that verify:
1. Returns status code 200
2. Response contains "ready" field with value true
3. Response contains "service" field with value "governor"
4. Response is valid JSON
5. Response time under 50ms

Use Go's standard testing package and httptest for HTTP testing.

## Files
- `governor/internal/api/ready_test.go` - Ready endpoint tests

## Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["governor/internal/api/ready_test.go"],
  "tests_written": ["governor/internal/api/ready_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["governor/internal/api/ready_test.go"],
  "tests_written": ["governor/internal/api/ready_test.go"]
}
```

---

### T004: Run Tests and Verify Flow
**Confidence:** 0.96
**Category:** validation
**Dependencies:** T003

#### Prompt Packet
```markdown
# TASK: T004 - Run Tests and Verify Flow

## Context
Execute the tests to confirm the implementation works and the VibePilot flow completed successfully.

## What to Build
Run the test suite and verify:
1. All tests pass
2. Ready endpoint responds correctly
3. Output verification summary

Execute: `go test ./governor/internal/api/... -v`

## Files
- No new files - verification task

## Expected Output
```json
{
  "task_id": "T004",
  "files_created": [],
  "tests_written": [],
  "tests_passed": true,
  "flow_verified": true
}
```
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": [],
  "tests_written": [],
  "tests_passed": true,
  "flow_verified": true
}
```
