# VibePilot Current State - 2026-04-01 18:45

## Status: ✅ ROUTING FIX APPLIED - Governor Running

## 🧹 CLEANUP PROCEDURES
**Before testing:** See `docs/CLEANUP.md` for full cleanup guide.
- Quick Supabase reset: `TRUNCATE task_runs, tasks, plan_revisions, plans CASCADE;`
- Clean GitHub branches: `git branch | grep "task/" | xargs git branch -D`
- Restart governor after cleanup

## Status: ✅ ROUTING FIX APPLIED

### 🔧 Internal Routing Fix Applied (18:45)

**Commit:** `89c2452e` - Use agent's configured model for internal routing

**Problem:** Router searched for models by taskType/taskCategory, returned empty for generic tasks
- **Symptom:** `[Router] No internal routing available for role internal_cli`
- **Root Cause:** `selectModelForConnector("claude-code", "", "")` → no model matched
- **Impact:** Governor couldn't route tasks to internal_cli agent

**Fix Applied:**
1. ✅ **Router logic** (`governor/internal/runtime/router.go:97-127`)
   - When `Role="internal_cli"`, get agent's configured model from agents.json
   - `internal_cli` → `model: glm-5` → connector `claude-code` (in glm-5's `access_via`)
   - Added `canConnectorAccessModel()` helper

2. ✅ **Governor rebuilt and running**
   - PID: 336590
   - Connected to Supabase
   - Listening on port 8080

**Expected Flow:**
1. Task available → router checks for courier (needs vision/browser)
2. glm-5 (via claude-code CLI) lacks courier support → internal routing
3. Internal routing: `internal_cli` → `glm-5` → `claude-code`
4. Governor creates second Claude CLI session for task execution

### Previous Fix: Task Execution Result (17:00)

**Commit:** `8c328b6f` - Store task execution results in task_runs.result

**Problem:** task_runs.result was NULL, supervisor had nothing to review → infinite fail loop
- **Root Cause:** task_runs.result column existed but create_task_run never wrote to it
- **Impact:** Supervisor received task_run with no execution data → failed with empty reason

**Fix Applied:**
1. ✅ **Supabase function** (`docs/supabase-schema/fix_task_run_result.sql`)
   - Updated `create_task_run` to accept and store `p_result`
   - **Needs to be run in Supabase SQL Editor**

2. ✅ **Go code** (`governor/cmd/governor/handlers_task.go:227-266`)
   - Captures execution result (files, summary, raw_output)
   - Passes to `create_task_run` as `p_result`
   - Binary rebuilt (16:55)

3. ⚠️ **Deployment pending:**
   - Run SQL in Supabase
   - Restart governor service

### Previous Fixes Still Active

**Fix 1: CLI Runner STDIN Bug** ✅
- Prompt written to STDIN: `echo "prompt" | claude -p`
- Works with ALL CLI tools

**Fix 2: Recovery Timeout** ✅
- Increased from 60s to 360s (6 minutes)
- Tasks can complete without premature termination

**Fix 3: T001 Numbering Bug** ✅
- Fixed RPC result parsing for slice-based task numbering
- Tasks now numbered: T001, T002, T003... per slice

### Current Status

**Governor:** Running (PID 336590)
**Routing:** Fixed - uses agent's configured model
**Waiting for:** Task to be created in Supabase to test execution flow

### Next Test

Create a task in Supabase to verify:
1. Router finds internal_cli → glm-5 → claude-code ✅
2. Governor creates second Claude CLI session ✅
3. Task executes in second session
4. Results stored in task_runs.result

**Before testing:** Run cleanup from `docs/CLEANUP.md`

---

**Last Updated:** 2026-04-01 18:45
**Status:** Routing fix applied, governor running
**Governor:** Running (PID 336590)
**Latest Commit:** 89c2452e
