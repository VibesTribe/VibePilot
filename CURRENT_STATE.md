# VibePilot Current State
**Last Updated:** 2026-03-09 Session 74 (19:30 UTC)
**Status:** LEARNING SYSTEM FIXES APPLIED - Ready for testing

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## ✅ SESSION 74 COMPLETED

### 1. Module Branch Creation Fixed
- `CreateModuleBranch()` now called when tasks are created
- Added `git` parameter to `createTasksFromApprovedPlan`
- Added `git` to `CouncilHandler` and plan handlers
- Module branches created for each unique `slice_id`

### 2. Learning System Fixes Applied
- **Supervisor rules**: Now created on rejection (both review and completion)
- **Tester rules**: Now created on test failure and needs_fix
- **Heuristics**: Now recorded on task success for routing optimization

### 3. Docs Updated
- `HOW_DASHBOARD_WORKS.md`: Corrected to show Realtime (not polling)
- `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md`: Updated dashboard section

---

## 🔴 REMAINING ISSUES

### Priority 1: Multiple Kilo Spawning
**Problem:** Extra kilo processes spawned without task activity
**Status:** Under investigation - not caused by governor (logs show idle)
**Possible causes:** Previous session cleanup, external triggers

### Priority 2: Problem-Solutions Never Recorded
**Problem:** No code calls `record_solution_result`
**Impact:** System doesn't learn what fixes work for what problems
**Fix:** Add call when retry succeeds after initial failure

### Priority 3: Schema Consolidation
**Problem:** 95 migrations is unmaintainable
**Impact:** Confusion, hard to debug
**Effort:** High

---

## 📊 AUDIT FINDINGS

### Tables: 49 Total
- **8 actively used** by governor
- **6 learning tables** (3 empty, 3 populated)
- **35 legacy/unused** (verify dashboard usage before removal)

### RPCs: 121 Total
- **34 actually called** in code
- **87 in allowlist** but never called
- **2 missing** from allowlist

### Learning System Status (After Session 74 Fixes)
| Component | Table Rows | Status |
|-----------|------------|--------|
| Supervisor rules | 42+ | ✅ Now creating on rejection |
| Failure records | 332 | ✅ Working |
| Tester rules | 0 → N/A | ✅ Now creating on test failure |
| Heuristics | 0 → N/A | ✅ Now recording on success |
| Problem solutions | 0 | ⚠️ Still not created |
| Lessons learned | 0 | ⚠️ Not populated |

---

## 📁 KEY DOCS

- [CURRENT_ISSUES.md](docs/CURRENT_ISSUES.md) - Full issue list with fixes
- [SUPABASE_AUDIT_2026-03-09.md](docs/SUPABASE_AUDIT_2026-03-09.md) - Raw audit data
- [HOW_DASHBOARD_WORKS.md](docs/HOW_DASHBOARD_WORKS.md) - Dashboard expectations

---

## 🖥️ SERVICE INFO

- **Governor:** Active (port 8080)
- **Active connectors:** kilo (cli)
- **Active models:** glm-5 (via kilo)
- **Concurrency:** 3 per module, 3 total

---

## 🧹 COMMANDS

```bash
# Restart governor
sudo systemctl restart governor

# Check logs
journalctl -u governor -f

# Clean test data
sudo bash -c 'source <(systemctl show governor -p Environment | sed "s/Environment=//" | tr " " "\n") && curl -s -X DELETE "${SUPABASE_URL}/rest/v1/task_runs?id=not.is.null" -H "apikey: ${SUPABASE_SERVICE_KEY}" -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}" -H "Prefer: return=minimal"'
```

---

## 📜 SESSION HISTORY

- **74:** Module branch creation, learning system fixes, docs update
- **73:** Full audit, testing fix, failure notes, dashboard alignment
- **72:** Processing lock timing, status dedup, task context
- **71:** Pool failure lock, processing_by check, event dedup
- **70:** Fixed endless session spawning
- **69:** Applied duplicate task fix
