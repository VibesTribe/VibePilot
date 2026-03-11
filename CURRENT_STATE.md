# VibePilot Current State
**Last Updated:** 2026-03-11 Session 79 (17:02 UTC)
**Status:** BROKEN - Fundamental issues remain

---

## CRITICAL UNRESOLVED ISSUES

### 1. Dashboard Shows Nothing
- Realtime connected but dashboard not updating
- No modules, tasks, or status changes visible
- Need to check what dashboard expects vs what we send

### 2. Duplicate Events Should Be Impossible
- Task in_progress should NEVER have duplicates
- Multiple kilo processes spawning for same task
- Root cause unknown

### 3. Status Value Confusion
- Tasks: `approval` (after testing) - schema correct
- Plans: `approved` (after supervisor) - schema correct
- But dashboard may expect different values

### 4. Reassignment Logic Broken
- Task should ONLY reassign on supervisor/tester FAIL
- On reassign: branch must be deleted/cleared
- Currently: old output stays, multiple attempts pile up

---

## CORRECT FLOW

**Task statuses:**
`pending → available → in_progress → review → testing → approval → merged`

**Only reassign on:**
- Supervisor: "fail" (with reason)
- Tester: "fail" (with reason)

**On reassign:**
- Delete task branch
- Fresh branch on next attempt

---

## FIXES COMMITTED SESSION 79

| Fix | File |
|-----|------|
| Realtime reconnect loop | `realtime/client.go` |
| Task status "approval" | `handlers_testing.go` |
| Orphan branch clean | `gitree.go` |
| Force push branches | `gitree.go` |
| Force checkout | `gitree.go` |
| Remove missing RPC | `handlers_task.go` |
| Revert to full prompts | `agents.json` |

---

## NEXT SESSION

1. **Read dashboard code** - what status values expected
2. **Fix duplicate events** - why possible?
3. **Fix dashboard updates** - why not showing?
4. **Fix reassignment** - only on fail, with branch cleanup
5. **Stop multiple kilo spawns** for same task

---

## Status Values (Schema)

**Tasks:** `pending, available, in_progress, review, testing, approval, merged, escalated, blocked`

**Plans:** `draft, review, council_review, revision_needed, prd_incomplete, blocked, pending_human, error, approved, active, archived, cancelled`
