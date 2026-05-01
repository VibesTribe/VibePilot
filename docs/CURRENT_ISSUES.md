# VibePilot Current Issues
> Last updated: April 30, 2026 (verified against actual code, DB, and running system)
> Previous: April 29, 2026 (cost tracking + ROI dashboard fixes)

## Status Summary

| Category | Open | Fixed | Deferred |
|----------|------|-------|----------|
| Dashboard display | 0 | 13 (token/ROI fixes Apr 30) | 0 |
| Pipeline data | 0 | 8 | 0 |
| Pipeline events | 0 | 21+ | 0 |
| Race conditions | 0 | 2 (Apr 30) | 0 |
| Token tracking | 0 | 4 (Apr 30) | 0 |
| Model management | 0 | 3 (Apr 30) | 0 |
| Research/council | 0 | 2 | 2 |

---

## Fixed Issues (April 30, 2026 — Race conditions, tokens, model management)

### Race Condition: Task Double-Dispatch — FIXED
**Priority**: P0 → FIXED
**What happened**: Two task_dispatched events for same task, 86ms apart. Same model dispatched twice.
**Root cause**: `claim_task` atomically sets `status=in_progress, processing_by=worker_id`. Then `handleTaskAvailable` called `transition_task` which reset `processing_by=NULL`. Second event arrived, `claim_task` saw `processing_by IS NULL`, claimed again.
**Fix**: Removed redundant `transition_task` call after `claim_task`. The claim already handles status and assignment atomically.
**Files**: governor/cmd/governor/handlers_task.go
**Commit**: 66d4d373

### PRD Webhook Flood (66 duplicate events) — FIXED
**Priority**: P0 → FIXED
**What happened**: Force-push/rebase caused git to list all old PRD files as "added". 66 duplicate prd_committed events fired.
**Root cause**: `prdExists` only checked `plans` table. Old test PRDs never had plans, so all passed through.
**Fix**: `prdExists` now also checks `orchestrator_events` for existing `prd_committed` events. Dedup at the source.
**Files**: governor/internal/webhooks/github.go
**Commit**: 66d4d373

### Dashboard: Token Count Only Showed Run Tokens — FIXED
**Priority**: P1 → FIXED
**What happened**: Header "Now 2,667" only counted executor run tokens. Ignored task-level totals and subscription tokens.
**Fix**: `calculateMetrics` now takes `max(runTokens, taskTokens)`. VibePilotTask interface updated with `total_tokens_in/out` fields.
**Files**: vibeflow/apps/dashboard/lib/vibepilotAdapter.ts
**Commit**: 2b973a432

### Dashboard: ROI Totals Excluded Subscription Savings — FIXED
**Priority**: P1 → FIXED
**What happened**: Header ROI showed $0. `roi.totals` only summed pipeline run costs, not subscription savings.
**Fix**: Totals now include subscription data: `total_savings_usd += (api_equivalent - prorated)` for all subscriptions.
**Files**: vibeflow/apps/dashboard/lib/vibepilotAdapter.ts
**Commit**: 577d4f8f5

### Dashboard: Plan-Stage Runs Orphaned from Tasks — FIXED
**Priority**: P1 → FIXED
**What happened**: Planner/plan_reviewer runs recorded with planID as task_id. Dashboard adapter only looked up by task ID. Those tokens were invisible.
**Fix**: Adapter builds `planToTask` map from `tasks.plan_id`. Plan-stage runs get attributed to the correct task.
**Files**: vibeflow/apps/dashboard/lib/vibepilotAdapter.ts
**Commit**: 82c0e1103

### Plan-Level Supervisor Tokens Never Recorded — FIXED
**Priority**: P1 → FIXED
**What happened**: Plan review supervisor ran model calls but never wrote tokens to `task_runs`. Tokens lost.
**Fix**: Added `record_internal_run` call with role="plan_reviewer" after plan supervisor succeeds.
**Files**: governor/cmd/governor/handlers_plan.go
**Commit**: 66d4d373

