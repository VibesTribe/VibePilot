# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SYSTEM_REFERENCE.md`** ← **WHAT WE HAVE AND HOW IT WORKS** (start here!)
3. **`docs/GOVERNOR_HANDOFF.md`** ← **GO GOVERNOR STATUS** (what's done, what's next)
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`docs/prd_v1.4.md`** - Complete system specification

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-24
**Updated By:** GLM-5 - Session 28: Learning System
**Session Focus:** Security audit + Learning system architecture (Phase 1)
**Direction:** Learning infrastructure schema created, ready for Go implementation

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Go Governor Phase 1-6 COMPLETE, Learning Phase 1 Schema READY

---

# SESSION 28: LEARNING SYSTEM (2026-02-24)

## What We Did

### 1. Security Audit + Fixes

**7 security hardening fixes committed:**
- Vault key caching at init (not per-decrypt)
- WebSocket origin check fix (prevent `evil-vercel.app` bypass)
- Config validation (fail fast on missing Supabase creds)
- Error body truncation (prevent secret leaks in logs)
- Courier result timeout (30s context)
- Orchestrator goroutine lifecycle (clean shutdown)
- Git add error checking (no silent failures)

**1 race condition fix:**
- Hub RLock → Lock when modifying clients map

### 2. Learning System Architecture

**Full plan:** `docs/LEARNING_SYSTEM_PLAN.md`

**Core principle:**
- Go = Fast, deterministic, free (90%)
- LLM = Smart, adaptive, costs tokens (10%)
- Supabase = Truth (everything here)

### 3. Phase 1 Schema Created

**File:** `docs/supabase-schema/024_learning_system.sql`

**New tables:**
| Table | Purpose |
|-------|---------|
| `learned_heuristics` | Model preferences per task type |
| `failure_records` | Structured failure logging |
| `problem_solutions` | What fixed what |

**New RPCs:**
- `record_failure` - Log structured failure
- `get_heuristic` - Get routing preference
- `get_problem_solution` - Find proven fix
- `record_heuristic_result` - Track heuristic success
- `record_solution_result` - Track solution success
- `get_recent_failures` - For routing exclusions
- `upsert_heuristic` - LLM updates heuristics

## Files Changed This Session

| File | Change |
|------|--------|
| `governor/internal/vault/vault.go` | Cache vault key at init |
| `governor/internal/server/server.go` | WebSocket origin fix |
| `governor/internal/config/config.go` | Required field validation |
| `governor/internal/db/supabase.go` | Error body truncation |
| `governor/internal/dispatcher/dispatcher.go` | Timeout context |
| `governor/internal/orchestrator/orchestrator.go` | Goroutine lifecycle |
| `governor/internal/maintenance/maintenance.go` | Git add error check |
| `governor/internal/server/hub.go` | RLock → Lock fix |
| `docs/LEARNING_SYSTEM_PLAN.md` | NEW - Full implementation plan |
| `docs/supabase-schema/024_learning_system.sql` | NEW - Phase 1 schema |
| `docs/GOVERNOR_HANDOFF.md` | Updated for Session 28 |

## Commits

```
5f537459 fix: use Lock instead of RLock when modifying hub clients map
0b30e3bc Security hardening and defensive fixes
```

## Code Stats

```
Total Go files: 24
Total lines:   4,949 (+48 from Session 27)
Build:         ✅ Clean
Vet:           ✅ No issues
```

## Next Steps (Phase 1 Implementation)

1. **Apply migration** `024_learning_system.sql` to Supabase
2. **Go changes:**
   - Add failure recording to `orchestrator.go`
   - Add heuristic checking to `pool/model_pool.go`
   - Add problem/solutions lookup
   - Exclude recently failed models from routing

---

# SESSION 27: STATELESS ORCHESTRATOR (2026-02-24)

## What We Did

### 1. Schema Migration (023_orchestrator_state.sql)

**New columns:**
- `runners.max_concurrent` - Maximum concurrent tasks per runner
- `runners.current_in_flight` - Current tasks in progress
- `tasks.routing_history` - JSONB array of routing decisions

**New tables:**
- `orchestrator_events` - Audit trail of all orchestrator decisions
- `security_audit` - Sensitive operation tracking

**New RPCs:**
- `log_orchestrator_event` - Record decision to event log
- `append_routing_history` - Add routing step to task
- `increment_in_flight` - Atomic concurrent capacity check
- `decrement_in_flight` - Release concurrent slot
- `get_system_state` - Full snapshot for orchestrator
- `log_security_audit` - Track vault access

**Security:**
- Vault RLS hardened (no bulk export/delete for authenticated)

### 2. Vault Module (internal/vault/vault.go)

Go Governor can now access encrypted secrets:
- Fernet decryption (matches Python vault_manager.py)
- In-memory caching (5 min TTL)
- Audit logging to security_audit table

