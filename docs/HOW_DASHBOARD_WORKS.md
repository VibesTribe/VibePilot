# How the Vibeflow Dashboard Works

**CRITICAL: This document explains what the dashboard displays and where it gets its data. DO NOT modify the dashboard code. If something isn't displaying correctly, the problem is in the VibePilot Go code or Supabase data, NOT the dashboard.**

---

## Overview

The Vibeflow dashboard is a React/TypeScript application that displays real-time mission control for VibePilot. It uses **Supabase Realtime (PostgreSQL Change Data Capture)** to receive instant updates when data changes - **NOT polling**.

**Key Principle:** The dashboard is a **read-only view** of VibePilot state. It does NOT make decisions, route tasks, or execute code. It only displays what VibePilot has already done.

---

## Data Sources (Supabase Tables)

The dashboard subscribes to these Supabase tables via Realtime:

| Table | Purpose | Update Method |
|-------|---------|---------------|
| `tasks` | All task records with status, assignment, etc. | Realtime (instant) |
| `task_runs` | Execution records with tokens, costs, model info | Realtime (instant) |
| `models` | Active/paused models with subscription info | Realtime (instant) |
| `platforms` | Web platforms (Gemini Web, Claude Web, etc.) | Realtime (instant) |
| `orchestrator_events` | Event log for routing decisions | Realtime (instant) |

---

## Dashboard Sections

### 1. Mission Header (Top Bar)

**Location:** Top of screen, full width

**Displays:**
- **Status Pills:** 4 pills showing task counts
  - ✓ Complete (green): Tasks with status `merged`, `supervisor_approval`, `ready_to_merge`
  - ↻ Active (blue): Tasks with status `in_progress`, `assigned`, `received`, `testing`
  - ⏳ Pending (yellow): Tasks with status `assigned`, `blocked`
  - 🚩 Review (red): Tasks with status `supervisor_review`

- **Token Usage:** Total tokens used across all task_runs
  - Source: `task_runs.tokens_in + task_runs.tokens_out`
  - Format: "1.2M tokens"

- **ROI Savings:** Total savings from using free tiers vs paid APIs
  - Source: `task_runs.total_savings_usd` (sum)
  - Format: "$12.34 saved"

**Data Source:**
```typescript
// From useMissionData.ts
const statusSummary = buildStatusSummary(snapshot.tasks);
const tokenUsage = runMetrics.runs.reduce((sum, run) => sum + run.tokens_used, 0);
const roi = snapshot.roi?.totals.total_savings_usd;
```

**Supabase Query:**
```sql
SELECT * FROM tasks ORDER BY updated_at DESC LIMIT 100;
SELECT * FROM task_runs ORDER BY started_at DESC LIMIT 500;
```

---

### 2. Slice Hub (Main Grid)

**Location:** Center of screen, grid layout

**Displays:**
- **Slice Cards:** One card per `slice_id` (or "general" if null)
- Each card shows:
  - Slice name (e.g., "General", "Auth", "UI")
  - Progress ring: `completed / total` percentage
  - Task count: "3/5 tasks"
  - Active agents: Icons of agents currently working on tasks in this slice
  - Task list: All tasks grouped by slice

**Data Source:**
```typescript
// From vibepilotAdapter.ts - transformSlices()
const sliceMap = new Map<string, { total: number; done: number; tokens: number }>();
for (const task of tasks) {
  const sliceId = task.slice_id || "general";
  const stats = sliceMap.get(sliceId) || { total: 0, done: 0, tokens: 0 };
  stats.total += 1;
  if (task.status === "merged") {
    stats.done += 1;
  }
  sliceMap.set(sliceId, stats);
}
```

**Supabase Fields:**
- `tasks.slice_id` - Groups tasks into slices
- `tasks.status` - Determines completed vs active
- `task_runs.tokens_used` - Token count per slice

**Color Coding:**
```typescript
const SLICE_ACCENTS = {
  auth: "#f97316",    // Orange
  data: "#38bdf8",    // Blue
  ui: "#c084fc",      // Purple
  api: "#22d3ee",     // Cyan
  core: "#6366f1",    // Indigo
  testing: "#22c55e", // Green
  docs: "#facc15",    // Yellow
  config: "#ec4899",  // Pink
  general: "#94a3b8"  // Gray
};
```

