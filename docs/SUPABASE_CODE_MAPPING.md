# VibePilot Supabase → Go Code → Dashboard Mapping

**Purpose:** Single source of truth for data flow. If dashboard shows wrong data, find the Go code here.

**Last Updated:** 2026-03-07

---

## Core Principle

> **Dashboard is READ-ONLY. If something doesn't display correctly, fix the Go code that writes to Supabase, NOT the dashboard.**

---

## Table: tasks

### Dashboard Expects

| Column | Type | Dashboard Uses | Go Code That Writes |
|--------|------|----------------|---------------------|
| `id` | UUID | Task identifier | `create_task_with_packet` RPC |
| `title` | TEXT | Task card title | `create_task_with_packet` RPC |
| `status` | TEXT | Status pill color | `update_task_status` RPC, `update_task_assignment` RPC |
| `assigned_to` | TEXT | Agent icon, hangar count | `update_task_assignment` RPC |
| `slice_id` | TEXT | Slice card grouping | `create_task_with_packet` RPC |
| `task_number` | TEXT | Task card number | `create_task_with_packet` RPC |
| `routing_flag` | TEXT | Location badge | `update_task_assignment` RPC |
| `routing_flag_reason` | TEXT | Tooltip | `update_task_assignment` RPC |
| `confidence` | FLOAT | Confidence display | `create_task_with_packet` RPC |
| `category` | TEXT | Model selection | `create_task_with_packet` RPC |
| `type` | TEXT | Model selection | `create_task_with_packet` RPC |
| `dependencies` | JSONB | Dependency tracking | `create_task_with_packet` RPC |
| `max_attempts` | INT | Retry limit | `create_task_with_packet` RPC |
| `attempts` | INT | Current attempts | `record_model_failure` RPC |
| `result` | JSONB | Prompt packet display | `create_task_with_packet` RPC |

### Valid Status Values

```go
// From handlers_task.go
const (
    StatusPending    = "pending"     // Has unmet dependencies
    StatusAvailable  = "available"   // Ready to claim
    StatusInProgress = "in_progress" // Being executed
    StatusReview     = "review"      // Needs supervisor review
    StatusTesting    = "testing"     // Tests running
    StatusApproval   = "approval"    // Ready for human approval
    StatusMerged     = "merged"      // Complete
    StatusEscalated  = "escalated"   // Failed max attempts
)
```

### Valid routing_flag Values

```go
// From handlers_task.go:687-700
const (
    RoutingInternal = "internal" // CLI/API execution
    RoutingMCP      = "mcp"      // MCP gateway
    RoutingWeb      = "web"      // Web platform via courier
)
```

### Valid type Values (Schema Constraint)

```sql
-- From schema constraint
CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility'))
```

### Go Code Reference

```go
// handlers_task.go:133-144
_, err = h.database.RPC(ctx, "update_task_assignment", map[string]any{
    "p_task_id":             taskID,
    "p_status":              "in_progress",
    "p_assigned_to":         modelID,
    "p_routing_flag":        routingFlag,        // ← MUST SET
    "p_routing_flag_reason": routingReason,
})

// validation.go:147-162
taskID, err := database.RPC(ctx, "create_task_with_packet", map[string]any{
    "p_plan_id":             planID,
    "p_task_number":         task.TaskNumber,
    "p_title":               task.Title,
    "p_type":                task.Type,
    "p_status":              status,
    "p_priority":            5,
    "p_confidence":          task.Confidence,
    "p_category":            task.Category,
    "p_routing_flag":        routingFlag,
    "p_routing_flag_reason": routingReason,
    "p_dependencies":        task.Dependencies,
    "p_prompt":              task.PromptPacket,
    "p_expected_output":     task.ExpectedOutput,
    "p_context":             map[string]any{"source": "plan_approval"},
    // MISSING: "p_max_attempts": 3,
})
```

---

## Table: task_runs

### Dashboard Expects

