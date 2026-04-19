# Test: Dependency Chain Validation

## Summary
Add a simple `/ping` endpoint to the VibePilot API that returns a JSON response, then add a test that verifies it works. This is a two-task PRD to validate that task dependencies work correctly: Task 2 MUST NOT start until Task 1 is complete.

## Requirements
1. **Task 1 (T001)**: Create a new HTTP handler at `GET /api/ping` that returns `{"status":"ok","timestamp":"<ISO8601>"}`. Add the route to the existing API server setup. This is a standalone task with no dependencies.

2. **Task 2 (T002)**: Write a Go test file that makes an HTTP request to `/api/ping` and verifies the response has status "ok" and a valid timestamp. This task DEPENDS on T001 — the handler must exist before tests can be written.

## Constraints
- Exactly 2 tasks, no more, no less
- T001 has no dependencies and should be status "available" immediately
- T002 depends on T001 and should be status "pending" until T001 completes
- Both tasks are internal (require codebase access)
- Target repository: VibePilot itself (dogfooding)
- Slice: general

## Expected Behavior
When this plan is approved:
- T001 should appear in the task list as "available"
- T002 should appear as "pending" with dependencies = ["T001"]
- T002 should NOT be picked up by any worker until T001 status = "completed"
- After T001 completes, T002 should automatically become "available"

## Acceptance Criteria
- [ ] `/api/ping` endpoint returns correct JSON
- [ ] Test file exists and passes
- [ ] Dependency locking is confirmed: T002 was never in "available" state before T001 was "completed"
