# VibePilot Current Issues
> Last updated: April 28, 2026 (verified against actual code, DB, and running system)
> Previous: April 28, 2026 (gap analysis fixes deployed)

## Status Summary

| Category | Open | Fixed | Deferred |
|----------|------|-------|----------|
| Dashboard | 1 | 0 | 0 |
| Pipeline data | 0 | 6 | 0 |
| Pipeline events | 0 | 21+ | 0 |
| Gap analysis | 0 | 8 | 0 |
| Research/council | 0 | 2 | 2 |

---

## Fixed Issues (April 28, 2026 — gap analysis, 8 fixes deployed and verified)

### Gap Analysis — 8 fixes deployed against running system

| # | Priority | Issue | Root Cause | Fix |
|---|----------|-------|------------|-----|
| 1 | CRITICAL | 13 RPCs called in Go but missing from allowlist | rpc.go had 91 entries, Go code calls 13 more | Added all 13: add_bookmark, calc_run_costs, create_rule_from_rejection, get_change_approvals, get_failure_patterns, get_model_performance, get_slice_task_info, queue_maintenance_command, recall_memories, record_planner_rule_prevented_issue, store_memory, update_maintenance_command_status, update_model_learning. Allowlist now 101 entries. |
| 2 | CRITICAL | Dependency chain: 'available'/'locked' statuses rejected by DB | tasks CHECK constraint only had original statuses. unlock_dependent_tasks looked for 'pending' only | Migration 130: added 'available' and 'locked' to CHECK. Recreated unlock_dependent_tasks/unlock_dependents to look for `status IN ('locked','pending')` and set to 'available'. validation.go: zero-dep tasks start as 'available', dep tasks as 'pending'. pgnotify listener: added "available" case → EventTaskAvailable |
| 3 | HIGH | get_change_approvals and queue_maintenance_command RPCs called but didn't exist in DB | Functions never created during Supabase→local migration | Migration 131: created get_change_approvals (stub, no table yet) and queue_maintenance_command (inserts into maintenance_commands) |
| 4 | HIGH | commitOutput errors silently discarded | Both call sites used `_, _ := h.commitOutput(...)` | Changed to capture error and log warning at both sites (internal task ~line 488, courier task ~line 665) |
| 5 | MEDIUM | Supervisor review timeout hardcoded at 2 minutes | Not configurable via system.json | Added ReviewTimeoutSeconds to ExecutionConfig in config.go. handlers_task.go reads from config with 2-minute fallback. system.json: `"review_timeout_seconds": 120` |
| 6 | MEDIUM | Webhook "complete" status mapped to dead event | mapToEventType returned EventTaskCompleted (no handler). pgnotify correctly used EventTaskApproval | Changed webhook mapping from EventTaskCompleted → EventTaskApproval. Now fires handleTaskApproved (merge flow) |
| 7 | INFRA | Binary stale after fixes | Source changed but binary not rebuilt | Rebuilt binary, killed old process, systemd Restart=always respawned with new binary. All health checks passing. |
| 8 | DOCS | CURRENT_STATE.md and CURRENT_ISSUES.md outdated | Docs reflected pre-fix state | Updated both docs with fix details, bug status, remaining gaps. Pushed to GitHub. |

**Verification**: All 13 new RPCs confirmed in binary via `strings`. CHECK constraint confirmed via direct psql query. DB functions confirmed via `pg_proc` query. Governor /status returns healthy. Dashboard /api/dashboard returns valid JSON.

