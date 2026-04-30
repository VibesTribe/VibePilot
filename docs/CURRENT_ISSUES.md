# VibePilot Current Issues
> Last updated: April 30, 2026 — 03:55 UTC
> Previous: April 30, 2026 (cost tracking + ROI dashboard fixes)

## Status Summary

| Category | Open | Fixed | Deferred |
|----------|------|-------|----------|
| Courier routing | 0 | 2 (Apr 30) | 0 |
| Config cleanup | 0 | 2 (Apr 30) | 0 |
| Race conditions | 0 | 2 (Apr 30) | 0 |
| Dashboard display | 0 | 13 (token/ROI fixes Apr 30) | 0 |
| Pipeline data | 0 | 8 | 0 |
| Pipeline events | 0 | 21+ | 0 |
| Token tracking | 0 | 4 (Apr 30) | 0 |
| Model management | 0 | 3 (Apr 30) | 0 |
| Research/council | 0 | 2 | 2 |

---

## Fixed Issues (April 30, 2026 — Config cleanup)

### routing.json Dead Model References — FIXED
**Priority**: P2 → FIXED
**What**: `routing.json` strategy `free_cascade` referenced `qwen/qwen3-coder-480b-a35b:free` (actual ID: `qwen/qwen3-coder:free`) and `minimax/m2.5:free` (actual ID: `minimax/minimax-m2.5`).
**Effect**: Governor started in DEGRADED mode with 2 validation errors.
**Fix**: Corrected both model IDs to match models.json.
**Commit**: 130746cc

### Stale Draft Plans Cleaned — FIXED
**Priority**: P3 → FIXED
**What**: 65 draft plans in DB referenced test PRD files that were deleted during cleanup. Governor logged "Failed to fetch PRD" for each on startup.
**Fix**: Deleted 65 stale drafts. 1 valid plan remains (e2e-hello-world.md, status=review).
**Commit**: DB cleanup (not committed, operational)

---

## Fixed Issues (April 30, 2026 — Courier routing fix)

### Courier Routing: claim_task Never Matched 'available' Tasks — FIXED
**Priority**: P0 → FIXED
**What happened**: Router correctly selected web routing (connector=gemini-api-courier, model=gemini-2.5-flash-lite, destination=chatgpt-web). claim_task returned false every time. After 5 model retries, task fell through to internal execution. Zero tasks ever had `routing_flag='web'`.
**Root cause**: Migration 130 added `available` as a status for zero-dependency tasks. `claim_task` RPC only matched `WHERE status = 'pending'`. Tasks with no dependencies sat at `available`, never claimable.
**Fix**: Migration 133 recreates `claim_task` with `WHERE status IN ('pending', 'available')`. Dependency check still guards -- tasks with unmet deps remain `locked` or `pending`.
**Files**: `docs/supabase-schema/133_fix_claim_task_for_available.sql`
**Commit**: 84441bdc

### Courier Routing: Handler Overrode Router's RoutingFlag — FIXED
**Priority**: P0 → FIXED
**What happened**: Router returned `RoutingFlag: "web"`. Handler at line 234 ignored it and called `deriveRoutingFlag(connConfig)`. Since `gemini-api-courier` is `type=api`, it derived `"internal"`. Courier dispatch check on line 338 (`if routingFlag == "web"`) would always fail.
**Root cause**: `deriveRoutingFlag` looked at the connector fueling the model, not the destination platform. The connector is API (Gemini powers the task) but the destination is web (chatgpt-web). These are different things.
**Fix**: Use `routingResult.RoutingFlag` when explicitly set by the router. Only fall back to `deriveRoutingFlag()` when empty. Also removed redundant `transition_task` call that cleared `processing_by=NULL` (race condition fix from earlier commit, now in same code path).
**Files**: `governor/cmd/governor/handlers_task.go`
**Commit**: e62d960a

---

## Fixed Issues (April 30, 2026 — Race conditions, tokens, model management)

### Race Condition: Task Double-Dispatch — FIXED
**Priority**: P0 → FIXED
**Root cause**: `claim_task` atomically sets `processing_by`. Then handler called `transition_task` which reset it to NULL. Second event arrived, claim succeeded again.
**Fix**: Removed redundant `transition_task` call after `claim_task`.
**Commit**: 66d4d373

### PRD Webhook Flood (66 duplicate events) — FIXED
**Priority**: P0 → FIXED
**Root cause**: `prdExists` only checked `plans` table. Old test PRDs never had plans, so all passed through.
**Fix**: `prdExists` now also checks `orchestrator_events`.
**Commit**: 66d4d373

### Dashboard: Token Count Only Showed Run Tokens — FIXED
**Fix**: `calculateMetrics` now takes `max(runTokens, taskTokens)`.
**Commit**: 2b973a432

### Dashboard: ROI Totals Excluded Subscription Savings — FIXED
**Fix**: Totals now include subscription data.
**Commit**: 577d4f8f5

### Dashboard: Plan-Stage Runs Orphaned from Tasks — FIXED
**Fix**: Adapter builds `planToTask` map from `tasks.plan_id`.
**Commit**: 82c0e1103

### Plan-Level Supervisor Tokens Never Recorded — FIXED
**Fix**: Added `record_internal_run` call with role="plan_reviewer".
**Commit**: 66d4d373

### Models Config: Nemotron Still Present — FIXED
**Fix**: Removed all 6 Nemotron + 2 dead Ling models.
**Commit**: 484cf5ea

### Daily Model Health Check — NEW
**What**: Daily cron at 6 AM. Checks APIs, health-checks models, updates rate limits.
**Files**: `governor/scripts/daily_model_health.py`
**Commit**: 484cf5ea

---

## Fixed Issues (April 29, 2026)

### E2E Pipeline Test — PASSED
PRD `e2e-hello-world.md` pushed. Full autonomous pipeline completed. 12 orchestrator_events recorded.
**Commit**: 22212860

### Dashboard Fixes (10 fixes)
Owner mapping, token counting, log popup CSS, event sorting.
**Commits**: d2a791e30, 11e6b9818, e6c644471 through f75d8f9c2

### Cost Tracking 4-Phase Overhaul
subscription_history, record_internal_run, ROI panel, alerting.
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

---

## Build Priority (REMAINING)

| Priority | Issue | Effort | What's Needed |
|----------|-------|--------|---------------|
| P1 | Courier E2E test (verify web routing works end-to-end) | Medium | Push a PRD, verify routing_flag='web' lands in DB |
| P2 | Research agent + knowledgebase | Medium | Build researcher, wire knowledgebase |
| P3 | Stale Supabase-era prompts in DB | Trivial | DELETE FROM prompts |
| P3 | Dead code cleanup | Low | Remove old Python orchestrator refs |
| P3 | jcodemunch transport error | Low | Debug MCP connection on startup |
