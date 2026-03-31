# VibePilot Current State - CRITICAL UPDATE
**Date:** 2026-03-31 17:25
**Session:** Investigating and fixing task branch collision issue

---

## 🎯 WHAT WE ACCOMPLISHED TODAY

### ✅ Proven Solutions Working:

1. **Config Format Fix (agents.json)**
   - **Problem:** "No internal routing available for role planner"
   - **Root Cause:** Added "role" field to agents.json (broke schema)
   - **Solution:** Restored original agents.json without "role" field
   - **Status:** ✅ **WORKING** - Plans creating successfully
   - **File:** `governor/config/agents.json`

2. **Permission Bypass Wrapper**
   - **Problem:** Claude CLI requires `--permission-mode bypassPermissions` for file writes
   - **Solution:** Created wrapper script that auto-adds permission bypass
   - **Status:** ✅ **WORKING** - Tested successfully, creates files
   - **File:** `governor/claude-wrapper`
   - **Test Result:** File created, no permission errors

3. **End-to-End Pipeline**
   - ✅ GitHub PRD → Supabase Plan → Dashboard visibility
   - ✅ Plan creation → Supervisor approval → Task creation
   - ✅ One successful task execution (test-simple-task-v2)
   - **Proof:** Task `05432bce` (T001 from plan `4dd6fa99`) successfully merged

---

## ⛠️ CURRENT CRITICAL ISSUES

### 🔴 Issue #1: Task Branch Naming Collision (BLOCKING)

**Severity:** CRITICAL - Preventing all task execution

**Problem:**
Multiple tasks from different plans have the same task number (T001), causing branch conflicts:

```go
// CURRENT CODE (3 locations):
func (h *TaskHandler) buildBranchName(taskNumber, taskID string) string {
    prefix := h.cfg.GetTaskBranchPrefix()
    if prefix == "" {
        prefix = "task/"
    }
    if taskNumber != "" {
        return prefix + taskNumber  // ← PROBLEM: Just "task/T001"
    }
    return prefix + truncateID(taskID)
}
```

**Evidence:**
- Plan `4dd6fa99` (test-simple-task-v2): T001 → `task/T001` ✅ MERGED
- Plan `d886ca13` (test-consecutive-execution): T001 → `task/T001` ⚠️ BLOCKED
- Plan `47a8757a` (test-config-fix-v5): T001 → `task/T001` ⚠️ BLOCKED
- Plan `f527daf0` (test-config-fix-v5 duplicate): T001 → `task/T001` ⚠️ BLOCKED

**Impact:**
- Only the first T001 succeeded
- 3 tasks stuck in "available" status
- Git branch conflicts preventing execution
- Dashboard shows 3 pending tasks (all T001)

---

### 🟡 Issue #2: Config Files Not Persisting (WORKAROUND IN PLACE)

**Problem:** Governor loses config files on restart
**Root Cause:** Files in `governor/config/` are untracked, lost when switching branches
**Current Workaround:**
- Manually restore files: `git checkout HEAD -- governor/`
- But governor binary still can't load them properly

**Files Affected:**
- `governor/config/connectors.json` - Updated to use wrapper
- `governor/config/agents.json` - Restored to working version
- `governor/claude-wrapper` - Permission bypass script

---

## 🔧 WHERE THE FIX IS NEEDED

### Files to Modify:

**3 locations with identical `buildBranchName` function:**

1. **governor/cmd/governor/handlers_task.go**
   - Line: 452-461
   - Function: `func (h *TaskHandler) buildBranchName(taskNumber, taskID string) string`

2. **governor/cmd/governor/handlers_maint.go**
   - Line: 254-262
   - Function: `func (h *MaintenanceHandler) buildBranchName(taskNumber, taskID string) string`

3. **governor/cmd/governor/handlers_testing.go**
   - Line: 194-202
   - Function: `func (h *TestingHandler) buildBranchName(taskNumber, taskID string) string`

---

## 💡 PROPOSED SOLUTION

### Option A: Include Plan ID in Branch Name (RECOMMENDED)

```go
func (h *TaskHandler) buildBranchName(taskNumber, taskID, planID string) string {
    prefix := h.cfg.GetTaskBranchPrefix()
    if prefix == "" {
        prefix = "task/"
    }

    // Create unique branch name with plan ID
    if taskNumber != "" && planID != "" {
        // Format: task/T001-abc12345 (first 8 chars of plan ID)
        return prefix + taskNumber + "-" + planID[:8]
    }

    if taskNumber != "" {
        return prefix + taskNumber
    }

    return prefix + truncateID(taskID)
}
```

**Result:**
- Plan `4dd6fa99`: T001 → `task/T001-4dd6fa99` ✅
- Plan `d886ca13`: T001 → `task/T001-d886ca13` ✅
- Plan `47a8757a`: T001 → `task/T001-47a8757a` ✅
- No more collisions!

### Option B: Globally Unique Task Numbers

Modify task creation to use globally incrementing numbers instead of per-plan numbering.

