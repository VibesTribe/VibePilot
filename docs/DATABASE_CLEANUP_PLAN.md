# VibePilot Database Cleanup Plan

**Audit Date:** 2026-02-20  
**Auditor:** GLM-5  
**Database:** Supabase (qtpdzsinvifkgpxyxlaz.supabase.co)

---

## Executive Summary

The VibePilot database contains **33 tasks** and **38 task runs** across **9 accessible tables**. Before enabling full autonomous testing, **2 high-priority cleanup actions** are required to prevent data quality issues and system confusion.

### Key Findings

| Metric | Count | Status |
|--------|-------|--------|
| Total Tasks | 33 | ⚠️ |
| Test Tasks | 11 | 🔴 NEEDS CLEANUP |
| Stuck Tasks (in_progress) | 4 | 🔴 NEEDS RESET |
| Failed Runs | 22 | ⚠️ |
| Accessible Tables | 9 | ✅ |
| Missing Tables | 3 | ⚠️ |

---

## Table Inventory

### Core Tables (Found)

| Table | Purpose | Record Count | Status |
|-------|---------|--------------|--------|
| **projects** | Project definitions | 2 | ✅ OK |
| **tasks** | Task definitions and status | 33 | ⚠️ NEEDS CLEANUP |
| **task_packets** | Task execution packets | 7 | ✅ OK |
| **task_runs** | Execution runs and results | 38 | ✅ OK |
| **secrets_vault** | API keys and secrets | 7 | ✅ OK |
| **task_backlog** | Backlog items | 0 | ✅ Empty |
| **lessons_learned** | Learning records | 0 | ✅ Empty |
| **agent_tasks** | Agent task assignments | 0 | ✅ Empty |
| **maintenance_commands** | Maintenance history | 0 | ✅ Empty |

### Missing/Unavailable Tables

| Table | Expected Purpose | Impact |
|-------|-----------------|--------|
| **rate_limit_windows** | Rate limiting tracking | Medium - using fallback |
| **model_performance** | Model performance metrics | Low - telemetry only |
| **session_states** | Session state management | Low - feature not active |

---

## Data Quality Issues Identified

### Issue 1: Test Data Pollution (HIGH PRIORITY)

**Problem:** 11 tasks contain test-related keywords ("test", "hello", "world", "example") that were created during pipeline testing.

**Impact:**
- Confuses autonomous task selection
- Skews metrics and analytics
- May trigger unnecessary task processing

**Affected Tasks:**
| Task ID (short) | Title | Status |
|-----------------|-------|--------|
| c3e1cec2 | Write a hello world function | merged |
| 22cd2ec2 | Test Task: Generate a greeting | testing |
| 97db598e | Test Task: Generate a greeting | merged |
| e784837f | Test Task: Generate a greeting | in_progress |
| 4292d9cb | Test Task: Generate a greeting | in_progress |
| 550e8400 | Write tests | in_progress |
| a895cf51 | Test Task: Generate a greeting | testing |
| 44eaf114 | Kimi Test: Fibonacci function | testing |
| b429f481 | ROI Test: Calculate 2+2 | testing |
| 153fe19e | Kimi Test: Fibonacci function | escalated |
| 20bd4715 | Test Task: Generate a greeting | testing |

**Duplicate Titles:**
- "Test Task: Generate a greeting" appears **6 times**
- "Kimi Test: Fibonacci function" appears **2 times**

### Issue 2: Tasks Stuck in Active States (HIGH PRIORITY)

**Problem:** 4 tasks are stuck in `in_progress` state with 0 attempts, blocking the task queue.

**Impact:**
- Prevents new task processing
- Wastes orchestrator cycles
- May indicate retry logic issues

**Stuck Tasks:**
| Task ID (short) | Title | Status | Attempts |
|-----------------|-------|--------|----------|
| 550e8400 | Build API endpoints | in_progress | 0 |
| e784837f | Test Task: Generate a greeting | in_progress | 0 |
| 4292d9cb | Test Task: Generate a greeting | in_progress | 0 |
| 550e8400 | Write tests | in_progress | 0 |

### Issue 3: Failed Task Runs (MEDIUM PRIORITY)

**Problem:** 22 out of 38 task runs (58%) have failed status.

**Impact:**
- Indicates potential runner/platform issues
- Skews success rate metrics
- May indicate configuration problems

**Status Distribution:**
- Failed: 22 (58%)
- Success: 15 (39%)
- Running: 1 (3%)

### Issue 4: Missing Rate Limit Tracking (LOW PRIORITY)

**Problem:** The `rate_limit_windows` table is not accessible.

**Impact:**
- System uses fallback rate limiting
- May hit API limits unexpectedly
- No historical rate limit data

---

## The "Infinite Retry Bug" Context

Based on log analysis from Session 15, the infinite retry bug was characterized by:

1. **Symptoms:**
   - Tasks stuck in `in_progress` with increasing attempt counts
   - Retry loop without proper backoff
   - Tasks never transitioning to `failed` or `completed`

2. **Root Causes Fixed in Migrations:**
   - Missing `attempts` ceiling (max retries not enforced)
   - Race condition in task claiming
   - No exponential backoff between retries
   - Tasks not being released back to `available` on runner failure

3. **Current State:**
   - The 4 stuck tasks with 0 attempts may indicate a **NEW variant** of the bug
   - Tasks are stuck but not incrementing attempts
   - This suggests the retry logic isn't even being triggered

4. **Safety Measures Added:**
   - Migration 013: Added `max_attempts` column with default 3
   - Migration 014: Fixed race condition in `claim_next_task` RPC
   - Migration 015: Added exponential backoff in retry logic

