# VibePilot Current Issues
> Last updated: April 27, 2026 (verified + fixed against actual code and DB)
> Previous: April 27, 2026

## Status Summary

| Category | Open | Partially Fixed | Fixed | Deferred |
|----------|------|-----------------|-------|----------|
| Database migration | 0 | 0 | 5 | 0 |
| SSE bridge | 0 | 0 | 3 | 0 |
| Infrastructure | 0 | 0 | 5 | 0 |
| Pipeline gaps | 0 | 0 | 7 | 0 |
| Learning system | 0 | 0 | 4 | 0 |
| Model health | 0 | 0 | 1 | 0 |
| Dashboard | 1 | 0 | 0 | 0 |

---

## Dashboard Issues

### 1. ROI Popup Needs Work
**Priority**: P3 (after E2E verified)
**Impact**: ProjectTracker and SessionTracker deployed but user says "it needs work"
**Status**: Waiting for user specifications after E2E test
**Files**: MissionModals.tsx

---

## Fixed Issues (April 27, 2026 — commit 133cd28a)

### 2. Task Packets — Target Files — FIXED
**Priority**: P2 → FIXED
**What was the problem**: Planner prompt never asked the model to include "Target Files" in task output, so validation.go always parsed empty TargetFiles. Task agents got zero code context.
**What was done**: Added "Target Files" as required field in planner prompt task template, example output, and constraints. Now every task must list the files it will create/modify, feeding BuildTargetedContext().
**Files**: prompts/planner.md, cmd/governor/validation.go (already parses **Target Files:**)

### 3. Planner — Context FULLY WIRED
**Priority**: P2 → FIXED (was already wired, just untested)
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

## Fixed Issues (April 27, 2026 — commit 133cd28a)

### 11. Maintenance Commands Never Processed — FIXED
**Priority**: P0 → FIXED (showstopper)
**What was the problem**: maintenance_commands and research_suggestions tables had no pg_notify triggers. When handlers created maintenance commands (e.g., for merge conflicts), no event fired, so handleMaintenanceCommand never ran. Merge conflicts sat forever.
**What was done**:
  - Added `notify_maintenance_commands` trigger on maintenance_commands table
  - Added `notify_research_suggestions` trigger on research_suggestions table
  - Filtered listener.go to only fire EventMaintenanceCmd for status=pending (prevents infinite re-trigger after completion)
  - Added handler guard in handlers_maint.go against non-pending commands
**Files**: listener.go, handlers_maint.go, DB triggers

### 12. Plan Revisions Never Re-Triggered — FIXED
**Priority**: P0 → FIXED (showstopper)
**What was the problem**: When supervisor rejected a plan (status → revision_needed), EventRevisionNeeded fired but no handler was registered. Rejected plans sat in revision_needed forever.
**What was done**: Added handlePlanRevisionNeeded handler that:
  - Re-runs planner with supervisor feedback (latest_feedback, tasks_needing_revision)
  - Includes existing plan content so planner revises rather than starts from scratch
  - Increments revision_round via increment_revision_round RPC
  - Max 3 revision rounds, then sets plan to "error"
  - On success, sets plan to "review" (re-triggers supervisor re-review)
  - Full cascade retry (5 attempts) with model health tracking
**Files**: handlers_plan.go, setupPlanHandlers registration

---

## Fixed Issues (April 27, 2026 — commit 0f65f686)

### 3.5. Cooldown Expiry Re-verification — FIXED
**Priority**: Infrastructure → FIXED
**What was the problem**: When a model's cooldown timer expired (e.g., after a rate limit), the router blindly assumed "timer done = model fine." A model with a dead API key or a deprecated endpoint would cycle forever: cooldown expires → router tries → fails → new cooldown → repeat.
**What was done**: Added `CooldownWatcher` — a background goroutine that polls every 2 minutes for models whose cooldown recently expired, probes each via its connector's HealthCheck(), and either confirms healthy or extends cooldown. Tracks which expirys have been probed to avoid re-checking. Staggers probes (2s between) to avoid hitting rate limits. Persists state to DB after probe failures.
**Files**: runtime/cooldown_watcher.go, cmd/governor/main.go (wired after LoadFromDatabase)

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
| 11 | Task packets had zero codebase context | ContextBuilder wired, planner prompt requires Target Files | 133cd28a |
| 12 | Planner had no codebase context | BuildPlannerContext wired for full_map policy | 0f65f686 |
| 13 | Learning RPCs never called | All 5 learning RPCs wired into handlers | 0f65f686 |
| 14 | 3 DB functions missing | create_tester_rule, record_tester_rule_hit, upsert_heuristic | 0f65f686 |
| 15 | No module integration test | go build gate before module-to-testing merge | 0f65f686 |
| 16 | Code map goes stale | git post-checkout hook auto-regenerates | 0f65f686 |
| 17 | Maintenance commands never processed | Added pg_notify triggers + status filter | 133cd28a |
| 18 | Plan revisions never re-triggered | handlePlanRevisionNeeded handler with max 3 rounds | 133cd28a |

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup — graceful fallback to existing map.md

---

## Fix Priority Order (REMAINING only)

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P0 | E2E test to verify pipeline | Low | Push PRD, monitor |
| P1 | ROI popup revisions | Low | User specs |
| P2 | Task context E2E verification | Low | Run E2E, check if planner outputs target_files |
