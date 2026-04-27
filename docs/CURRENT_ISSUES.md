# VibePilot Current Issues
> Last updated: April 27, 2026
> Previous: April 23, 2026

## Status Summary

| Category | Open | Fixed | Deferred |
|----------|------|-------|----------|
| Database migration | 0 | 5 | 0 |
| SSE bridge | 0 | 3 | 0 |
| Infrastructure | 0 | 3 | 0 |
| Pipeline gaps | 3 | 5 | 0 |
| Learning system | 4 | 0 | 0 |
| Dashboard | 1 | 0 | 0 |

---

## Dashboard Issues

### 1. ROI Popup Needs Work
**Priority**: P3 (after E2E verified)
**Impact**: ProjectTracker and SessionTracker deployed but user says "it needs work"
**Status**: Waiting for user specifications after E2E test
**Files**: MissionModals.tsx

---

## Pipeline Gaps (remaining)

### 2. No Module-Level Integration Test
**Priority**: P2
**Impact**: After all tasks in a module pass individual testing, module merges to testing branch without running a module-level integration test
**Fix**: Add module integration test step before tryMergeModuleToTesting
**Files**: handlers_testing.go, handlers_maint.go

### 3. Task Packets Have Zero Codebase Context
**Priority**: P2
**Impact**: Executor gets prompt text only, no actual file contents. Models produce generic/ungrounded output.
**Fix**: Inject relevant file contents into task packets before execution
**Files**: validation.go, handlers_task.go

### 4. Planner Has No Codebase Context
**Priority**: P2
**Impact**: Planner invents file paths and patterns without seeing the repo
**Fix**: Give planner a file tree + key file contents
**Files**: handlers_plan.go

---

## Learning System Gaps (pre-existing)

### 5. Supervisor Rules Not Created from Rejections
**Location**: `governor/cmd/governor/handlers_task.go`
**Impact**: System doesn't learn from supervisor rejections
**Fix**: Call `create_supervisor_rule` RPC in rejection handler

### 6. Tester Rules Never Created
**Location**: `governor/cmd/governor/handlers_testing.go`
**Impact**: No learning from test failures
**Fix**: Call `create_tester_rule` RPC on test failure

### 7. Heuristics Never Recorded from Task Outcomes
**Impact**: Router doesn't learn model preferences per task type
**Fix**: Call `upsert_heuristic` RPC on task success
**Note**: `learned_heuristics` table has 20 entries (from direct DB inserts), but handler doesn't automatically create them

### 8. Problem-Solutions Never Recorded
**Impact**: Same failures repeat, no automatic remediation
**Fix**: Call `record_solution_result` RPC on retry success

---

## Fixed Since Last Update (April 23-27, 2026)

| # | Issue | Fix | Commit |
|---|-------|-----|--------|
| 1 | Pipeline events showed raw types (broken_output) | 26 semantic event types with human-readable labels | a08afe74+ |
| 2 | Module branches never deleted after merge | Added DeleteBranch after module-to-testing merge | 16d9724a |
| 3 | Testing branch never deleted after merge | Added DeleteBranch after testing-to-main merge | 16d9724a |
| 4 | Module merge failures had no recovery | Maintenance agent dispatches on module merge failure | 16d9724a |
| 5 | Integration merge failures had no recovery | Maintenance agent dispatches on integration merge failure | 16d9724a |
| 6 | Task merge conflicts reassigned to model | Maintenance agent handles merge conflicts, no model penalty | pre-existing |
| 7 | SSE E2E tested | Working with live dashboard | Apr 24 |
| 8 | Courier writes to local PG | governor API endpoint replaces Supabase | Apr 23 |
| 9 | Dashboard polling | SSE EventSource replaces polling | Apr 23 |
| 10 | Pipeline timeline shows meaningless events | Full 26-event lifecycle with readable labels | Apr 27 |

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup — graceful fallback to existing map.md

---

## Fix Priority Order

| Priority | Issue | Effort | Files |
|----------|-------|--------|-------|
| P0 | E2E test to verify pipeline | Low | Push PRD, monitor |
| P1 | ROI popup revisions | Low | MissionModals.tsx |
| P2 | Module integration test | Medium | handlers_testing.go |
| P2 | Task packet codebase context | Medium | validation.go, handlers_task.go |
| P2 | Planner codebase context | Medium | handlers_plan.go |
| P3 | Supervisor rule creation | Low | handlers_task.go |
| P3 | Tester rule creation | Low | handlers_testing.go |
| P3 | Heuristic recording | Low | handlers_task.go |
