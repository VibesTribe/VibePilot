# VibePilot Current State
**Last Updated:** 2026-03-09 Session 71
**Status:** COMPREHENSIVE FIXES APPLIED - Ready for testing

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## 🔧 FIXES APPLIED (Session 71)

### Bug: Endless Session Spawning
**Symptom:** Governor spawns multiple kilo sessions for the same task, overwhelming the system

### Root Causes Found (5 Critical Bugs):

1. **Processing lock not cleared on pool failure**
   - Location: `handlers_task.go:handleTaskAvailable`
   - If pool.SubmitWithDestination fails, `handleTaskError` was called but processing lock wasn't cleared
   - Task stuck with `processing_by` set, never picked up again

2. **Realtime doesn't check processing_by**
   - Location: `realtime/client.go:mapToEventType`
   - Events fired even when task/plan already had `processing_by` set
   - Multiple handlers could start for same task

3. **No event deduplication**
   - Location: `realtime/client.go:handlePostgresChange`
   - Duplicate events from reconnects/replays could race past processing lock
   - No short-term memory of seen events

4. **No unique constraint on (plan_id, task_number)**
   - Location: Database schema
   - Allows duplicate tasks to be created

5. **Atomic task creation RPC missing**
   - Location: `validation.go:createTasksFromApprovedPlan`
   - Race condition in task creation

### Fixes Applied:

**handlers_task.go:**
- Added `clear_processing` call when pool submission fails (line 182-185)

**realtime/client.go:**
- Added `seenEvents` map and mutex for deduplication
- Added `processing_by` check in `mapToEventType` for tasks and plans
- Added `isDuplicateEvent()`, `markEventSeen()`, `cleanupOldEvents()`, `cleanupSeenEvents()`
- Events with `processing_by` set are now skipped
- Duplicate events (same table:id:eventType within 30s) are skipped

**Migration 077 (needs to be applied):**
- Unique constraint on `tasks(plan_id, task_number)`
- Atomic `create_task_if_not_exists` RPC with ON CONFLICT

---

## Database Migration Required

Apply `docs/supabase-schema/077_prevent_duplicate_tasks.sql` in Supabase SQL Editor

---

## Next Steps

1. Apply migration 077 to Supabase
2. Rebuild governor: `cd ~/vibepilot/governor && go build -o governor ./cmd/governor`
3. Enable and start governor: `sudo systemctl enable governor && sudo systemctl start governor`
4. Test with simple PRD
5. Verify only 1 session per task
6. Verify flow completes

---

## System Status
- Governor: stopped (disabled)
- Sessions: 1 (this interactive session only)
- Supabase: clean (no tasks/plans) - NEEDS MIGRATION 077
- GitHub: on branch task/T001

---

## Configuration
- **Active connectors:** kilo (cli)
- **Active models:** glm-5 (via kilo)
- **Concurrency:** 2 per module, 2 total

---

## Session History
- **71:** Deep analysis + 4 code fixes (pool failure lock clear, processing_by check, event dedup, cleanup routine)
- **70:** Fixed endless session spawning bug (processing lock timing, unique constraint, atomic RPC)
- **69:** Applied duplicate task fix, ready for testing
- **68:** Branch creation from source, merge fixes, disabled gemini-api
- **67:** Gemini API activated, status mapping fixed, removed escalated status
