# Dashboard Audit: Vibeflow Mission Control

**Generated:** 2026-04-16  
**Source:** ~/vibeflow/ (separate repo from VibePilot)  
**Purpose:** Complete traceability map -- what the dashboard NEEDS from Supabase, what Go WRITES, and where they MUST align.

---

## 1. Tables the Dashboard Reads (5 tables, realtime subscriptions)

### PRIMARY QUERY (useMissionData.ts lines 106-116):
```
tasks       → select("*") order(updated_at desc) limit(100)
task_runs   → select("*") order(started_at desc) limit(500)
models      → select("*") in(status, ["active","paused"])
platforms   → select("*") in(status, ["active","paused"])
orchestrator_events → select(id,event_type,task_id,runner_id,from_runner_id,to_runner_id,model_id,reason,details,created_at) order(created_at desc) limit(500)
```

### REALTIME SUBSCRIPTIONS (useMissionData.ts lines 252-263):
```
Channel "dashboard-tasks"  → postgres_changes on: tasks, task_runs, models, platforms
Channel "dashboard-events" → postgres_changes on: orchestrator_events
```
Any INSERT/UPDATE/DELETE on these 5 tables triggers a full re-fetch.

---

## 2. Exact Columns the Dashboard Expects Per Table

### tasks table (via VibePilotTask interface, vibepilotAdapter.ts lines 27-46):

| Column | Type | Used For | Required? |
|--------|------|----------|-----------|
| id | string | Task identity, run lookup key | YES |
| title | string/null | Task card display | YES (fallback "Untitled Task") |
| status | string | Status badge, routing, filtering | YES |
| priority | number | Sort order (not currently displayed) | Read |
| slice_id | string/null | Slice grouping, dock panels | YES |
| phase | string/null | Read but not used in display | Read |
| task_number | string/null | Task card label "T-001" | Displayed |
| routing_flag | string | Location badge (internal/mcp/web) | YES |
| routing_flag_reason | string/null | Task summary text | YES |
| assigned_to | string/null | Owner badge "agent.model_id" | YES |
| dependencies | string[]/null | Dependency graph | YES |
| result | JSON/null | Contains prompt_packet for task detail | YES |
| result.prompt_packet | nested | Task packet detail view | YES |
| confidence | number/null | Confidence badge (default 0.85) | YES |
| category | string/null | Read from Supabase | Read |
| created_at | string | Timeline display | YES |
| updated_at | string | Sort order, "last updated" | YES |
| started_at | string/null | Runtime calculation | YES |
| completed_at | string/null | Runtime calculation | YES |

**STATUS MAPPING (vibepilotAdapter.ts lines 125-140):**
```
Supabase → Dashboard
pending       → "pending"
available     → "pending"
in_progress   → "in_progress"
review        → "review"
testing       → "testing"
awaiting_human → "human_review"
merged        → "merged"
complete      → "complete"
merge_pending → "merge_pending"
failed        → "pending"     ← STATUS LOST! Shows as pending
escalated     → "pending"     ← STATUS LOST! Shows as pending
```

**WARNING:** `failed` and `escalated` statuses are mapped to `pending` -- the distinction is invisible in the dashboard.

### task_runs table (via VibePilotTaskRun interface, vibepilotAdapter.ts lines 49-67):

| Column | Type | Used For | Required? |
|--------|------|----------|-----------|
| id | string | Unique run identity | YES |
| task_id | string | Join to tasks | YES |
| model_id | string/null | Agent assignment, ROI by model | YES |
| platform | string/null | Location badge | YES |
| courier | string/null | Read but not directly displayed | Read |
| status | string | Run status, success count | YES |
| tokens_used | number/null | Token metrics display | YES |
| tokens_in | number/null | ROI calculation | YES |
| tokens_out | number/null | ROI calculation | YES |
| courier_model_id | string/null | Model ROI (courier role) | YES |
| courier_tokens | number/null | ROI calculation | YES |
| courier_cost_usd | number/null | ROI actual cost | YES |
| platform_theoretical_cost_usd | number/null | ROI theoretical cost | YES |
| total_actual_cost_usd | number/null | ROI actual cost | YES |
| total_savings_usd | number/null | ROI savings | YES |
| started_at | string | Runtime calculation, sort order | YES |
| completed_at | string/null | Runtime calculation | YES |

### models table (via VibePilotModel interface, vibepilotAdapter.ts lines 70-91):