| Column | Type | Dashboard Uses | Go Code That Writes |
|--------|------|----------------|---------------------|
| `id` | UUID | Run identifier | Auto-generated |
| `task_id` | UUID | Link to task | `create_task_run` RPC |
| `model_id` | TEXT | Model display | `create_task_run` RPC |
| `courier` | TEXT | Courier display | `create_task_run` RPC |
| `platform` | TEXT | Platform display | `create_task_run` RPC |
| `status` | TEXT | Run status | `create_task_run` RPC |
| `tokens_in` | INT | Token count | `create_task_run` RPC |
| `tokens_out` | INT | Token count | `create_task_run` RPC |
| `tokens_used` | INT | Total tokens | `create_task_run` RPC |
| `courier_tokens` | INT | Courier tokens | `create_task_run` RPC |
| `courier_cost_usd` | DECIMAL | Courier cost | `create_task_run` RPC |
| `platform_theoretical_cost_usd` | DECIMAL | Theoretical cost | `create_task_run` RPC |
| `total_actual_cost_usd` | DECIMAL | Actual cost | `create_task_run` RPC |
| `total_savings_usd` | DECIMAL | ROI savings | `create_task_run` RPC |
| `started_at` | TIMESTAMPTZ | Duration calc | `create_task_run` RPC |
| `completed_at` | TIMESTAMPTZ | Duration calc | `create_task_run` RPC |

### Go Code Reference

```go
// handlers_task.go:233-256
_, err = h.database.RPC(ctx, "create_task_run", map[string]any{
    "p_task_id":                       taskID,
    "p_model_id":                      modelID,
    "p_courier":                       connectorID,
    "p_platform":                      h.deriveRoutingFlag(h.cfg.GetConnector(connectorID)),
    "p_status":                        status,
    "p_tokens_in":                     tokensIn,
    "p_tokens_out":                    tokensOut,
    "p_tokens_used":                   totalTokens,
    "p_courier_model_id":              nil,
    "p_courier_tokens":                0,
    "p_courier_cost_usd":              0,
    "p_platform_theoretical_cost_usd": costs.Theoretical,
    "p_total_actual_cost_usd":         costs.Actual,
    "p_total_savings_usd":             costs.Savings,
    "p_started_at":                    runStart,
    "p_completed_at":                  time.Now(),
})
```

---

## Table: models

### Dashboard Expects

| Column | Type | Dashboard Uses | Go Code That Writes |
|--------|------|----------------|---------------------|
| `id` | TEXT | Model identifier | Config load |
| `name` | TEXT | Display name | Config load |
| `status` | TEXT | Agent status | `record_model_failure`, `record_model_success` |
| `access_type` | TEXT | Tier display | Config load |
| `context_limit` | INT | Context window | Config load |
| `status_reason` | TEXT | Warning display | Router updates |
| `cooldown_expires_at` | TIMESTAMPTZ | Cooldown timer | `record_model_failure` |
| `subscription_cost_usd` | DECIMAL | Subscription cost | Config load |
| `tokens_used` | INT | Lifetime tokens | `record_model_success` |
| `tasks_completed` | INT | Task count | `record_model_success` |

---

## Table: platforms

### Dashboard Expects

| Column | Type | Dashboard Uses | Go Code That Writes |
|--------|------|----------------|---------------------|
| `id` | TEXT | Platform identifier | Config load |
| `name` | TEXT | Display name | Config load |
| `status` | TEXT | Platform status | Router updates |
| `type` | TEXT | Type display | Config load |
| `context_limit` | INT | Context window | Config load |
| `config` | JSONB | Free tier info | Config load |

---

## Table: orchestrator_events

### Dashboard Expects

| Column | Type | Dashboard Uses | Go Code That Writes |
|--------|------|----------------|---------------------|
| `event_type` | TEXT | Event type | Router decisions |
| `task_id` | UUID | Task link | Router decisions |
| `model_id` | TEXT | Model involved | Router decisions |
| `runner_id` | TEXT | Runner used | Router decisions |
| `from_runner_id` | TEXT | Previous runner | Model switches |
| `to_runner_id` | TEXT | New runner | Model switches |
| `reason` | TEXT | Event reason | Router decisions |
| `details` | JSONB | Extra context | Router decisions |
| `created_at` | TIMESTAMPTZ | Timestamp | Auto-generated |