**Original 10 bugs (Apr 25 → Apr 28 final status)**:
1. task_packets never written — FIXED (commit 61b1a3da)
2. commitOutput on main repo not worktree — FIXED
3. Supervisor timeout hardcoded — FIXED (Fix #5)
4. Testing can't find output in worktree — FIXED
5. Stale Supabase-era prompts in DB — STILL PRESENT (harmless)
6. commitOutput errors silently ignored — FIXED (Fix #4)
7. STATUS_ORDER missing human_review — FIXED
8. transition_task no status validation — FIXED
9. Duplicate task creation race — FIXED
10. Task stuck at review after max attempts — UNCLEAR

---

## Fixed Issues (April 28, 2026 — commit 61b1a3da)

### Pipeline Data Layer — 6 fixes deployed and verified

| # | Issue | Root Cause | Fix |
|---|-------|------------|-----|
| 1 | Prompt packet lost during execution | transition_task COALESCE replaced entire result JSONB | Changed to `result = COALESCE(result,'{}')::jsonb || COALESCE(p_result,'{}')::jsonb` (merge, not replace) |
| 2 | Prompt packet never in task_packets table | createTasksFromApprovedPlan stuffed prompt_packet into tasks.result JSONB only | Added INSERT into task_packets table after each task creation (survives result overwrites) |
| 3 | Plan handlers never wrote events | recordEvent was a TaskHandler-only method; plan handlers are standalone functions | Extracted standalone recordEvent; then unified to use existing recordPipelineEvent from pipeline_events.go |
| 4 | orchestrator_events.task_id was UUID | Plan-level events use planID (not UUID); INSERT failed | Changed column to TEXT: `ALTER TABLE orchestrator_events ALTER COLUMN task_id TYPE TEXT` |
| 5 | vp_notify_change trigger broke on tables without status column | Trigger blindly referenced OLD.status | Added guard: checks information_schema for status column before referencing |
| 6 | Build broken: APIRunner.HealthCheck + registerConnectors | HealthCheck referenced missing connectorID field; registerConnectors used undefined ctx | Fixed: HealthCheck uses provider/model/endpoint; added context.Background() |

**E2E proof**: Task d5823cd1 completed full quality harness loop: 3 attempts, 2 failures with supervisor feedback, 3rd attempt passed, tested, merged. 12 orchestrator_events recorded correctly.

**Event call sites verified**: 29 recordPipelineEvent calls across 5 handlers:
- Plan (6): planner_called, plan_created, supervisor_called, plan_approved, plan_rejected, council_review
- Task (7): task_dispatched, output_received, supervisor_called, run_completed, run_failed, revision_needed, reroute
- Testing (5): test_passed, test_failed, module_merge_failed, module_merged_to_testing, plan_complete
- Maintenance (7): merge_conflict_detected, task_merged_to_module, module_integration_test, module_merge_failed, module_merged_to_testing, integration_merge_failed, plan_complete
- Council (2): council_approved, council_feedback

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

## Fixed Issues (April 27, 2026 — commit 54e6eec0)

### 19. Council Context Never Provided — FIXED
**Priority**: P1 → FIXED
**What was the problem**: BuildCouncilContext existed in context_builder.go but was never called. Council agents had `context_policy: file_tree` which only gave them a bare file tree — no instructions to verify plan references, no verification guidance.
**What was done**: Added `council` context_policy case in session.go that calls BuildCouncilContext (with fallback to BuildBaseContext on error). Changed council agent's context_policy from `file_tree` to `council` in agents.json. Council members now get file tree + plan reference verification instructions.
**Files**: session.go, agents.json

### 20. "Blocked" Council Consensus Created Dead-End — FIXED
**Priority**: P1 → FIXED
**What was the problem**: When council consensus was "blocked", plan status was set to "blocked" and sat forever with no handler. Also `council_done` event type was defined/mapped but never emitted and had no handler — pure dead code (53 lines). `council_rejected` event name was misleading since nothing is rejected, only feedback given.
**What was done**: "blocked" consensus now routes to revision_needed with full council feedback (same path as revision_needed). Removed handleCouncilDone function, EventCouncilDone constant, and all council_done mappings from pgnotify listener, server.go, and handler registration. Renamed council_rejected → council_feedback in timeline events. Updated pipeline YAML to remove "blocked" vote option.
**Files**: handlers_council.go, events.go, listener.go, server.go, code-pipeline.yaml
**Commit**: 477b84be

---

## Deferred Issues (awaiting knowledgebase build)

### 21. Research Flow — DEFERRED
**Priority**: P2 (blocked on knowledgebase repo being operational)
**What's needed**: Researcher agent runs via GitHub Actions cron (2x daily), deposits reports to knowledgebase repo (VibesTribe/knowledgebase). Supervisor auto-approves simple model/platform additions. Council reviews complex ones. Feedback appended to report. Human reviews via knowledgebase link. Implementation in vibepilot task branches. All findings become institutional memory in Postgres.
**Why deferred**: Knowledgebase repo exists (11 commits) but not yet operational. Researcher agent hasn't run yet. Full flow requires knowledgebase schema, sources.txt, and dashboard DOCS button wiring.

### 22. Council for Research — DEFERRED
**Priority**: P2 (blocked on knowledgebase)
**What's needed**: Council reads research reports FROM knowledgebase repo, gives feedback per point, feedback appended to same doc, report+feedback goes to human via knowledgebase link. New research+feedback instantly added to knowledgebase.
**Why deferred**: Same blocker as #21.

---

## Fixed Issues (April 27, 2026 — commit 2384b572)

### 21. "Blocked" Eliminated System-Wide — FIXED
**Priority**: P1 → FIXED
**What was the problem**: "Blocked" consensus existed in council handler, research handler, consensus functions, council prompt, and pgnotify/server mappings. All created dead-end states where tasks/plans sat forever. Nothing should ever be blocked — feedback should always route to the right agent.
**What was done**: 
- determineConsensus in council and research handlers only returns approved/revision_needed
- "Blocked" vote in council prompt replaced with STRONG REVISION_NEEDED
- BLOCKED votes counted as revision_needed in vote alignment tracking
- Dead "blocked" pgnotify and server.go mappings removed
- Research handler "blocked→rejected" dead-end removed
- 5 dead event types removed (EventPlanBlocked, EventHumanQuery, EventPRDIncomplete, EventPlanError, EventCouncilComplete)
- EventTaskEscalated removed (nothing escalates to human for code)
- maxRetries hardcoded in 6 locations → config-driven via system.json
**Files**: handlers_council.go, handlers_research.go, handlers_task.go, handlers_plan.go, events.go, state.go, listener.go, server.go, config.go, system.json, council.md
**Commit**: 2384b572

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
| 19 | Council never got proper context | BuildCouncilContext wired via council policy | 54e6eec0 |
| 20 | "Blocked" council consensus = dead-end | Blocked routes to revision_needed, council_done dead code removed | 477b84be |
| 21 | "Blocked" everywhere + dead events + hardcoded maxRetries | System-wide elimination, 5 dead events removed, config-driven | 2384b572 |

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup — graceful fallback to existing map.md

---

## Fix Priority Order (REMAINING only)

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P0 | Next E2E test to verify all gap analysis fixes + Apr 28 data fixes | Low | Push PRD, monitor all 29 event types + learning RPCs |
| P1 | ROI popup revisions | Low | User specs |
| P2 | Task context E2E verification | Low | Run E2E, check if planner outputs target_files |
| P3 | Stale Supabase-era prompts in DB | Trivial | DELETE FROM prompts (governor reads filesystem) |
| P3 | Bug 10: No explicit terminal state on max review retries | Low | Add status='failed' after exhausting attempts |
