# Task Numbering and Branch Cleanup

## Task Numbering System

**Current Behavior:**
- Task numbers are **per-plan**, not global
- Each plan creates tasks starting at T001
- Example:
  - Plan A (test-simple-task-v2): Created T001
  - Plan B (test-consecutive-execution): Also creates T001

**This is INTENTIONAL** for modularity:
- Each plan is self-contained
- Tasks don't have cross-plan dependencies
- Easier to understand task flow within a plan

**Task ID Format:**
```
{plan_id}/T001
```

## Branch Cleanup

### Why Branch Deletion Failed

The main repository (`~/vibepilot`) was a **git worktree** with `task/T001` checked out. This prevented deletion.

**Error:**
```
error: cannot delete branch 'task/T001' used by worktree at '/home/vibes/vibepilot'
```

### Cleanup Process

1. **Switch worktree to main:**
   ```bash
   cd ~/vibepilot
   git checkout main
   ```

2. **Delete local task branch:**
   ```bash
   git branch -D task/T001
   ```

3. **Delete remote task branch:**
   ```bash
   git push origin --delete task/T001
   ```

4. **Prune stale references:**
   ```bash
   git remote prune origin
   ```

### Automated Cleanup Script

```bash
#!/bin/bash
# cleanup-task-branches.sh

echo "=== VibePilot Branch Cleanup ==="

# Switch to main if on task branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ $CURRENT_BRANCH =~ ^task/ ]]; then
  echo "Switching from $CURRENT_BRANCH to main"
  git checkout main
fi

# List all task branches
echo "Local task branches:"
git branch | grep "^  task/" || echo "  None"

# Delete local task branches
for branch in $(git branch | grep "^  task/" | sed 's/^[ \t]*//'); do
  echo "Deleting local branch: $branch"
  git branch -D "$branch" 2>/dev/null || echo "  Already deleted or protected"
done

# Prune remote references
echo "Pruning remote references..."
git remote prune origin

# Show remaining branches
echo ""
echo "Remaining branches:"
git branch -a

echo "✅ Cleanup complete"
```

## Best Practices

### During Development
1. **Stay on main** - Don't work on task branches directly
2. **Let governor manage branches** - Automatic cleanup should work
3. **Manual cleanup only when needed** - If automatic cleanup fails

### After Task Completion
1. **Verify merge** - Check TEST_MODULES/[module] has the changes
2. **Check governor logs** - Look for "branch -D" attempts
3. **Run cleanup script** - If branches weren't auto-deleted

### Task Flow Verification

**Expected flow:**
```
PRD → Plan → Task (T001) → Execute → Review → Test → Merge to TEST_MODULES/general
                                              ↓
                                    Delete task/T001 branch
```

**Check with:**
```bash
# Verify task is in module branch
git log TEST_MODULES/general --oneline -3

# Verify task branch is gone
git branch -a | grep task/T001

# Check governor logs
grep "merged to" ~/vibepilot/governor.log | tail -5
```

## Common Issues

### Issue: Worktree on Task Branch
**Symptom:** Cannot delete task branch
**Fix:** `git checkout main` then delete

### Issue: Remote Branch Exists
**Symptom:** `git push --delete` says "remote ref does not exist"
**Status:** Already deleted, safe to ignore
**Fix:** Run `git remote prune origin`

### Issue: Branch Protected
**Symptom:** Deletion blocked by protection rules
**Fix:** Check GitHub repository settings, remove protection if needed

## Current Status (2026-03-31)

✅ **Clean:**
- task/T001 deleted locally
- Remote already cleaned up
- Worktree switched to main
- Only branch remaining: TEST_MODULES/general

✅ **Working:**
- Task numbering (per-plan)
- Merge to module branches
- Governor attempts cleanup

⚠️ **Needs Monitoring:**
- Automatic branch deletion (worktree issue)
- Cleanup retry logic in governor