---

### 3. Task Cards (Within Slice Cards)

**Location:** Inside each slice card, expandable

**Displays:**
- **Task Title:** `tasks.title`
- **Status Badge:** Color-coded status chip
- **Assigned Agent:** `tasks.assigned_to` (model ID)
- **Confidence:** Planner's confidence score (hardcoded to 85% currently)
- **Updated Time:** `tasks.updated_at`
- **Token Count:** From latest `task_runs` record for this task
- **Prompt Packet:** Collapsible section showing `tasks.result.prompt_packet`

**Status Mapping:**
```typescript
// From vibepilotAdapter.ts - mapTaskStatus()
const statusMap = {
  pending: "pending",      // Awaiting dependencies or resources
  available: "pending",    // Ready but waiting for model/connector
  in_progress: "in_progress",   // Actively being worked on
  review: "in_progress",   // Supervisor reviewing output
  testing: "in_progress",  // Tests running
  approval: "supervisor_approval",  // Ready for human review
  merged: "complete",      // Successfully merged
  complete: "complete",    // Task done
  failed: "pending",       // Will retry
  escalated: "pending",    // Will retry (no human needed)
};
```

**Human Review Required Only For:**
1. Visual UI/UX changes (requires human aesthetic judgment)
2. System researcher suggestions (after council review)
3. Paid API key out of credit (requires human to add funds)

All other failures are handled by AI - retries, model switching, etc.

**Owner Display:**
```typescript
// tasks.assigned_to -> "agent.{model_id}"
owner: task.assigned_to ? `agent.${task.assigned_to}` : null
```

**Location Badge:**
```typescript
// From deriveTaskLocation()
if (routing_flag === "internal") return { kind: "internal", label: "VibePilot" };
if (routing_flag === "mcp") return { kind: "mcp", label: "MCP Gateway" };
if (platform) return { kind: "platform", label: platform };
return { kind: "platform", label: "Web" };
```

---

### 4. Agent Hangar (Model/Platform Status)

**Location:** Accessible via "Models" button in action bar

**Displays:**
- **Agent Cards:** One per active model or platform
- Each card shows:
  - Model/Platform name
  - Status: `idle`, `in_progress`, `cooldown`, `credit_needed`
  - Tier: Q (internal/CLI), M (MCP), W (web platform)
  - Context window: `models.context_limit`
  - Active task count: How many tasks assigned to this model
  - Cooldown timer: If `models.cooldown_expires_at` is set
  - Warnings: From `models.status_reason`

**Agent Status Derivation:**
```typescript
// From transformAgents()
let agentStatus = "idle";
if (needsCredit) {
  agentStatus = "credit_needed";
} else if (inCooldown) {
  agentStatus = "cooldown";
} else if (stats.active > 0) {
  agentStatus = "in_progress";
}
```

**Tier Assignment:**
```typescript
// Q tier: CLI/API models (internal execution)
// M tier: MCP gateway models
// W tier: Web platforms (courier-based)
const tier = model.access_type === "web" ? "W" : 
             model.access_type === "mcp" ? "M" : "Q";
```

**Supabase Query:**
```sql
SELECT * FROM models WHERE status IN ('active', 'paused');
SELECT * FROM platforms WHERE status IN ('active', 'paused');
```

---

### 5. ROI Panel (Token & Cost Analytics)

**Location:** Accessible via "Tokens" button in header

**Displays:**
- **Total Savings:** Sum of `task_runs.total_savings_usd`
- **Theoretical Cost:** What it would cost using paid APIs
- **Actual Cost:** What was actually paid (courier costs)
- **By Slice:** ROI broken down by slice_id
- **By Model:** ROI per model (executor vs courier)
- **Subscriptions:** Active subscription value tracking

**ROI Calculation:**
```typescript
// From calculateROI()
{
  total_tokens_in: runs.reduce((sum, r) => sum + r.tokens_in, 0),
  total_tokens_out: runs.reduce((sum, r) => sum + r.tokens_out, 0),
  total_theoretical_usd: runs.reduce((sum, r) => sum + r.platform_theoretical_cost_usd, 0),
  total_actual_usd: runs.reduce((sum, r) => sum + r.total_actual_cost_usd, 0),
  total_savings_usd: runs.reduce((sum, r) => sum + r.total_savings_usd, 0)
}
```

