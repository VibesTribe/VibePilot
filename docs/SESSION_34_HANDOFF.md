# Session 34 Handoff

**Date:** 2026-02-27
**Agent:** GLM-5
**Branch:** main
**Commits:** 3 (cleanup fix, event persistence, startup recovery)

---

## What Was Accomplished

### 1. Fixed "signal: terminated" Bug ✅
- Root cause: `cleanup_zombies.sh` killed governor children
- Fix: Check cgroup membership before killing
- Verified: Planner now runs successfully

### 2. Event Persistence & Recovery ✅
- Schema 032 applied to Supabase
- Tables: `event_checkpoints`, `runner_sessions`, `event_queue`, `system_config`
- 8 new RPCs for recovery operations
- Startup recovery wired in main.go

### 3. Usage Tracking System ✅
- Multi-window tracking (minute/hour/day/week)
- 80% buffer enforcement
- Auto-calculated request spacing
- Cooldown countdown per model

### 4. Model Profiles ✅
- Full `models.json` with rate limits, pricing, recovery config
- `model_loader.go` syncs to DB
- All values configurable, no hardcoded defaults

### 5. Config Improvements ✅
- session.go uses config for timeout/maxTurns
- events.go uses configurable query limits
- runners.go has configurable CLI args

### 6. GCE Cleanup ✅
- Removed OpenClaw, Docker, Playwright, Python caches
- Saved ~3GB disk, ~330MB RAM

---

## What's NOT Done (Critical)

### Tool Protocol Mismatch

**The Problem:**
```
Governor expects:  TOOL: db_update {"table": "tasks", ...}
OpenCode does:     Uses its own built-in tools, ignores TOOL: format
```

**Impact:**
- Agents can't reliably do database operations
- Supervisor can't update task status through expected flow
- Learning RPCs (`record_model_success`, `record_model_failure`) never called

**Options:**
1. **TOOL: adapter** - Parse OpenCode output, convert TOOL: calls to actual operations
2. **OpenCode tool server** - Implement tools OpenCode can call
3. **Different protocol** - Change how governor expects tool calls

### Learning Loop Not Connected

**What exists:**
- Schema: `models.learned` with best_for, avoid_for, failure_rates
- RPCs: `record_model_success`, `record_model_failure` in database
- Allowlist: Both RPCs in allowlist

**What's missing:**
- Nothing calls these RPCs after task completion
- Orchestrator makes decisions on config, not learned data
- Supervisor prompt mentions tracking but doesn't execute TOOL: calls

**To wire it:**
```go
// After task completion in main.go or supervisor handling:
database.RPC(ctx, "record_model_success", map[string]interface{}{
    "p_model_id": modelID,
    "p_task_type": taskType,
    "p_duration_seconds": duration,
})
```

---

## Files Changed This Session

| File | Change |
|------|--------|
| `scripts/cleanup_zombies.sh` | Check cgroup before killing |
| `governor/config/models.json` | Full model profiles with rate limits |
| `governor/config/system.json` | Added recovery and defaults sections |
| `governor/config/destinations.json` | Added cli_args field |
| `governor/cmd/governor/main.go` | Startup recovery, model loading |
| `governor/internal/db/rpc.go` | Added 7 new RPCs to allowlist |
| `governor/internal/runtime/config.go` | Recovery, Defaults fields, GetRuntimeConfig |
| `governor/internal/runtime/events.go` | Configurable query limits |
| `governor/internal/runtime/session.go` | Use config for timeout/maxTurns |
| `governor/internal/runtime/usage_tracker.go` | NEW - Multi-window tracking |
| `governor/internal/runtime/model_loader.go` | NEW - Sync models.json to DB |
| `governor/internal/destinations/runners.go` | Configurable CLI args |
| `docs/supabase-schema/032_event_persistence.sql` | NEW - Event tables + RPCs |
| `CURRENT_STATE.md` | Updated |

---

## Config Reference

### system.json - Recovery Section
```json
{
  "recovery": {
    "heartbeat_interval_seconds": 30,
    "orphan_threshold_seconds": 300,
    "stuck_dependency_threshold_hours": 2,
    "human_approval_timeout_days": 3,
    "max_task_attempts": 3,
    "model_failure_threshold": 3,
    "usage_threshold_pct": 80,
    "auto_bench_on_failures": true,
    "auto_restore_after_cooldown": true
  }
}
```

### models.json - Model Profile Structure
```json
{
  "id": "gemini-2.5-flash",
  "rate_limits": {
    "requests_per_minute": 15,
    "requests_per_day": 1500,
    "tokens_per_day": 1000000
  },
  "throttle_behavior": "hard_cutoff",
  "buffer_pct": 80,
  "spacing_min_seconds": 5,
  "recovery": {
    "on_rate_limit": "cooldown",
    "cooldown_minutes": 60
  },
  "api_pricing": {
    "input_per_1m_usd": 0.30,
    "output_per_1m_usd": 2.50
  },
  "learned": {
    "avg_task_duration_seconds": null,
    "failure_rate_by_type": {},
    "best_for_task_types": [],
    "avoid_for_task_types": []
  }
}
```

---

## Database Tables Added

### event_checkpoints
```sql
source TEXT PRIMARY KEY,
last_seen_at TIMESTAMPTZ NOT NULL,
updated_at TIMESTAMPTZ DEFAULT NOW()
```

### runner_sessions
```sql
id UUID PRIMARY KEY,
task_id UUID REFERENCES tasks(id),
destination_id TEXT NOT NULL,
model_id TEXT,
started_at TIMESTAMPTZ,
last_heartbeat TIMESTAMPTZ,
status TEXT,  -- running, completed, orphaned, failed
failure_reason TEXT,
tokens_in INT,
tokens_out INT
```

### event_queue
```sql
id UUID PRIMARY KEY,
event_type TEXT NOT NULL,
source_table TEXT NOT NULL,
record_id TEXT NOT NULL,
payload JSONB,
status TEXT,  -- pending, processing, completed, failed
attempts INT,
last_error TEXT
```

### system_config
```sql
key TEXT PRIMARY KEY,
value JSONB NOT NULL
```

---

## RPCs Added

| RPC | Purpose |
|-----|---------|
| `update_event_checkpoint` | Persist last processed timestamp |
| `get_event_checkpoint` | Get checkpoint for source |
| `find_orphaned_sessions` | Find sessions with no heartbeat |
| `recover_orphaned_session` | Mark orphaned, reset task to available |
| `record_model_failure` | Track failure, auto-bench if threshold |
| `record_model_success` | Reset failures, update learned data |
| `check_model_availability` | Check rate limits + cooldown |

---

## Verification Commands

```bash
# Check governor status
systemctl status vibepilot-governor

# Check recovery ran
journalctl -u vibepilot-governor | grep -i recovery

# Check tables exist
psql -c "SELECT * FROM event_checkpoints;"

# Check model profiles loaded
curl "$SUPABASE_URL/rest/v1/models?select=id,rate_limits,learned" \
  -H "apikey: $SUPABASE_SERVICE_KEY"
```

---

## Next Session Priorities

1. **Tool Protocol** - Decide approach, implement solution
2. **Wire Learning Loop** - Connect success/failure RPCs to task completion
3. **End-to-End Test** - Run full task, verify all pieces work together
4. **Vibes Agent** - Audio/visual dashboard agent (future)

---

## Key Principles to Maintain

- **No hardcoded values** - Everything in config files
- **No secrets in code** - Use vault or env vars
- **Configurable per model** - Different models have different behaviors
- **Learning from experience** - System should improve over time
- **Graceful degradation** - Failures should route, not crash
