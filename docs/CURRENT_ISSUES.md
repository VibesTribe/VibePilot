# VibePilot Current Issues

**Last Updated:** 2026-03-07
**Source:** Strategic Go Code Audit - Session 59

---

## 🔴 Blocking Issues (Must fix before flow works)

### 1. Schema `type` Constraint Violation

**Location:** Supabase `tasks` table
**Problem:** Tasks fail creation with error: `type` field must match allowed values
**Impact:** All task creation fails

**Fix:** Create migration `docs/supabase-schema/067_fix_task_type_check.sql`:
```sql
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_type_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility'));
```

**Go Code:** None (schema change only)

---

### 2. Missing RPC in Allowlist

**Location:** `governor/internal/db/rpc.go:10-117`
**Problem:** `check_platform_availability` RPC not in allowlist
**Impact:** Router silently fails to check web platform availability

**Fix:** Add to `defaultRPCAllowlist`:
```go
"check_platform_availability": true,
```

**File:** `governor/internal/db/rpc.go`

---

### 3. `max_attempts` Not Passed to Task Creation

**Location:** `governor/cmd/governor/validation.go:147-162`
**Problem:** `create_task_with_packet` RPC doesn't receive `max_attempts` parameter
**Impact:** Tasks may retry infinitely or fail unexpectedly

**Fix:** Add parameter to RPC call:
```go
"p_max_attempts": 3,
```

**File:** `governor/cmd/governor/validation.go`

---

## 🟡 Hardcoding Issues (config-driven instead)

### 4. Timeout Values Hardcoded

| Location | Constant | Current | Config Key |
|----------|----------|---------|------------|
| `connectors/runners.go:20` | `DefaultTimeoutSecs` | 300 | `runtime.default_timeout_seconds` |
| `connectors/courier.go:14` | `CourierPollIntervalSecs` | 5 | `runtime.courier_poll_interval_secs` |
| `runtime/session.go:12` | `DefaultSessionTimeoutSecs` | 300 | `runtime.default_timeout_seconds` |
| `realtime/client.go:266` | Heartbeat | 30s | `runtime.realtime_heartbeat_secs` |
| `realtime/client.go:521` | Reconnect delay | 5s | `runtime.realtime_reconnect_delay_secs` |

**Fix:** Add to `governor/config/system.json`:
```json
{
  "runtime": {
    "default_timeout_seconds": 300,
    "courier_poll_interval_secs": 5,
    "realtime_heartbeat_secs": 30,
    "realtime_reconnect_delay_secs": 5
  }
}
```

---

## 🟢 Working Correctly

| Component | Location | Status |
|-----------|----------|--------|
| task_runs creation | handlers_task.go:233-256 | ✅ Working |
| Token extraction | runners.go:89-131 | ✅ Working |
| Cost calculation | handlers_task.go:598-622 | ✅ Working |
| assigned_to field | handlers_task.go:133-144 | ✅ Working |
| routing_flag field | handlers_task.go:131-138 | ✅ Working |

---

## 📋 Fix Order

1. **Add RPC to allowlist** - `governor/internal/db/rpc.go`
2. **Pass max_attempts to RPC** - `governor/cmd/governor/validation.go`
3. **Create schema migration** - `docs/supabase-schema/067_fix_task_type_check.sql`
4. **Externalize hardcoded values** - `governor/config/system.json`

---

## 📁 Key Files

| File | Purpose |
|------|---------|
| `governor/internal/db/rpc.go` | RPC allowlist |
| `governor/cmd/governor/validation.go` | Task creation |
| `governor/cmd/governor/handlers_task.go` | Task execution |
| `governor/internal/connectors/runners.go` | CLI execution |
| `governor/config/system.json` | Runtime config |

---

## 🔗 Related Docs

- [SUPABASE_CODE_MAPPING.md](SUPABASE_CODE_MAPPING.md) - Go → Supabase → Dashboard mapping
- [HOW_DASHBOARD_WORKS.md](HOW_DASHBOARD_WORKS.md) - Dashboard data expectations
- [DATA_FLOW_MAPPING.md](DATA_FLOW_MAPPING.md) - Data flow details