### Valid event_type Values

```go
// From core/state.go
const (
    EventTaskAssigned   = "task_assigned"
    EventTaskCompleted  = "task_completed"
    EventTaskFailed     = "task_failed"
    EventModelSwitched  = "model_switched"
    EventRateLimitHit   = "rate_limit_hit"
    EventCooldownStarted = "cooldown_started"
)
```

---

## RPC Allowlist (rpc.go)

### Currently Allowed

```go
var defaultRPCAllowlist = map[string]bool{
    "update_task_status":          true,
    "update_task_assignment":      true,
    "create_task_with_packet":     true,
    "create_task_run":             true,
    "record_model_success":        true,
    "record_model_failure":        true,
    "calculate_run_costs":         true,
    "set_processing":              true,
    "clear_processing":            true,
    "save_checkpoint":             true,
    "delete_checkpoint":           true,
    "update_plan_status":          true,
    "record_performance_metric":   true,
    // ... others
}
```

### Missing (Must Add)

```go
"check_platform_availability": true,  // Used by router.go:213
```

---

## Config-Driven Values

### system.json Structure

```json
{
  "runtime": {
    "default_timeout_seconds": 300,
    "courier_poll_interval_secs": 5,
    "realtime_heartbeat_secs": 30,
    "realtime_reconnect_delay_secs": 5,
    "max_concurrent_per_module": 3,
    "max_concurrent_total": 10
  },
  "validation": {
    "valid_task_types": ["feature", "bug", "fix", "test", "refactor", "lint", "typecheck", "visual", "accessibility"],
    "default_task_type": "feature",
    "default_max_attempts": 3,
    "min_task_confidence": 0.0,
    "require_prompt_packet": true,
    "require_category": true,
    "require_expected_output": true
  },
  "recovery": {
    "max_task_attempts": 3,
    "cooldown_minutes": 30,
    "orphan_threshold_seconds": 300,
    "heartbeat_interval_seconds": 30
  }
}
```

### Go Code That Should Use Config

| Current Hardcode | Location | Config Key |
|------------------|----------|------------|
| `DefaultTimeoutSecs = 300` | runners.go:20 | `runtime.default_timeout_seconds` |
| `CourierPollIntervalSecs = 5` | courier.go:14 | `runtime.courier_poll_interval_secs` |
| `DefaultSessionTimeoutSecs = 300` | session.go:12 | `runtime.default_timeout_seconds` |
| `30 * time.Second` | realtime/client.go:266 | `runtime.realtime_heartbeat_secs` |
| `5 * time.Second` | realtime/client.go:521 | `runtime.realtime_reconnect_delay_secs` |
| `max_attempts: 3` | validation.go | `validation.default_max_attempts` |

---

## Quick Reference: Fix Dashboard Issues

| Dashboard Issue | Root Cause | Fix Location |
|-----------------|------------|--------------|
| Task shows "Unassigned" | `assigned_to` is NULL | handlers_task.go:133-144 |
| Token count shows 0 | `task_runs` not created | handlers_task.go:233-256 |
| ROI shows $0 | Cost fields not calculated | handlers_task.go:598-622 |
| Location shows "Web" not "VibePilot" | `routing_flag` not set | handlers_task.go:131-138 |
| Model not in hangar | `models.status` wrong | Config load or RPC |
| Slice shows "General" | `slice_id` not set | validation.go:147-162 |

---

## Checklist: Before Any Code Change

- [ ] Does this write to Supabase correctly?
- [ ] Does the field match what dashboard expects?
- [ ] Is the value from config, not hardcoded?
- [ ] Is the RPC in the allowlist?
- [ ] Does this pass the vendor swap test?