---

## Cleanup Recommendations

### Before First Autonomous Task

#### 1. Reset Stuck Tasks (MUST DO)

**Action:** Reset all `in_progress` tasks to `available` status.

**SQL:**
```sql
-- Preview what will be reset
SELECT id, title, status, started_at, attempts
FROM tasks
WHERE status = 'in_progress';

-- Reset stuck tasks
UPDATE tasks
SET status = 'available',
    started_at = NULL,
    assigned_to = NULL,
    updated_at = NOW()
WHERE status = 'in_progress';
```

**Verification:**
```sql
SELECT COUNT(*) as stuck_tasks
FROM tasks
WHERE status = 'in_progress';
-- Should return 0
```

#### 2. Archive Test Tasks (MUST DO)

**Action:** Move test tasks to archived state or separate archive table.

**Option A - Mark as archived (recommended):**
```sql
-- Add archived status if not exists
-- Then archive test tasks
UPDATE tasks
SET status = 'archived',
    archived_at = NOW(),
    archive_reason = 'Test task - cleanup before autonomous testing'
WHERE LOWER(title) LIKE '%test%'
   OR LOWER(title) LIKE '%hello world%'
   OR LOWER(title) LIKE '%example%';
```

**Option B - Delete entirely (not recommended):**
```sql
-- Only if we're sure we don't need the history
DELETE FROM task_runs
WHERE task_id IN (
    SELECT id FROM tasks
    WHERE LOWER(title) LIKE '%test%'
       OR LOWER(title) LIKE '%hello world%'
       OR LOWER(title) LIKE '%example%'
);

DELETE FROM tasks
WHERE LOWER(title) LIKE '%test%'
   OR LOWER(title) LIKE '%hello world%'
   OR LOWER(title) LIKE '%example%';
```

**Recommendation:** Use Option A (archive) to preserve history.

#### 3. Verify Token Data (ALREADY CLEAN)

The `cleanup_task_runs.py` script has already been run. Verification:
- 0 runs with mismatched token counts
- 0 suspicious runs with high tokens but no in/out data

**Status:** ✅ CLEAN

#### 4. Check Rate Limit Table (OPTIONAL)

If autonomous testing will use rate-limited APIs:

```sql
-- Check if rate_limit_windows table exists
SELECT EXISTS (
    SELECT FROM information_schema.tables 
    WHERE table_name = 'rate_limit_windows'
);

-- If not, create it (refer to migration files)
```

---

## Safety Considerations

### What NOT to Touch

1. **secrets_vault table** - Contains encrypted API keys
   - ✅ Safe to read
   - ⚠️ Never modify or delete keys unless rotating

2. **projects table** - Core project definitions
   - ✅ Safe to read
   - ⚠️ Don't delete projects with active tasks

3. **Successfully completed tasks** - Production history
   - ✅ Should be preserved
   - ✅ Can be archived after 30 days

4. **Task runs with real results** - Execution history
   - ✅ Keep for analytics
   - ✅ Part of audit trail

### Backup Before Cleanup

```bash
# Create a backup before any changes
# (This is a recommendation - implement based on your backup strategy)
```

---

## Post-Cleanup Verification

After cleanup, verify:

1. **No stuck tasks:**
   ```sql
   SELECT status, COUNT(*) 
   FROM tasks 
   WHERE status IN ('in_progress', 'claimed', 'running')
   GROUP BY status;
   -- All counts should be 0
   ```

2. **Test tasks archived:**
   ```sql
   SELECT COUNT(*) as remaining_test_tasks
   FROM tasks
   WHERE (LOWER(title) LIKE '%test%' OR LOWER(title) LIKE '%hello%')
     AND status != 'archived';
   -- Should return 0
   ```

3. **Ready for autonomous testing:**
   ```sql
   SELECT COUNT(*) as available_tasks
   FROM tasks
   WHERE status = 'available';
   -- Should show only production tasks
   ```

---

## Migration History Context

### Migrations 013, 014, 015 Summary

Based on codebase analysis, these migrations addressed:

- **Migration 013:** Added `max_attempts` column to tasks table (default: 3)
- **Migration 014:** Fixed `claim_next_task` RPC to prevent race conditions
- **Migration 015:** Added `retry_delay` column for exponential backoff

These changes prevent the "infinite retry bug" by:
1. Limiting retries to a maximum of 3 attempts
2. Adding 2^attempt seconds delay between retries
3. Properly releasing tasks back to available pool on failure

---

## Next Steps

1. **Execute cleanup** (with human confirmation)
2. **Verify cleanup** using verification queries above
3. **Start autonomous testing** with monitoring
4. **Monitor for stuck tasks** in first 24 hours

---

## Appendix: Useful SQL Queries

### Daily Monitoring Queries

```sql
-- Task status summary
SELECT status, COUNT(*) as count
FROM tasks
GROUP BY status
ORDER BY count DESC;

-- Recent task completions
SELECT DATE(completed_at) as date, COUNT(*) as completed
FROM tasks
WHERE completed_at > NOW() - INTERVAL '7 days'
GROUP BY DATE(completed_at)
ORDER BY date DESC;

-- Tasks by model
SELECT model_id, COUNT(*) as runs, 
       SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successes
FROM task_runs
GROUP BY model_id
ORDER BY runs DESC;

-- Stuck tasks alert
SELECT id, title, status, started_at, attempts
FROM tasks
WHERE status IN ('in_progress', 'claimed', 'running')
  AND started_at < NOW() - INTERVAL '1 hour';
```

---

**Document Version:** 1.0  
**Last Updated:** 2026-02-20  
**Review Required:** Before first autonomous task execution