| Column | Type | Used For | Required? |
|--------|------|----------|-----------|
| id | string | Agent identity "agent.model_id" | YES |
| name | string/null | Agent display name | YES |
| vendor | string/null | Agent vendor badge | Displayed |
| access_type | string | Tier badge (Q/M/W) | YES |
| context_limit | number/null | Context window display | YES |
| status | string | Agent status (active/paused) | YES |
| status_reason | string/null | Cooldown/credit reason | YES |
| logo_url | string/null | Agent avatar | YES |
| tokens_used | number/null | Subscription ROI | YES |
| tasks_completed | number/null | Subscription ROI | YES |
| tasks_failed | number/null | Subscription ROI | YES |
| success_rate | number/null | Subscription ROI | YES |
| cooldown_expires_at | string/null | Cooldown timer | YES |
| config | JSON/null | Read but not used in display | Read |
| subscription_cost_usd | number/null | Subscription ROI | YES |
| subscription_started_at | string/null | Subscription ROI dates | YES |
| subscription_ends_at | string/null | Subscription ROI dates | YES |
| subscription_status | string/null | Filter active subscriptions | YES |
| cost_input_per_1k_usd | number/null | Value comparison | YES |
| cost_output_per_1k_usd | number/null | Value comparison | YES |

### platforms table (via VibePilotPlatform interface, vibepilotAdapter.ts lines 94-120):

| Column | Type | Used For | Required? |
|--------|------|----------|-----------|
| id | string | Platform identity | YES |
| name | string/null | Display name fallback | YES |
| vendor | string/null | Vendor badge | Displayed |
| type | string | Platform type | YES |
| context_limit | number/null | Context window display | YES |
| status | string | Filter active only | YES |
| logo_url | string/null | Not used (logo derived from id) | -- |
| theoretical_cost_input_per_1k_usd | number/null | Not used in current code | -- |
| theoretical_cost_output_per_1k_usd | number/null | Not used in current code | -- |
| config | JSON/null | Deeply used for display | YES |
| config.name | string | Display name override | YES |
| config.provider | string | Vendor override | YES |
| config.free_tier.model | string | Summary text | YES |
| config.free_tier.rate_limits | object | Read but not displayed | Read |
| config.free_tier.context_limit | number | Context window override | YES |
| config.capabilities | string[] | Read but not displayed | Read |
| config.strengths | string[] | Read but not displayed | Read |
| config.notes | string | Summary text | YES |

### orchestrator_events table (useMissionData.ts lines 111-115):

| Column | Type | Used For | Required? |
|--------|------|----------|-----------|
| id | string | Event identity | YES |
| event_type | string | Timeline display | YES |
| task_id | string | Event→Task join | YES |
| runner_id | string | Details.runnerId | YES |
| from_runner_id | string | Details.fromRunnerId | YES |
| to_runner_id | string | Details.toRunnerId | YES |
| model_id | string | Details.modelId | YES |
| reason | string | Reason code | YES |
| details | JSON | Extended event info | YES |
| created_at | string | Timestamp, sort order | YES |

---

## 3. Computed/Derived Fields (Client-Side, NOT in Supabase)

These are calculated by the adapter from raw Supabase data:

| Derived Field | Source | Logic |
|---------------|--------|-------|
| runtimeSeconds | task_runs.started_at + completed_at | Date diff in seconds |
| metrics.costUsd | -- | Hardcoded to 0 (not calculated!) |
| owner | tasks.assigned_to | Prefixed with "agent." |
| sliceId | tasks.slice_id | Prefixed with "slice." |
| summary | tasks.routing_flag_reason | Direct passthrough |
| mergePending | tasks.status === "approval" | Boolean flag |
| AgentSnapshot.status | models.status + cooldown | Complex: idle/in_progress/cooldown/credit_needed |
| AgentSnapshot.tier | models.access_type | Q/M/W mapping |
| AgentSnapshot.creditStatus | models.status + status_reason | available/depleted/unknown |
| effectiveContextWindowTokens | models.context_limit * 0.75 | 75% of stated limit |
| SliceCatalog | Grouped by tasks.slice_id | Count totals/done/mergePending |
| ROITotals | Sum of all task_runs | Aggregated costs |
| SliceROI | Per-slice aggregation | From tasks + runs |
| ModelROI | Per-model from runs | Executor + courier roles |
| SubscriptionROI | models subscription fields | Prorated cost, recommendation |

---

## 4. What the Dashboard WRITES Back to Supabase

**NOTHING.** The dashboard is read-only. It only SELECTs and subscribes to realtime changes. All writes come from the Go governor.

---

## 5. TypeScript Type Contracts

### TaskSnapshot (what every task card renders):
```
id, title, status, confidence, updatedAt, owner, lessons,
sliceId, taskNumber, location, dependencies, packet, summary,
mergePending, metrics{tokensUsed, runtimeSeconds, costUsd}
```

### AgentSnapshot (what every agent card renders):
```
id, name, status, summary, updatedAt, logo, tier,
cooldownReason, costPerRunUsd, vendor, capability,
contextWindowTokens, effectiveContextWindowTokens,
cooldownExpiresAt, creditStatus, rateLimitWindowSeconds,
costPer1kTokensUsd, warnings[]
```

