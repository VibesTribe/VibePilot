# VibePilot Current Issues

**Last Updated:** 2026-03-07
**Source:** Strategic Go Code Audit - Session 59

---

## 🔴 Blocking Issues (Must fix before flow works)

### 1. Schema `type` Constraint Violation

**Location:** Supabase `tasks` table
**Problem:** Tasks fail creation with error: `type` field must not match allowed values

**Impact:** All task creation fails

**Fix:** Create migration `067_fix_task_type_check.sql`:
```sql
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_type_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility'));
```

**Go Code:** None (schema change only)

---

### 2. Missing RPC in Allowlist

**Location:** `governor/internal/db/rpc.go`
**Problem:** `check_platform_availability` RPC not in allowlist

**Impact:** Router silently fails to check web platform availability

**Fix:** Add to `defaultRPCAllowlist`:
```go
"check_platform_availability": true,
```

**Go Code:** `governor/internal/db/rpc.go:10-117`

---

### 3. `max_attempts` Not Passed to Task Creation

**Location:** `governor/cmd/governor/validation.go:147-162`
**Problem:** `create_task_with_packet` RPC doesn't receive `max_attempts` parameter

**Impact:** Tasks may retry infinitely or fail unexpectedly

**Fix:** Add parameter to RPC call:
```go
"p_max_attempts": 3,
```

**Go Code:** `governor/cmd/governor/validation.go:147-162`

---

## 🟡 Hardcoding Issues (config-driven instead)

### 4. Timeout Values Hardcoded

| Location | Constant | Current Value | Should Be |
|----------|----------|---------------|----------|
| `connectors/runners.go:20` | `DefaultTimeoutSecs` | 300 | `system.json` |
| `connectors/courier.go:14` | `CourierPollIntervalSecs` | 5 | `system.json` |
| `runtime/session.go:12` | `DefaultSessionTimeoutSecs` | 300 | `system.json` |
| `realtime/client.go:266` | Heartbeat interval | 30s | `system.json` |
| `realtime/client.go:521` | Reconnect delay | 5s | `system.json` |

**Fix:** Add to `system.json`:
```json
{
  "runtime": {
    "courier_poll_interval_secs": 5,
    "realtime_heartbeat_secs": 30,
    "realtime_reconnect_delay_secs": 5
  }
}
```

---

## 🟡 Data Flow Gaps (Dashboard Impact)

### 5. Token Extraction from CLI

**Status:** ✅ Working

**Location:** `governor/internal/connectors/runners.go:89-131`

The CLI output is parsed for token counts in `step_finish` events.

### 6. task_runs Creation

**Status:** ✅ Working

**Location:** `governor/cmd/governor/handlers_task.go:233-256`

    Creates `task_runs` record with all required fields.

### 7. Cost Calculation

**Status:** ✅ Working

**Location:** `governor/cmd/governor/handlers_task.go:598-622`

    Calls `calculate_run_costs` RPC to computes ROI.

### 8. Task Assignment Fields

**Status:** ✅ Working

| Field | Location |
|------|----------|
| `assigned_to` | `handlers_task.go:133-144` |
| `routing_flag` | `handlers_task.go:131-138` |

---

## 📋 Recommended Fix Order

1. **Add RPC to allowlist** (`rpc.go`)
2. **Pass `max_attempts` to RPC** (`validation.go`)
3. **Create schema migration** for `type` constraint
4. **Externalize hardcoded timeouts** to config

---

## 📁 Files Referenced

| File | Purpose |
|------|---------|
| `governor/internal/db/rpc.go` | RPC allowlist |
| `governor/cmd/governor/validation.go` | Task creation |
| `governor/cmd/governor/handlers_task.go` | Task execution |
| `governor/internal/connectors/runners.go` | CLI execution |
| `docs/supabase-schema/034_task_improvements.sql` | Schema reference |
| `docs/HOW_DASHBOARD_WORKS.md` | Dashboard expectations |
| `docs/DATA_FLOW_MAPPING.md` | Data flow details |

---

## 🔗 Related Documentation

- [HOW_DASHBOARD_WORKS.md](HOW_Dashboard Works.md) - Dashboard data expectations
- [DATA_FLOW_MAPPING.md](Data Flow Mapping.md) - Go code → Supabase mapping
- [VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md](VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md) - Core principles

---

**For implementation details, see individual issue sections above.**
