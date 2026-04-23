# VibePilot Current Issues
> Last updated: April 23, 2026
> Previous: April 22, 2026

## Status Summary

| Category | Open | Fixed | Deferred |
|----------|------|-------|----------|
| Database migration | 0 | 5 | 0 |
| SSE bridge | 1 | 2 | 0 |
| Infrastructure | 2 | 1 | 0 |
| Pipeline gaps | 5 | 0 | 0 |
| Learning system | 4 | 0 | 0 |

---

## SSE Bridge Issues

### 1. SSE Not End-to-End Tested
**Priority**: P1
**Impact**: Code compiles clean, wiring verified in code, but never run with governor + dashboard together
**Status**: Code complete, needs live test
**Files**: sse.go, listener.go, server.go, main.go, useMissionData.ts

---

## Infrastructure Issues

### 2. systemd Service Not Enabled
**Priority**: Medium
**Impact**: Governor runs as manual process (PID 95117). Won't survive reboot.
**Fix**: `systemctl --user enable governor`

### 3. Courier Still Writes to Supabase
**Priority**: High (pipeline broken for courier tasks)
**Impact**: courier_run.py writes results to Supabase REST, but realtime listener is gone. Governor will never see courier results.
**Fix Needed**: Update courier_run.py to write to governor API or directly to local PG

---

## Pipeline Gaps (pre-existing)

### 4. Module Branches Never Created
**Location**: `governor/internal/gitree/gitree.go:387`
**Impact**: Task branches have nowhere to merge to. Merge fails.
**Fix**: Call `CreateModuleBranch()` after task creation in `handlers_plan.go`

### 5. Maintenance Agent Not Wired
**Impact**: Git write access disconnected, maintenance commands go nowhere

### 6. Worktrees Disabled
**Impact**: All tasks share same directory, no isolation

### 7. Orchestrator Not an LLM Call
**Impact**: Just hardcoded cascade in Go, no intelligent routing

### 8. Consultant Agent Not Wired
**Impact**: PRD template and prompt exist but aren't integrated into governor flow

---

## Learning System Gaps (pre-existing)

### 9. Supervisor Rules Not Created from Rejections
**Location**: `governor/cmd/governor/handlers_task.go`
**Impact**: System doesn't learn from supervisor rejections
**Fix**: Call `create_supervisor_rule` RPC in rejection handler

### 10. Tester Rules Never Created
**Location**: `governor/cmd/governor/handlers_testing.go`
**Impact**: No learning from test failures
**Fix**: Call `create_tester_rule` RPC on test failure

### 11. Heuristics Never Recorded from Task Outcomes
**Impact**: Router doesn't learn model preferences per task type
**Fix**: Call `upsert_heuristic` RPC on task success
**Note**: `learned_heuristics` table has 20 entries (from direct DB inserts), but handler doesn't automatically create them

### 12. Problem-Solutions Never Recorded
**Impact**: Same failures repeat, no automatic remediation
**Fix**: Call `record_solution_result` RPC on retry success

---

## Recently Fixed (April 23, 2026)

| # | Issue | Fix | Status |
|---|-------|-----|--------|
| 1 | Dashboard polling every 5s | SSE EventSource replaces polling | Code done, not E2E tested |
| 2 | Supabase Realtime dependency removed | pg_notify + SSE bridge | Code done, not E2E tested |

## Previously Fixed (April 22, 2026)

| # | Issue | Fix | Commit |
|---|-------|-----|--------|
| 3 | Rehydration SQL ordering (`ORDER BY` before `WHERE`) | Collect clauses separately in `buildSelectQuery` | e3767ba5 |
| 4 | UUID `[16]byte` not converted to string | Added case in `convertValue()` | e3767ba5 |
| 5 | Timestamp parse warnings (Go Time.String format) | Added `parseTime()` with fallback formats | e3767ba5 |
| 6 | Routing referenced nonexistent `gemini-2.5-flash` | Changed to `gemini-2.5-flash-lite` in routing.json | e3767ba5 |
| 7 | Supabase polling pounding remote DB | Native pgx backend replaces Supabase entirely | ffd29bfa |

## Previously Fixed (March-April 2026)

| # | Issue | Fix | Commit |
|---|-------|-----|--------|
| 8 | Testing event mapped wrong | Fixed `status == "testing"` to emit `EventTaskTesting` | Session 73 |
| 9 | Failure notes not recorded in testing | Added `recordFailureNotes()` calls | Session 73 |
| 10 | Plan review race condition | Retry loop (3 attempts, 3s sleep) | April 21 |
| 11 | Stale lock cleanup | Recovery.go cleanup | April 21 |
| 12 | Supervisor rubber-stamping | Intelligence overhaul (code map + verification) | 57654556 |
| 13 | Planner over-engineering | Code map context + targeted file injection | 57654556 |

---

## Non-Issues (log noise only)

- jcodemunch CodeMap refresh transport error on startup -- graceful fallback to existing map.md, does not affect operation

---

## Fix Priority Order

| Priority | Issue | Effort | Files |
|----------|-------|--------|-------|
| P1 | SSE E2E test | Low | Start governor, open dashboard, verify live |
| P2 | Courier writes to local PG instead of Supabase | Medium | courier_run.py |
| P3 | Enable systemd service | Trivial | systemctl |
| P4 | Module branch creation | Medium | handlers_plan.go |
| P5 | Supervisor rule creation | Low | handlers_task.go |
| P6 | Tester rule creation | Low | handlers_testing.go |
| P7 | Heuristic recording | Low | handlers_task.go |
