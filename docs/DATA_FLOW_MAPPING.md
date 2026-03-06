# VibePilot Data Flow Mapping

**Purpose:** Clear mapping of Dashboard → Supabase → Go code. Know exactly what needs fixing.

---

## Dashboard Sections & Their Data Sources

### 1. Status Pills (Top Bar)

| Pill | Dashboard Reads | Supabase Field | Go Code That Writes |
|------|-----------------|----------------|---------------------|
| ✓ Complete | `status === "merged" \|\| "supervisor_approval" \|\| "ready_to_merge"` | `tasks.status` | `handlers_task.go` - EventTaskCompleted |
| ↻ Active | `status === "in_progress" \|\| "assigned" \|\| "received" \|\| "testing"` | `tasks.status` | `handlers_task.go` - EventTaskAvailable |
| ⏳ Pending | `status === "pending" \|\| "available" \|\| "blocked"` | `tasks.status` | Planner creates tasks with "available" |
| 🚩 Review | `status === "supervisor_review"` | `tasks.status` | `handlers_task.go` - after task execution |

**Current Issue:** Status mapping in `vibepilotAdapter.ts` line 124-137:
```typescript
const statusMap = {
  pending: "pending",
  available: "pending",      // ← Maps to pending
  in_progress: "in_progress",
  review: "supervisor_review", // ← Maps to review flag
  ...
};
```

---

### 2. Task Cards

| What Dashboard Shows | Supabase Field | Go Code That Writes | Status |
|---------------------|----------------|---------------------|--------|
| Task Title | `tasks.title` | Planner (handlers_plan.go) | ✅ Works |
| Status Badge | `tasks.status` | handlers_task.go | ⚠️ Check mapping |
| **Assigned Agent** | `tasks.assigned_to` | Router in EventTaskAvailable | ❌ **May be NULL** |
| Slice Name | `tasks.slice_id` | Planner | ✅ Works |
| Task Number | `tasks.task_number` | Planner | ✅ Works |
| Location Badge | `tasks.routing_flag` | Router | ⚠️ Check default |
| **Token Count** | `task_runs.tokens_used` | After task execution | ❌ **Not being written** |
| Prompt Packet | `tasks.result.prompt_packet` | Planner | ✅ Works |

**Critical Fields Dashboard Expects:**

```typescript
// From vibepilotAdapter.ts line 188-218
return {
  id: task.id,
  title: task.title || "Untitled Task",
  status: mapTaskStatus(task.status),
  owner: task.assigned_to ? `agent.${task.assigned_to}` : null,  // ← NEEDS assigned_to
  sliceId: task.slice_id ? `slice.${task.slice_id}` : undefined,
  location: deriveTaskLocation(task.routing_flag, ...),          // ← NEEDS routing_flag
  metrics: {
    tokensUsed: run?.tokens_used || 0,  // ← NEEDS task_runs record
  },
};
```

---

### 3. Agent Hangar (Model Status)

| What Dashboard Shows | Supabase Field | Go Code That Writes |
|---------------------|----------------|---------------------|
| Model Name | `models.name` | Config (models.json) |
| Status (idle/in_progress/cooldown) | Derived from `models.status` + `cooldown_expires_at` | Router updates |
| Active Task Count | Count of `tasks.assigned_to === model.id` | Via task assignment |
| Context Window | `models.context_limit` | Config |
| Subscription Cost | `models.subscription_cost_usd` | Config |

**Status Derivation (vibepilotAdapter.ts line 265-272):**
```typescript
let agentStatus = "idle";
if (needsCredit) {
  agentStatus = "credit_needed";
} else if (inCooldown) {
  agentStatus = "cooldown";
} else if (stats.active > 0) {  // ← Counts tasks with this assigned_to
  agentStatus = "in_progress";
}
```

---

### 4. ROI Panel

| What Dashboard Shows | Supabase Field | Go Code That Writes | Status |
|---------------------|----------------|---------------------|--------|
| Total Tokens | `task_runs.tokens_in + tokens_out` | After execution | ❌ **Not written** |
| Theoretical Cost | `task_runs.platform_theoretical_cost_usd` | After execution | ❌ **Not written** |
| Actual Cost | `task_runs.total_actual_cost_usd` | After execution | ❌ **Not written** |
| Savings | `task_runs.total_savings_usd` | After execution | ❌ **Not written** |

**ROI Calculation (vibepilotAdapter.ts line 482-510):**
```typescript
return runs.reduce((acc, run) => ({
  total_tokens_in: acc.total_tokens_in + (run.tokens_in || 0),
  total_tokens_out: acc.total_tokens_out + (run.tokens_out || 0),
  total_theoretical_usd: acc.total_theoretical_usd + (run.platform_theoretical_cost_usd || 0),
  total_actual_usd: acc.total_actual_usd + (run.total_actual_cost_usd || 0),
  total_savings_usd: acc.total_savings_usd + (run.total_savings_usd || 0),
}), { ... });
```

---

## Go Code → Supabase Field Mapping

### handlers_task.go - EventTaskAvailable

**What it should write:**

