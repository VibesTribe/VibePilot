# VibePilot Current State
**Last Updated:** 2026-03-09 Session 72 (06:45 UTC)
**Status:** FLOW ISSUES - Events not triggering properly

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## 🔧 CURRENT ISSUE: Flow Stuck at Various Stages

### Symptoms:
1. Tasks take 1-2 minutes even for simple hello-world
2. Prompt packet not showing in dashboard
3. Tasks stuck in "Queued" status after execution
4. Status transitions not triggering next stage handlers

### Root Cause: Processing Lock Timing
The realtime event fires when status changes, but if `processing_by` is still set, the event is skipped.

**Wrong Pattern:**
```go
defer clearProcessingLock()  // Runs AFTER function returns
updateStatus("review")       // Event fires, but processing_by still set → SKIPPED
```

**Correct Pattern:**
```go
clearProcessingLock()        // Clear FIRST
updateStatus("review")       // Event fires, processing_by is null → PROCESSED
```

---

## 🔧 FIXES APPLIED (Session 72)

### 1. handlers_plan.go - Clear lock BEFORE status update
**File:** `governor/cmd/governor/handlers_plan.go:154-165`
**Change:** Moved `clearProcessingLock()` before `update_plan_status` RPC

### 2. handlers_task.go - Clear lock BEFORE status update
**File:** `governor/cmd/governor/handlers_task.go:361-437`
**Change:** Same pattern in `handleTaskReview` - clear lock before status update
**Also:** Added task_packet and task_run context to supervisor review input

### 3. realtime/client.go - Status-based deduplication
**File:** `governor/internal/realtime/client.go:401-410`
**Change:** Event key now includes old_status → new_status transition
```go
eventKey = fmt.Sprintf("%s:%s:%s:%s->%s", change.Table, id, change.EventType, oldStatus, newStatus)
```
**Why:** Multiple UPDATEs with different status transitions were being deduplicated

---

## 🐛 REMAINING ISSUES

### 1. Prompt Packet Not Showing in Dashboard
**Root Cause:** Dashboard expects `tasks.result.prompt_packet` (jsonb field)
**Current State:** Prompt stored in `task_packets` table (separate table)
**Solution Needed:** 
- Option A: Update `create_task_if_not_exists` RPC to write to `tasks.result`
- Option B: Update dashboard to join with `task_packets` table

### 2. slice_id is NULL
**Root Cause:** `create_task_if_not_exists` RPC doesn't set `slice_id`
**Impact:** Tasks show outside slices on dashboard
**Solution Needed:** Add `p_slice_id` parameter to RPC

### 3. Status Values Mismatch
**Dashboard expects:** `assigned`, `in_progress`, `supervisor_review`, `testing`, `supervisor_approval`, `ready_to_merge`, `complete`
**Governor uses:** `available`, `review`, `testing`, `approval`, `merged`
**Note:** Dashboard has mapping in `HOW_DASHBOARD_WORKS.md` but some don't map cleanly

### 4. Task Stuck After Execution
**Symptom:** Task shows "Queued" with 569 tokens, 26s runtime but never completes
**Possible Cause:** `handleTaskCompleted` not being triggered, or testing → complete flow broken

---

## 📊 LAST TEST RUN (06:42 UTC)

1. PRD pushed: 06:42:23
2. Plan created: 06:42:46 (23s)
3. Plan review started: 06:42:46
4. Supervisor approved: 06:43:12 (26s)
5. Task T001 created: status "available"
6. Task executed: 569 tokens, 26s runtime
7. **STUCK:** Dashboard shows "Queued" - never progressed to complete

---

## 📁 FILES MODIFIED THIS SESSION

1. `governor/cmd/governor/handlers_plan.go` - Clear lock before status update
2. `governor/cmd/governor/handlers_task.go` - Clear lock before status update, pass context to supervisor
3. `governor/internal/realtime/client.go` - Status-based deduplication

---

## 🧹 CLEANUP COMMANDS

```bash
# Kill stuck governor processes
sudo pkill -9 -f "governor/governor"

# Clean port 8080
sudo fuser -k 8080/tcp

# Restart governor
sudo systemctl restart governor

# Clean Supabase test data
sudo bash -c 'source <(systemctl show governor -p Environment | sed "s/Environment=//" | tr " " "\n") && curl -s -X DELETE "${SUPABASE_URL}/rest/v1/task_runs?id=not.is.null" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}" -H "Prefer: return=minimal" && curl -s -X DELETE "${SUPABASE_URL}/rest/v1/task_packets?id=not.is.null" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}" -H "Prefer: return=minimal" && curl -s -X DELETE "${SUPABASE_URL}/rest/v1/tasks?id=not.is.null" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}" -H "Prefer: return=minimal" && curl -s -X DELETE "${SUPABASE_URL}/rest/v1/plans?id=not.is.null" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}" -H "Prefer: return=minimal"'
```

---

## 🚀 NEXT STEPS

### Immediate:
1. Check all handlers follow pattern: clear lock BEFORE status update
2. Investigate why task stuck at "Queued" after execution
3. Fix prompt packet visibility (write to tasks.result or update dashboard)

### Investigation:
1. Is `handleTaskCompleted` being triggered?
2. Is testing → complete flow working?
3. Check `handleTaskAvailable` - does it clear lock before updating to "review"?

### Performance:
1. Consider reducing agent timeouts
2. Consider direct RPC calls for immediate transitions instead of realtime events
3. Consider single transaction for lock clear + status update

---

## 📚 KEY DOCUMENTATION

- `/home/mjlockboxsocial/vibepilot/docs/HOW_DASHBOARD_WORKS.md` - Dashboard data expectations
- `/home/mjlockboxsocial/vibepilot/docs/supabase-schema/` - Database schema migrations
- `/home/mjlockboxsocial/vibepilot/governor/config/` - Agent and connector configs

---

## 🖥️ SERVICE INFO

- **Governor binary:** `/home/mjlockboxsocial/vibepilot/governor/governor`
- **Service:** `sudo systemctl restart governor`
- **Logs:** `journalctl -u governor -f`
- **Active connectors:** kilo (cli)
- **Active models:** glm-5 (via kilo)
- **Concurrency:** 3 per module, 3 total

---

## 📜 SESSION HISTORY

- **72:** Fixed processing lock timing in handlers, status-based dedup, task context for supervisor
- **71:** Deep analysis + 4 fixes (pool failure lock, processing_by check, event dedup, migration 077)
- **70:** Fixed endless session spawning bug
- **69:** Applied duplicate task fix
