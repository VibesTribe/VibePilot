# VibePilot Current Issues
> Last updated: April 27, 2026 (verified + fixed against actual code and DB)
> Previous: April 27, 2026

## Status Summary

| Category | Open | Partially Fixed | Fixed | Deferred |
|----------|------|-----------------|-------|----------|
| Database migration | 0 | 0 | 5 | 0 |
| SSE bridge | 0 | 0 | 3 | 0 |
| Infrastructure | 0 | 0 | 3 | 0 |
| Pipeline gaps | 0 | 2 | 4 | 0 |
| Learning system | 0 | 0 | 4 | 0 |
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
**Impact**: `BuildPlannerContext()` IS wired in session.go:164 for agents with `context_policy: full_map` (planner has this in agents.json). Injects: incomplete slices, learned rules, recent failures, MCP tools, AND the full code map from `.context/map.md` (context_builder.go:231-237). The code map is auto-regenerated on git checkout via post-checkout hook.
**What's done**:
  - `BuildPlannerContext()` called for `full_map` policy agents (planner)
  - Queries `get_slice_task_info`, `get_supervisor_rules`, `get_problem_solution`, `get_heuristic`
  - Includes MCP tool listings
  - Includes full `.context/map.md` (64KB, 1281 lines of compressed function signatures)
  - `.context/` auto-regenerated via git post-checkout hook (fires on git pull, git reset)
**What's untested**:
  - E2E verification that planner uses the code map correctly
**Files**: context_builder.go:157-237, session.go:155-175, .git/hooks/post-checkout
**Verified**: 2026-04-27 against Go code

---

## Fixed Issues (April 27, 2026 — commit 0f65f686)

### 4. Module-Level Integration Test — FIXED
**Priority**: P2 → FIXED
**What was done**: Added `runModuleIntegrationTest()` to MaintenanceHandler. Before merging module branch to testing, runs `go build ./...` on the module branch. If build fails, creates maintenance command instead of merging broken code. Records `module_integration_test` pipeline event.
**Files**: handlers_maint.go:580-632

### 5. Supervisor Rules Not Created from Rejections — FIXED
**Priority**: P3 → FIXED
**What was done**: Added `record_supervisor_rule` call in both "fail" and "needs_revision" cases of `handleTaskReview()`. Also added `create_rule_from_rejection` call in "needs_revision" case to create planner learning rules from rejection patterns.
**Files**: handlers_task.go (fail case ~line 1090, needs_revision case ~line 1148)

### 6. Tester Rules Never Created — FIXED
**Priority**: P3 → FIXED
**What was done**: Created `create_tester_rule` DB function (was whitelisted in Go but never existed in DB). Wired call in `handleTaskTesting()` test failure path. Creates rule from test output pattern so future runs can catch similar issues.
**DB function**: `create_tester_rule(p_applies_to, p_test_type, p_test_command, p_trigger_pattern, ...)`
**Files**: handlers_testing.go ~line 207

### 7. Heuristics Never Recorded from Task Outcomes — FIXED
**Priority**: P3 → FIXED
**What was done**: Created `upsert_heuristic` DB function. Wired call in `recordSuccess()` — on every task success, records that the model succeeded at this task type, boosting confidence for future routing.
**DB function**: `upsert_heuristic(p_task_type, p_condition, p_action, p_preferred_model, ...)`
**Files**: handlers_task.go:1419-1427

### 8. Problem-Solutions Never Recorded — FIXED
**Priority**: P3 → FIXED
**What was done**: Wired `record_solution_on_success` call in `recordSuccess()`. When a task succeeds after previous failures, records the model that solved it as a solution for that failure pattern.
**Files**: handlers_task.go:1428-1433

### 9. Code Map Staleness — FIXED
**Priority**: Infrastructure → FIXED
**What was done**: Added git post-checkout hook that runs `.context/build.sh` in background when HEAD changes. Also regenerated map.md from commit 952e2898 to current 0f65f686.
**Files**: .git/hooks/post-checkout

### 10. Missing DB Functions — FIXED
**Priority**: Infrastructure → FIXED
**What was done**: Created 3 DB functions that were whitelisted in Go rpc.go but never existed in PostgreSQL:
- `create_tester_rule()` — idempotent, deduplicates by trigger_pattern + applies_to
- `record_tester_rule_hit()` — increments caught_bugs or false_positives
- `upsert_heuristic()` — creates new or boosts confidence on existing

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
| 13 | Learning RPCs never called | All 5 learning RPCs wired into handlers | 0f65f686 |
| 14 | 3 DB functions missing | create_tester_rule, record_tester_rule_hit, upsert_heuristic | 0f65f686 |
| 15 | No module integration test | go build gate before module-to-testing merge | 0f65f686 |
| 16 | Code map goes stale | git post-checkout hook auto-regenerates | 0f65f686 |

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup — graceful fallback to existing map.md

---

## Fix Priority Order (REMAINING only)

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P0 | E2E test to verify pipeline | Low | Push PRD, monitor |
| P1 | ROI popup revisions | Low | User specs |
| P2 | Task context E2E verification | Low | Run E2E, check if planner outputs target_files |
