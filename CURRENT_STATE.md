# VibePilot Current State
**Last Updated:** 2026-03-09 Session 73 (19:00 UTC)
**Status:** AUDIT COMPLETE - Ready for fixes

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## ✅ SESSION 73 COMPLETED

### 1. Testing Flow Fixed
- `status == "testing"` now triggers `EventTaskTesting`
- Testing handler actually runs now

### 2. Failure Notes Added
- All failure paths now record detailed reasons
- Dashboard can show why tasks failed

### 3. Dashboard Alignment (Migration 079)
- RPC writes `prompt_packet` to `tasks.result`
- `slice_id` parsed and passed to RPC
- **Applied in Supabase** ✅

### 4. Full Audit Completed
- 49 tables analyzed
- 121 RPCs documented
- 95 migrations reviewed
- Learning gaps identified

---

## 🔴 CRITICAL ISSUES TO FIX

### Priority 1: Module Branch Creation
**Problem:** `CreateModuleBranch()` exists but is NEVER CALLED
**Impact:** Tasks cannot merge properly
**Fix:** Call in `handlers_plan.go` after tasks created

### Priority 2: Supervisor Rule Creation
**Problem:** Rules not created from rejections
**Impact:** No learning from mistakes
**Fix:** Call `create_supervisor_rule` on rejection

### Priority 3: Tester Rule Creation
**Problem:** Rules not created from test failures
**Impact:** Same bugs repeat
**Fix:** Call `create_tester_rule` on test failure

### Priority 4: Heuristic Recording
**Problem:** Router doesn't learn model preferences
**Impact:** Suboptimal routing
**Fix:** Call `upsert_heuristic` on success

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

### Learning System Status
| Component | Table Rows | Status |
|-----------|------------|--------|
| Supervisor rules | 42 | ✅ Partially working |
| Failure records | 332 | ✅ Working |
| Tester rules | 0 | ❌ Not created |
| Heuristics | 0 | ❌ Not created |
| Problem solutions | 0 | ❌ Not created |
| Lessons learned | 0 | ❌ Not populated |

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

- **73:** Full audit, testing fix, failure notes, dashboard alignment
- **72:** Processing lock timing, status dedup, task context
- **71:** Pool failure lock, processing_by check, event dedup
- **70:** Fixed endless session spawning
- **69:** Applied duplicate task fix