**Supabase Fields:**
- `task_runs.tokens_in` - Input tokens
- `task_runs.tokens_out` - Output tokens
- `task_runs.courier_tokens` - Tokens used by courier
- `task_runs.platform_theoretical_cost_usd` - What paid API would cost
- `task_runs.courier_cost_usd` - Actual courier cost
- `task_runs.total_actual_cost_usd` - Total actual cost
- `task_runs.total_savings_usd` - Savings amount

---

### 6. Review Queue

**Location:** Right side panel (slides in when tasks need review)

**Displays:**
- Tasks with `status = 'supervisor_review'` or `approval`
- Each item shows:
  - Task title and number
  - Slice name
  - Diff URL: Link to GitHub diff
  - Preview URL: Link to Vercel preview (if available)
  - Review notes: From review table
  - Action buttons: Approve/Reject/Restore

**Data Source:**
```typescript
// From useReviewData.ts
const { data } = await supabase
  .from('review_queue')
  .select('*')
  .order('updated_at', { ascending: false });
```

---

### 7. Event Timeline

**Location:** Accessible via "Logs" button

**Displays:**
- Chronological list of orchestrator events
- Each event shows:
  - Timestamp
  - Event type: `task_assigned`, `task_completed`, `model_switched`, etc.
  - Task ID
  - Model ID: Which model was involved
  - Reason: Why this event happened
  - Details: Additional context

**Supabase Query:**
```sql
SELECT id, event_type, task_id, runner_id, from_runner_id, to_runner_id, 
       model_id, reason, details, created_at
FROM orchestrator_events
ORDER BY created_at DESC
LIMIT 500;
```

**Event Types:**
- `task_assigned` - Task given to a model
- `task_completed` - Task finished successfully
- `task_failed` - Task failed
- `model_switched` - Rerouted to different model
- `rate_limit_hit` - Model hit rate limit
- `cooldown_started` - Model in cooldown

---

## Data Flow: VibePilot → Dashboard

```
┌─────────────────────────────────────────────────────────────┐
│                    VibePilot (Go Backend)                    │
│                                                              │
│  1. PRD pushed to GitHub                                     │
│  2. Webhook triggers Governor                                │
│  3. Planner creates tasks                                    │
│  4. Router selects model (writes to tasks.assigned_to)      │
│  5. Task executed via kilo/opencode/courier                  │
│  6. Task run record created (writes to task_runs)           │
│  7. Tokens/costs recorded                                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Supabase Database                       │
│                                                              │
│  Tables:                                                     │
│  - tasks (id, title, status, assigned_to, slice_id, ...)    │
│  - task_runs (task_id, model_id, tokens_in, tokens_out, ...)│
│  - models (id, name, status, context_limit, ...)            │
│  - platforms (id, name, status, type, ...)                  │
│  - orchestrator_events (event_type, task_id, model_id, ...) │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ (poll every 5s)
┌─────────────────────────────────────────────────────────────┐
│                   Vibeflow Dashboard (React)                 │
│                                                              │
│  1. useMissionData() fetches from Supabase                  │
│  2. adaptVibePilotToDashboard() transforms data             │
│  3. Components render with transformed data                 │
│  4. User sees real-time mission status                      │
└─────────────────────────────────────────────────────────────┘
```

---

## Critical Fields the Dashboard Expects

### tasks table MUST have:
- `id` (UUID) - Task identifier
- `title` (text) - Task title
- `status` (text) - One of: pending, available, in_progress, review, testing, approval, merged, failed, escalated
- `assigned_to` (text) - Model ID (e.g., "glm-5", "gemini-2.5-flash")
- `slice_id` (text, nullable) - Slice grouping (e.g., "general", "auth", "ui")
- `task_number` (text, nullable) - Human-readable task ID (e.g., "T001")
- `routing_flag` (text) - "internal", "mcp", or "web"
- `updated_at` (timestamp) - Last update time
- `result` (jsonb, nullable) - Contains `prompt_packet`

