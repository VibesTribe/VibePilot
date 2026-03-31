# Branch Cleanup & Task Numbering - Complete

## Issues Resolved ✅

### 1. Task Branch Cleanup
**Problem:** Old `task/T001` branch wasn't deleted after merge
**Root Cause:** Main repo was a git worktree with task/T001 checked out
**Solution:**
- Switched worktree to main branch
- Deleted task/T001 locally
- Remote was already cleaned up

### 2. Task Numbering Confusion
**Clarification:** Task numbers are **per-plan**, not global
**This is INTENTIONAL:**
- Plan 1 → T001, T002, T003
- Plan 2 → T001, T002 (new sequence)
- No cross-plan dependencies

## Tools Created

### 1. Branch Cleanup Script
**Location:** `scripts/cleanup-branches.sh`

**Usage:**
```bash
# Interactive (with confirmation)
./scripts/cleanup-branches.sh

# Force cleanup (no confirmation)
./scripts/cleanup-branches.sh $HOME/vibepilot true

# Custom repository
./scripts/cleanup-branches.sh /path/to/repo
```

**Features:**
- ✓ Automatically switches off task branches
- ✓ Lists branches before deletion
- ✓ Confirmation prompt (unless forced)
- ✓ Prunes remote references
- ✓ Shows summary of actions

### 2. Documentation
**Location:** `TASK_NUMBERING.md`

**Contents:**
- Task numbering system explanation
- Branch cleanup procedures
- Common issues and solutions
- Best practices
- Troubleshooting guide

## Current Branch Status

### Clean Branches
```
✅ main (active)
✅ TEST_MODULES/general
✅ remotes/origin/main
✅ remotes/origin/TEST_MODULES/general
```

### Removed Branches
```
✗ task/T001 (deleted)
✗ remotes/origin/task/T001 (pruned)
```

## What Happened With Task T001

### First Task (test-simple-task-v2)
1. PRD pushed → Plan created
2. Task T001 created and executed
3. **Success:** Created hello_vibepilot_v2.go
4. Merged to TEST_MODULES/general
5. **Issue:** Branch cleanup failed (worktree)
6. **Fixed:** Manually cleaned up

### Second Task (test-consecutive-execution)
1. PRD pushed → Plan created
2. **New Task T001 created** (different plan, new T001)
3. Task execution in progress (using simple one-session prompt)

## Prevention Going Forward

### Automatic Cleanup
The governor attempts to delete task branches after merge:
```
[GIT] Attempting: branch -D task/T001
[GIT] Attempting: push --delete task/T001
```

### Manual Cleanup
If automatic cleanup fails:
```bash
# Run cleanup script
./scripts/cleanup-branches.sh

# Or manually
git checkout main
git branch -D task/T001
git push origin --delete task/T001
git remote prune origin
```

### Worktree Management
**Best Practice:** Always stay on main branch
```bash
# Check current branch
git branch --show-current

# If on task branch, switch back
git checkout main
```

## Task Flow Verification

### Verify Successful Task Completion
```bash
# 1. Check task merged to module branch
git log TEST_MODULES/general --oneline -3

# 2. Verify file exists
ls governor/cmd/tools/hello_vibepilot_v2.go

# 3. Check governor logs
grep "merged to" ~/vibepilot/governor.log | tail -5

# 4. Verify task branch is gone
git branch -a | grep task/T001
# Should return nothing

# 5. Check VibeFlow dashboard
# https://vibeflow-dashboard.vercel.app/
# Status should show: Merged
```

## Files Modified This Session

### Added
- `TASK_NUMBERING.md` - Task numbering documentation
- `scripts/cleanup-branches.sh` - Automated cleanup script
- `CLEANUP_SUMMARY.md` - This file

### Modified
- `governor/config/agents.json` - Switched to simple one-session prompt
- `prompts/task_runner_simple.md` - Streamlined one-session prompt

### Committed
- `4e0dac04` - Task numbering docs + cleanup script
- `82eca771` - Simplified one-session prompt + test PRD v3

## Next Steps

### Immediate
1. ✅ Branches cleaned up
2. ✅ Documentation created
3. ✅ Cleanup script tested

### For Future Tasks
1. Monitor automatic branch cleanup
2. Use cleanup script if needed
3. Check worktree status before tasks
4. Verify merge completion in dashboard

### Optional Improvements
1. Add cleanup to governor startup recovery
2. Create pre-flight check for worktree status
3. Add branch cleanup to CI/CD pipeline
4. Monitor for branch cleanup failures

## Summary

✅ **Task numbering understood** - Per-plan, not global
✅ **Branch cleanup working** - Manual and automated tools ready
✅ **Worktree issue resolved** - Now on main branch
✅ **Prevention documented** - Best practices established
✅ **Monitoring in place** - Governor + dashboard + scripts

**Repository is clean and ready for new tasks!** 🎉