**Trade-off:** Loses the per-plan numbering which is actually useful (T001, T002 within each plan).

### Option C: Use Full Task ID

```go
return prefix + truncateID(taskID)  // Always use task ID, ignore task number
```

**Result:** `task/ca06b2c7`, `task/ea2d84f6` etc.
**Trade-off:** Less readable, harder to identify task sequence within plan.

---

## 📂 CURRENT STATE OF FILES

### Governor Source Code: ✅ RESTORED
- **Location:** `/home/vibes/vibepilot/governor/`
- **Status:** Was missing, restored with `git checkout HEAD -- governor/`
- **Contains:**
  - `cmd/governor/handlers_task.go` - Task execution logic
  - `cmd/governor/handlers_maint.go` - Maintenance logic
  - `cmd/governor/handlers_testing.go` - Testing logic
  - `internal/` - All internal packages
  - `Makefile` - Build instructions
  - `go.mod`, `go.sum` - Go dependencies

### Compiled Binary:
- **Location:** `/home/vibes/vibepilot/governor/governor`
- **Status:** Present, but outdated (needs recompile after fix)
- **Size:** 11MB (11,065,354 bytes)
- **Type:** ELF 64-bit LSB executable

### Config Files:
- **Status:** Partially working, need persistence solution
- **connectors.json:** Updated to use claude-wrapper ✅
- **agents.json:** Restored to working version ✅
- **Issue:** Governor not loading them correctly

### Permission Bypass Wrapper:
- **Location:** `/home/vibes/vibepilot/governor/claude-wrapper`
- **Status:** ✅ Working when tested directly
- **Test:** Successfully creates files with bypass

---

## 🚀 NEXT STEPS

### Immediate (To Fix Branch Collision):

1. **Modify `buildBranchName` in 3 files**
   - Add `planID` parameter
   - Include plan ID in branch name
   - Ensure uniqueness across all plans

2. **Update function calls**
   - Find all calls to `buildBranchName`
   - Pass plan ID as parameter
   - Update function signatures

3. **Recompile governor**
   ```bash
   cd ~/vibepilot/governor
   make build  # or: go build -o governor ./cmd/governor
   ```

4. **Deploy and test**
   - Stop old governor
   - Start new governor
   - Test with fresh PRD
   - Verify unique branch names

5. **Clean up stuck tasks**
   - Delete 3 stuck T001 tasks from Supabase
   - Or update their branch_names to be unique
   - Create fresh PRD to test

### Secondary (To Fix Config Persistence):

1. **Commit config files to git**
   - Add `governor/config/` files
   - Push to GitHub
   - Ensure they're always available

2. **Or create startup script**
   - Script generates configs on startup
   - Idempotent (safe to run multiple times)
   - Called by governor init

---

## 📊 PERFORMANCE METRICS

### Before Fixes:
- Plan creation: 25-30s ✅
- Task execution: ~8 minutes (multiple timeouts) ❌
- Total: ~11 minutes ❌

### After Fixes (Expected):
- Plan creation: 25-30s ✅
- Task execution: ~90-120s (one session) ✅
- **Total: ~2-3 minutes** (60% faster) ✅

### What's Blocking Performance:
- ⛔ Task branch collision (main blocker)
- ⛔ Config persistence (secondary issue)
- ✅ Permission bypass (solved)
- ✅ Routing/config format (solved)

---

## 🎯 SUCCESS CRITERIA

### Branch Collision Fix:
- [ ] Tasks from different plans get unique branch names
- [ ] Multiple T001 tasks can execute concurrently
- [ ] No git branch conflicts
- [ ] Dashboard shows tasks progressing

### End-to-End Pipeline:
- [ ] Fresh PRD creates plan successfully
- [ ] Plan approved by supervisor
- [ ] Task created with unique branch
- [ ] Task executes with permission bypass
- [ ] Files created successfully
- [ ] Task merges to module branch
- [ ] Total time: 2-3 minutes

---

## 🔗 KEY FILES TO MODIFY

### Must Modify:
1. `governor/cmd/governor/handlers_task.go` (line 452)
2. `governor/cmd/governor/handlers_maint.go` (line 254)
3. `governor/cmd/governor/handlers_testing.go` (line 194)

### Must Update Calls:
1. Anywhere `buildBranchName` is called
2. Pass plan ID as new parameter
3. Update function signatures

### Build & Deploy:
1. `governor/Makefile` - Build script
2. `governor/governor` - Compiled binary (will be replaced)

---

## 📝 NOTES

- **Current branch:** `main` ✅
- **Governor source:** Restored and ready to edit ✅
- **Backup working task:** `05432bce` (T001 from plan `4dd6fa99`) ✅
- **Permission bypass wrapper:** Working ✅
- **Config files:** Need persistence solution ⚠️

---

**Summary:** We have ONE critical fix needed (branch naming) to unlock full autonomous task execution. The permission bypass is proven to work. Config persistence needs a secondary fix but has workarounds. Once branch collision is fixed, we should see 2-3 minute end-to-end task execution (vs 11+ minutes before).