### Models Config: Nemotron Still Present — FIXED
**Priority**: P1 → FIXED
**What happened**: 6 Nemotron models in models.json even after removing from routing. Governor could route to them.
**Fix**: Removed all 6 Nemotron + 2 dead Ling models. Fixed providers (Groq-native models had provider="meta"). Added missing rate limits.
**Files**: governor/config/models.json (58 models, no Nemotron)
**Commit**: 484cf5ea

### Daily Model Health Check — NEW
**Priority**: P1 → DONE
**What it does**: Daily cron at 6 AM. Checks Gemini/Groq/OpenRouter APIs. Health-checks all cascade models. Updates rate limits from provider data. Reports new free models. Writes health_report.json.
**Files**: governor/scripts/daily_model_health.py
**Commit**: 484cf5ea

---

## Fixed Issues (April 29, 2026)

### E2E Pipeline Test — PASSED
**Priority**: P0 → DONE
**What happened**: PRD `e2e-hello-world.md` pushed. Full autonomous pipeline completed: planner → supervisor → dispatch → executor → supervisor review → testing → merge. 12 orchestrator_events recorded. Analyst agent wired for stuck tasks.
**Commit**: 22212860

### Dashboard Fixes (Apr 29, 10 fixes)
- Owner mapping: use raw `assigned_to` without `agent.` prefix
- Token counting: sum ALL runs per task, not just latest
- Log popup CSS: reduced pseudo-elements to 88px
- Event sorting via useMemo
**Commits**: d2a791e30, 11e6b9818, e6c644471 through f75d8f9c2

### Cost Tracking 4-Phase Overhaul (Apr 29)
- Phase 1: subscription_history table, task_runs cost columns
- Phase 2: record_internal_run for planner/supervisor/analyst
- Phase 3: Dashboard ROI panel overhaul
- Phase 4: Alerting for subscription thresholds
**Commits**: 6d37581f, e22f1e99, 7613e19a7

---

## Fixed Issues (April 28, 2026 — gap analysis, 8 fixes)

| # | Issue | Fix |
|---|-------|-----|
| 1 | 13 RPCs called in Go but missing from allowlist | Added all 13 to rpc.go |
| 2 | Dependency chain: 'available'/'locked' statuses rejected | Migration 130 added to CHECK constraint |
| 3 | get_change_approvals and queue_maintenance_command didn't exist | Migration 131 created them |
| 4 | commitOutput errors silently discarded | Changed to log warning |
| 5 | Supervisor review timeout hardcoded | Configurable from system.json |
| 6 | Webhook "complete" mapped to dead event | Fixed mapping to EventTaskApproval |
| 7 | Binary stale after fixes | Rebuilt + restarted |
| 8 | CURRENT_ISSUES.md outdated | Updated docs |

---

## Deferred Issues

### Research Flow — DEFERRED
**Priority**: P2 (blocked on knowledgebase)
**Status**: Knowledgebase repo exists but not operational. Researcher agent not built yet.

### Council for Research — DEFERRED
**Priority**: P2 (blocked on knowledgebase)

---

## Token Tracking Status (as of April 30)

All pipeline stages now record tokens to task_runs via `record_internal_run`:

| Stage | role in task_runs | Status |
|-------|------------------|--------|
| Planner | `planner` | Records via record_internal_run |
| Plan Reviewer | `plan_reviewer` | Records via record_internal_run |
| Executor | `executor` | Courier creates run, updates with tokens |
| Task Supervisor | `supervisor` | Records via record_internal_run |
| Analyst | `analyst` | Records via record_internal_run |
| Council | — | Not tracked yet (future stage) |
| Consultant | `consultant` | Wired, ready when consultant agent exists |
| Researcher | — | Future stage |

Token lifecycle per task: planner + plan_reviewer (via plan_id) + executor + supervisor (per attempt, accumulates on retry). Adapter maps plan_id → task_id.

---

## Build Priority (REMAINING)

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P1 | Courier agent testing (browser-based execution) | Medium | Test browser-use courier dispatch |
| P2 | Research agent + knowledgebase | Medium | Build researcher, wire knowledgebase |
| P3 | Stale Supabase-era prompts in DB | Trivial | DELETE FROM prompts |
| P3 | Dead code cleanup | Low | Remove old Python orchestrator refs |