### 3. Orchestrator Event Logging

Every decision logged:
- task_dispatched, runner_selected, task_complete
- supervisor_approve, supervisor_reject, awaiting_human
- visual_test_passed, visual_test_failed
- escalated, analysis_complete, rerouted

### 4. Concurrent Tracking

Dispatcher uses atomic RPCs:
- `increment_in_flight(runner_id)` before task
- `decrement_in_flight(runner_id)` after completion

### 5. Systemd Service

Created `scripts/governor.service` for production deployment.

## Files Changed

| File | Change |
|------|--------|
| `docs/supabase-schema/023_orchestrator_state.sql` | NEW - Schema migration |
| `internal/vault/vault.go` | NEW - Vault access module |
| `internal/orchestrator/orchestrator.go` | Event logging added |
| `internal/dispatcher/dispatcher.go` | In-flight tracking + event logging |
| `internal/db/supabase.go` | New RPCs for events/concurrent |
| `scripts/governor.service` | NEW - Systemd service |

## Final Code Stats

```
Total Go files: 24
Total lines:   4,901
Build:         ✅ Clean
Vet:           ✅ No issues
```

## Branch Status

| Repo | Branch | Status |
|------|--------|--------|
| vibepilot | `go-governor` | Phase 6 complete, ready to push |
| vibeflow | `main` | Production with merge pending UI |
| vibeflow | `vibeflow-test` | Staging (can merge to main) |

## Go Governor Status

See `docs/GOVERNOR_HANDOFF.md` for full details.

**Done:**
- Supervisor 3 actions (APPROVE, REJECT, HUMAN) for task outputs
- Supervisor 3 actions (APPROVE, REJECT, COUNCIL) for plans/research
- Council reviews PLANS and RESEARCH SUGGESTIONS
- Visual testing agent (stub) before human review
- System Researcher for escalated task analysis
- All hardcoded values configurable
- Event logging to orchestrator_events
- Concurrent capacity tracking
- Vault access from Go
- Security audit trail
- Systemd service

**Stub Remaining:**
- `visual/visual.go` - `TestVisual()` passes by default (needs real implementation)
- `maintenance.go` - No command queue polling yet

## Config Options

```yaml
governor:
  poll_interval: 15s
  max_concurrent: 3
  stuck_timeout: 10m
  max_per_module: 8
  task_timeout_sec: 300
  council_max_rounds: 4
```

## Dashboard Status

**Live at Vercel** - auto-deploys from `main` branch.

**Event log now available:** `orchestrator_events` table feeds dashboard logs modal.

**Key status mappings:**
| DB Status | Dashboard Status | Flags? |
|-----------|------------------|--------|
| `awaiting_human` | `supervisor_review` | YES - needs review |
| `approval` | `supervisor_approval` | YES - merge pending badge |
| `testing` | `testing` | NO |
| `merged` | `complete` | NO |
| `failed`/`escalated` | `blocked` | NO |

## Active Models

| Model ID | Status | Notes |
|----------|--------|-------|
| glm-5 (opencode) | ✅ ACTIVE | Only working runner |
| kimi-cli | BENCHED | Subscription cancelled |
| gemini-api | PAUSED | Quota exhausted |
| deepseek-chat | PAUSED | Credit needed |

---

# NEXT SESSION

1. **Apply `024_learning_system.sql`** to Supabase
2. **Go implementation:**
   - Add `RecordFailure()` to db/supabase.go
   - Add `GetHeuristic()` to db/supabase.go  
   - Add `GetProblemSolution()` to db/supabase.go
   - Modify `pool/model_pool.go` to check heuristics + recent failures
   - Modify `orchestrator.go` to record structured failures
3. **Test:** Verify routing uses learned patterns
4. **Later:** Visual testing, maintenance polling, planner learning

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat CURRENT_STATE.md` | This file |
| `cat docs/GOVERNOR_HANDOFF.md` | Go Governor status |
| `cat AGENTS.md` | Mental model + workflow |
| `cd ~/vibepilot/governor && go build ./...` | Build Go Governor |
| `cd ~/vibeflow && npm run typecheck` | Check vibeflow types |

---

# FILES MODIFIED THIS SESSION

| File | Change |
|------|--------|
| `docs/supabase-schema/023_orchestrator_state.sql` | NEW - Schema migration |
| `governor/internal/vault/vault.go` | NEW - Vault access module |
| `governor/internal/orchestrator/orchestrator.go` | Event logging |
| `governor/internal/dispatcher/dispatcher.go` | In-flight tracking + event logging |
| `governor/internal/db/supabase.go` | New RPCs |
| `scripts/governor.service` | NEW - Systemd service |
| `docs/GOVERNOR_HANDOFF.md` | Updated for Session 27 |
