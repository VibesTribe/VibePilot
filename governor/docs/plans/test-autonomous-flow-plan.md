# PLAN: Test Autonomous Flow

## Overview
Test the complete autonomous execution flow from PRD to task completion. This plan validates that the planner, executor, and validation systems work together without human intervention.

## Tasks

### T001: Create Test PRD File
**Confidence:** 0.99
**Category:** setup
**Dependencies:** none

#### Prompt Packet
```markdown
# TASK: T001 - Create Test PRD File

## Context
A test PRD file is needed to validate the autonomous flow. This file should contain a simple, verifiable feature request.

## What to Build
Create a markdown file at `docs/prd/test-autonomous-flow.md` with the following content:

```markdown
# PRD: Test Autonomous Flow

## Objective
Verify the autonomous execution pipeline works end-to-end.

## Feature
Add a simple startup health check endpoint to the governor that returns:
- Status: "ok"
- Timestamp: current ISO timestamp
- Version: from build info

## Acceptance Criteria
1. Endpoint exists at `/health`
2. Returns JSON with status, timestamp, and version
3. Responds within 100ms
```

## Files
- `docs/prd/test-autonomous-flow.md` - The PRD document

## Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["docs/prd/test-autonomous-flow.md"],
  "tests_written": []
}
```
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": ["docs/prd/test-autonomous-flow.md"],
  "tests_written": []
}
```

---

### T002: Add Health Check Endpoint
**Confidence:** 0.97
**Category:** coding
**Dependencies:** T001

#### Prompt Packet
```markdown
# TASK: T002 - Add Health Check Endpoint

## Context
The governor needs a health check endpoint for monitoring and autonomous system verification.

## What to Build
Add a `/health` HTTP endpoint to the governor server that returns:
```json
{
  "status": "ok",
  "timestamp": "2026-03-03T12:00:00Z",
  "version": "1.0.0"
}
```

Implementation details:
1. Add endpoint handler in the HTTP server
2. Get version from build info (or default to "dev")
3. Use current UTC timestamp in ISO format
4. Return status code 200
5. Content-Type: application/json

## Files
- `governor/cmd/governor/main.go` - Add health endpoint registration
- `governor/internal/api/health.go` - Health check handler

## Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/internal/api/health.go"],
  "files_modified": ["governor/cmd/governor/main.go"],
  "tests_written": ["governor/internal/api/health_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["governor/internal/api/health.go"],
  "files_modified": ["governor/cmd/governor/main.go"],
  "tests_written": ["governor/internal/api/health_test.go"]
}
```

---

### T003: Write Health Check Tests
**Confidence:** 0.98
**Category:** testing
**Dependencies:** T002

#### Prompt Packet
```markdown
# TASK: T003 - Write Health Check Tests

## Context
Verify the health check endpoint works correctly and meets performance requirements.

## What to Build
Write unit tests for the health check endpoint that verify:
1. Returns status code 200
2. Response contains "status" field with value "ok"
3. Response contains "timestamp" field (valid ISO format)
4. Response contains "version" field (non-empty string)
5. Response time under 100ms

Use Go's standard testing package and httptest for HTTP testing.

## Files
- `governor/internal/api/health_test.go` - Health check tests

## Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["governor/internal/api/health_test.go"],
  "tests_written": ["governor/internal/api/health_test.go"]
}
```
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": ["governor/internal/api/health_test.go"],
  "tests_written": ["governor/internal/api/health_test.go"]
}
```

---

### T004: Verify Autonomous Flow Completion
**Confidence:** 0.96
**Category:** validation
**Dependencies:** T003

#### Prompt Packet
```markdown
# TASK: T004 - Verify Autonomous Flow Completion

## Context
Validate that all previous tasks completed successfully and the autonomous flow works end-to-end.

## What to Build
Create a verification script that:
1. Checks PRD file exists
2. Checks health endpoint implementation exists
3. Checks tests exist
4. Runs the tests and verifies they pass
5. Outputs a summary of autonomous flow execution

The script should output JSON with verification results.

## Files
- `scripts/verify-autonomous-flow.sh` - Verification script

## Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["scripts/verify-autonomous-flow.sh"],
  "tests_written": [],
  "verification_passed": true
}
```
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": ["scripts/verify-autonomous-flow.sh"],
  "tests_written": [],
  "verification_passed": true
}
```
