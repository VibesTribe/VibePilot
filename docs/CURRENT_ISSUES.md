# VibePilot Current Issues
> Last updated: April 27, 2026 (verified against actual code and DB)
> Previous: April 27, 2026

## Status Summary

| Category | Open | Partially Fixed | Fixed | Deferred |
|----------|------|-----------------|-------|----------|
| Database migration | 0 | 0 | 5 | 0 |
| SSE bridge | 0 | 0 | 3 | 0 |
| Infrastructure | 0 | 0 | 3 | 0 |
| Pipeline gaps | 0 | 2 | 3 | 0 |
| Learning system | 4 | 0 | 0 | 0 |
| Dashboard | 1 | 0 | 0 | 0 |

---

## Dashboard Issues

### 1. ROI Popup Needs Work
**Priority**: P3 (after E2E verified)
**Impact**: ProjectTracker and SessionTracker deployed but user says "it needs work"
**Status**: Waiting for user specifications after E2E test
**Files**: MissionModals.tsx

---

## Pipeline Gaps (partially fixed)

### 2. Task Packets — Context PARTIALLY WIRED
**Priority**: P2
**Impact**: contextBuilder IS wired (handlers_task.go:177, session.go:164). Reads `target_files` from planner result JSONB and injects file contents via `BuildTargetedContext()`. BUT: depends on planner outputting `target_files` array, which has never been tested E2E.
**What's done**:
  - `ContextBuilder` injected in main.go:199, wired to TaskHandler via SetContextBuilder
  - `BuildTargetedContext()` reads actual file contents and appends to prompt
  - `BuildBaseContext()` provides file tree for `file_tree` policy
**What's untested**:
  - Does the planner actually return `target_files` in its structured output?
  - Does the file content injection actually improve executor output quality?
**Files**: handlers_task.go:162-183, context_builder.go:142-155, session.go:155-175
**Verified**: 2026-04-27 against Go code

### 3. Planner — Context PARTIALLY WIRED
**Priority**: P2
**Impact**: `BuildPlannerContext()` IS wired in session.go:164 for agents with `context_policy: full_map` (planner has this in agents.json). Injects: incomplete slices, learned rules, recent failures, MCP tools. BUT: does NOT inject actual file tree or codebase contents — only metadata about existing slices/rules/failures.
**What's done**:
  - `BuildPlannerContext()` called for `full_map` policy agents (planner)
  - Queries `get_slice_task_info`, `get_supervisor_rules`, `get_problem_solution`, `get_heuristic`
  - Includes MCP tool listings
**What's missing**:
  - No file tree injection (planner doesn't see the repo structure)
  - No file contents (planner doesn't see actual code)
  - Planner can still invent file paths that don't exist
**Files**: context_builder.go:157-230, session.go:155-175
**Verified**: 2026-04-27 against Go code

### 4. No Module-Level Integration Test
**Priority**: P2
**Impact**: After all tasks in a module pass individual testing, module merges to testing branch without running a module-level integration test
**Fix**: Add module integration test step before tryMergeModuleToTesting
**Files**: handlers_testing.go, handlers_maint.go
**Verified**: 2026-04-27 — grep confirms no module integration test code exists

---

## Learning System Gaps (all same root cause)

> All 4 issues share the same pattern: RPC functions exist in PostgreSQL and are
> whitelisted in Go (rpc.go), but no handler actually CALLS them at the right moment.

### 5. Supervisor Rules Not Created from Rejections
**Priority**: P3
**Location**: `governor/cmd/governor/handlers_task.go`
**Impact**: System doesn't learn from supervisor rejections. `supervisor_learned_rules` table has 42 rows (from direct DB inserts), but `record_supervisor_rule` RPC is never called from any handler.
**Fix**: Call `record_supervisor_rule` RPC in the supervisor rejection path
**RPC exists**: YES — `rpc.go:64`, `create_rule_from_rejection` also exists but unused
**Verified**: 2026-04-27 — grep confirms zero handler calls

### 6. Tester Rules Never Created
**Priority**: P3
**Location**: `governor/cmd/governor/handlers_testing.go`
**Impact**: No learning from test failures. `get_tester_rules` IS called (context_builder.go:297 for context injection), but `create_tester_rule` RPC is never called from any handler.
**Fix**: Call `create_tester_rule` RPC on test failure
**RPC exists**: YES — `rpc.go:68`
**Verified**: 2026-04-27 — grep confirms zero handler calls

### 7. Heuristics Never Recorded from Task Outcomes
**Priority**: P3
**Impact**: Router doesn't learn model preferences per task type. `get_heuristic` IS called (context_builder.go:316 for routing context), but `upsert_heuristic` RPC is never called from any handler.
**Fix**: Call `upsert_heuristic` RPC on task success
**RPC exists**: YES — `rpc.go:100`
**Verified**: 2026-04-27 — grep confirms zero handler calls
**Note**: `learned_heuristics` table has 0 rows

### 8. Problem-Solutions Never Recorded
**Priority**: P3
**Impact**: Same failures repeat, no automatic remediation. `get_problem_solution` IS called (context_builder.go:336 for context injection), but `record_solution_on_success` RPC is never called from any handler.
**Fix**: Call `record_solution_on_success` RPC on retry success
**RPC exists**: YES — `rpc.go:105`
**Verified**: 2026-04-27 — grep confirms zero handler calls

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
| 11 | Task packets had zero codebase context | ContextBuilder wired, reads target_files from planner | uncommitted |
| 12 | Planner had no codebase context | BuildPlannerContext wired for full_map policy | uncommitted |

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup — graceful fallback to existing map.md

---

## Fix Priority Order

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P0 | E2E test to verify pipeline | Low | Push PRD, monitor |
| P1 | ROI popup revisions | Low | User specs |
| P2 | Task context E2E verification | Low | Run E2E, check if planner outputs target_files |
| P2 | Planner file tree injection | Medium | Add file tree to BuildPlannerContext |
| P2 | Module integration test | Medium | handlers_testing.go |
| P3 | Supervisor rule creation | Low | 1 RPC call in rejection handler |
| P3 | Tester rule creation | Low | 1 RPC call in test failure handler |
| P3 | Heuristic recording | Low | 1 RPC call on task success |
| P3 | Solution recording | Low | 1 RPC call on retry success |