| Field | Table | Current Status |
|-------|-------|----------------|
| `assigned_to` | tasks | ⚠️ Check if writing |
| `routing_flag` | tasks | ⚠️ Check default |
| `status` = "in_progress" | tasks | ✅ Should work |
| `model_id` | task_runs | ❌ Not creating record |
| `tokens_in` | task_runs | ❌ Not extracting from CLI |
| `tokens_out` | task_runs | ❌ Not extracting from CLI |

### handlers_task.go - After Task Execution

**What it should write:**

| Field | Table | Current Status |
|-------|-------|----------------|
| `status` = "review" | tasks | ⚠️ Check if commit succeeds first |
| `task_id` | task_runs | ❌ Record not created |
| `model_id` | task_runs | ❌ Record not created |
| `tokens_in` | task_runs | ❌ Not extracted |
| `tokens_out` | task_runs | ❌ Not extracted |
| `tokens_used` | task_runs | ❌ Not extracted |
| `platform_theoretical_cost_usd` | task_runs | ❌ Not calculated |
| `total_actual_cost_usd` | task_runs | ❌ Not calculated |
| `total_savings_usd` | task_runs | ❌ Not calculated |

---

## What's Broken (Priority Order)

### 1. task_runs Record Not Created (HIGH)

**Problem:** No task_runs records being created after task execution.

**Dashboard Impact:**
- Token counts show 0
- ROI shows $0
- No execution history

**Go File:** `governor/cmd/governor/handlers_task.go`

**What needs to happen:**
1. After task execution completes, create task_runs record
2. Extract tokens from CLI runner output
3. Calculate costs (theoretical vs actual)
4. Write to Supabase

### 2. Token Extraction from CLI (HIGH)

**Problem:** opencode/kilo output contains token counts but we're not extracting them.

**CLI Output Example:**
```
Tokens: 1500 in, 3200 out
```

**Go File:** `governor/internal/connectors/runners.go`

**What needs to happen:**
1. Parse CLI output for token counts
2. Return in SessionResult struct
3. Pass to task_runs creation

### 3. Status Logic After Execution (HIGH)

**Problem:** Status set to "review" even if commit fails.

**Go File:** `governor/cmd/governor/handlers_task.go`

**What needs to happen:**
1. Check if commit succeeded
2. If success → status = "review"
3. If failure → status = "failed" or "available" (retry)

### 4. assigned_to Field (MEDIUM)

**Problem:** May not be writing model ID to tasks.assigned_to

**Dashboard Impact:**
- Shows "Unassigned" for tasks
- Agent hangar doesn't show active tasks per model

**Go File:** `governor/cmd/governor/handlers_task.go`

**What needs to happen:**
1. Router selects model
2. Write model ID to tasks.assigned_to
3. Dashboard reads and displays

---

## Migration Status

| Migration | File | Status |
|-----------|------|--------|
| 064 | `064_update_task_assignment.sql` | ✅ Exists |
| 065 | `065_create_task_run.sql` | ❌ **DOES NOT EXIST** |

**Note:** CURRENT_STATE.md mentions 065 but it was never created. The RPC `create_task_run` doesn't exist in Supabase.

---

## Next Steps (In Order)

1. **Read current handlers_task.go** - Understand what's currently being written
2. **Read current runners.go** - Understand CLI output parsing
3. **Create migration 065** - Add create_task_run RPC (if needed)
4. **Fix token extraction** - Parse CLI output for tokens
5. **Fix task_runs creation** - Write record after execution
6. **Fix status logic** - Only "review" on success
7. **Test end-to-end** - Verify dashboard shows correct data

---

## Quick Reference: Dashboard Field Mapping

```
┌─────────────────────────────────────────────────────────────────────┐
│                         DASHBOARD DISPLAY                           │
├─────────────────────────────────────────────────────────────────────┤
│ Status Pills     ← tasks.status (mapped via statusMap)             │
│ Task Cards       ← tasks.*, task_runs.*                            │
│ Assigned Agent   ← tasks.assigned_to                               │
│ Token Count      ← task_runs.tokens_used                           │
│ Location Badge   ← tasks.routing_flag                              │
│ Agent Hangar     ← models.*, platforms.*, tasks.assigned_to        │
│ ROI Panel        ← task_runs (tokens_in, tokens_out, *_cost_usd)   │
└─────────────────────────────────────────────────────────────────────┘
                              ↑
                              │ polls every 5s
                              │
┌─────────────────────────────────────────────────────────────────────┐
│                          SUPABASE                                   │
├─────────────────────────────────────────────────────────────────────┤
│ tasks: id, title, status, assigned_to, slice_id, routing_flag      │
│ task_runs: task_id, model_id, tokens_in, tokens_out, costs         │
│ models: id, name, status, context_limit, subscription_*            │
│ platforms: id, name, status, config                                │
└─────────────────────────────────────────────────────────────────────┘
                              ↑
                              │ writes via RPC
                              │
┌─────────────────────────────────────────────────────────────────────┐
│                       GO GOVERNOR                                   │
├─────────────────────────────────────────────────────────────────────┤
│ handlers_task.go  → writes tasks.status, tasks.assigned_to         │
│ handlers_plan.go  → writes tasks (planner creates)                 │
│ router.go         → selects model, sets routing_flag               │
│ runners.go        → executes CLI, should extract tokens            │
│ (MISSING)         → should create task_runs records                │
└─────────────────────────────────────────────────────────────────────┘
```