### MergeCandidate (for ReadyToMerge panel):
```
branch, title, summary, checklist[]
```
**NOT populated from Supabase** -- currently empty array.

### FailureSnapshot:
```
id, title, summary, reasonCode
```
**NOT populated from Supabase** -- currently empty array.

---

## 6. Mismatches and Red Flags

### CRITICAL: Column name `access_type` on models table
- Dashboard reads `models.access_type` to determine tier (Q/M/W)
- Go code writes... unknown -- this column might not be populated by the governor
- **If empty, all agents show as Q tier**

### CRITICAL: `status_reason` on models table  
- Dashboard reads `models.status_reason` for cooldown/credit messages
- Go code: `record_model_failure` and `record_model_success` update the models table
- **Must contain meaningful text like "Rate limit cooldown" or "Out of credits"**

### CRITICAL: `failed` and `escalated` status mapping
- Dashboard maps BOTH to "pending" -- invisible distinction
- Tasks that failed show as pending, potentially re-queued visually

### WARNING: `metrics.costUsd` hardcoded to 0
- TaskSnapshot.metrics.costUsd is always 0
- ROI shows aggregate costs but per-task cost is missing

### WARNING: Empty arrays (not populated)
- `failures: []` -- FailureSnapshot never filled from Supabase
- `mergeCandidates: []` -- MergeCandidate never filled from Supabase
- These UI panels are dead/empty

### WARNING: `result.prompt_packet` structure assumption
- Dashboard expects `task.result.prompt_packet` to be a string
- Go stores `task.result` as JSONB -- must contain `prompt_packet` key

### WARNING: `phase` column on tasks
- Dashboard reads it (VibePilotTask interface) but never displays it
- Column may exist in Supabase from migration but Go doesn't write it

### MINOR: platforms.config structure
- Dashboard deeply reads `config.free_tier.model`, `config.notes`, `config.provider`
- These must be valid JSON in the platforms table
- If config is null, fallback to platform.name and platform.vendor

---

## 7. Go ↔ Dashboard Contract Summary

The Go governor MUST write these fields correctly or the dashboard breaks:

### tasks table (Go writes via transition_task, claim_task, update_task_branch):
```
id, title, status, slice_id, task_number, routing_flag,
routing_flag_reason, assigned_to, dependencies, result (JSONB),
confidence, category, updated_at, started_at, completed_at
```

### task_runs table (Go writes via create_task_run):
```
id, task_id, model_id, platform, courier, status,
tokens_used, tokens_in, tokens_out, courier_model_id,
courier_tokens, courier_cost_usd, platform_theoretical_cost_usd,
total_actual_cost_usd, total_savings_usd, started_at, completed_at
```

### models table (Go writes via record_model_success, record_model_failure):
```
id, name, status, status_reason, tokens_used, tasks_completed,
tasks_failed, success_rate, cooldown_expires_at
```
**Must also populate:** access_type, vendor, context_limit, logo_url,
subscription_cost_usd, subscription_started_at, subscription_ends_at,
subscription_status, cost_input_per_1k_usd, cost_output_per_1k_usd

### platforms table (Seeded manually, Go doesn't write):
```
id, name, vendor, type, status, context_limit, config (JSONB)
```

### orchestrator_events table (Go writes via record_state_transition):
```
id, event_type, task_id, runner_id, from_runner_id, to_runner_id,
model_id, reason, details (JSONB), created_at
```

---

## 8. What Happens If Each Table Breaks

| Table Broken | Dashboard Impact |
|---|---|
| tasks | **TOTAL FAILURE** -- no task cards, no slices, no status summary |
| task_runs | **BROKEN ROI** -- no costs, no tokens, no runtime, no model analytics |
| models | **NO AGENTS** -- Agent hangar empty, no tier badges, no cooldown info |
| platforms | **NO WEB AGENTS** -- W-tier agents missing, only Q/M from models |
| orchestrator_events | **NO TIMELINE** -- Event feed empty, no quality map |

---

## 9. Review Data (separate hook)

The review system reads from a separate data source:

### useReviewData.ts expects:
- Tasks with status "review" or "awaiting_human"
- Review entries with: taskId, title, taskNumber, sliceName, owner, summary, updatedAt, review status, notes, reviewer, diffUrl, comparisonUrl, previewUrl
- Restore records for undo capability

### ReviewQueueItem (types/review.ts):
```
taskId, title, taskNumber?, sliceName?, owner?, summary?,
updatedAt?, status, notes?, reviewer?, diffUrl?, comparisonUrl?,
previewUrl?, entry, task?, restore?
```

---

**This document is the CONTRACT between Go governor, Supabase schema, and Vibeflow dashboard.**
**Any schema change, Go edit, or migration MUST be checked against this document.**