### task_runs table MUST have:
- `id` (UUID) - Run identifier
- `task_id` (UUID) - FK to tasks
- `model_id` (text, nullable) - Which model executed
- `platform` (text, nullable) - Web platform name
- `tokens_in` (integer) - Input tokens
- `tokens_out` (integer) - Output tokens
- `tokens_used` (integer) - Total tokens
- `courier_tokens` (integer, nullable) - Tokens used by courier
- `courier_cost_usd` (decimal, nullable) - Courier cost
- `platform_theoretical_cost_usd` (decimal, nullable) - Theoretical API cost
- `total_actual_cost_usd` (decimal, nullable) - Actual cost
- `total_savings_usd` (decimal, nullable) - Savings amount
- `started_at` (timestamp) - Run start time
- `completed_at` (timestamp, nullable) - Run end time

### models table MUST have:
- `id` (text) - Model ID (e.g., "glm-5")
- `name` (text) - Display name
- `status` (text) - "active" or "paused"
- `access_type` (text) - "cli_subscription", "api", "web", or "mcp"
- `context_limit` (integer) - Context window size
- `status_reason` (text, nullable) - Why paused
- `cooldown_expires_at` (timestamp, nullable) - Cooldown end time
- `subscription_cost_usd` (decimal, nullable) - Monthly subscription cost
- `tokens_used` (integer) - Lifetime tokens
- `tasks_completed` (integer) - Tasks completed count

### platforms table MUST have:
- `id` (text) - Platform ID (e.g., "gemini-web")
- `name` (text) - Display name
- `status` (text) - "active" or "paused"
- `type` (text) - "web"
- `context_limit` (integer) - Context window size
- `config` (jsonb) - Contains free_tier info, capabilities, etc.

---

## Common Dashboard Issues & Root Causes

### Issue: Task shows "Unassigned"
**Root Cause:** `tasks.assigned_to` is NULL or empty
**Fix:** Governor must write model ID to `assigned_to` when routing

### Issue: Token count shows 0
**Root Cause:** `task_runs` record not created or `tokens_used` is NULL
**Fix:** Governor must create task_runs record after execution with token counts

### Issue: Model not showing in Agent Hangar
**Root Cause:** `models.status` not "active" or "paused", or model not in table
**Fix:** Ensure model exists in `models` table with correct status

### Issue: ROI shows $0 saved
**Root Cause:** `task_runs.total_savings_usd` not calculated or NULL
**Fix:** Governor must calculate and write savings when creating task_runs

### Issue: Slice shows as "General" instead of proper name
**Root Cause:** `tasks.slice_id` is NULL or not matching slice catalog
**Fix:** Planner must set correct slice_id when creating tasks

### Issue: Task location shows "Web" instead of "VibePilot"
**Root Cause:** `tasks.routing_flag` not set to "internal"
**Fix:** Router must set routing_flag when assigning task

---

## Dashboard Realtime Subscriptions

| Data | Method | Channel |
|------|--------|---------|
| Tasks/Runs/Models/Platforms | Supabase Realtime | `dashboard-tasks` |
| Events | Supabase Realtime | `dashboard-events` |
| Initial Load | REST API query | On mount |

The dashboard creates two realtime channels:
1. `dashboard-tasks` - Listens for changes on `tasks`, `task_runs`, `models`, `platforms`
2. `dashboard-events` - Listens for changes on `orchestrator_events`

---

## Key Takeaways for VibePilot Development

1. **Write to `tasks.assigned_to`** - Dashboard shows this as the agent working on the task
2. **Create `task_runs` records** - Dashboard needs this for token counts and ROI
3. **Set `tasks.slice_id`** - Groups tasks into slices in the UI
4. **Set `tasks.routing_flag`** - Determines location badge (internal/web/mcp)
5. **Calculate costs in `task_runs`** - ROI panel depends on accurate cost data
6. **Update `models.status`** - Agent hangar shows paused models in cooldown
7. **Write to `orchestrator_events`** - Timeline shows routing decisions

**Remember: The dashboard is READ-ONLY. If something doesn't display correctly, fix the Go code that writes to Supabase, NOT the dashboard code.**
