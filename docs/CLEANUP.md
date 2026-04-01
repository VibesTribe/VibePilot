# Cleanup Guide: Plans, Tasks, GitHub, and Supabase

## Purpose
Clean up all state for a fresh test run. Use when debugging task execution, routing, or session management issues.

## 1. Supabase Cleanup

### Clear Tasks
```sql
-- View all tasks
SELECT id, title, status FROM tasks ORDER BY created_at DESC;

-- Delete specific task (replace ID)
DELETE FROM task_runs WHERE task_id = 'your-task-id';
DELETE FROM tasks WHERE id = 'your-task-id';

-- Or delete all tasks (clean slate)
TRUNCATE task_runs CASCADE;
TRUNCATE tasks CASCADE;
```

### Clear Plans
```sql
-- View all plans
SELECT id, title, status FROM plans ORDER BY created_at DESC;

-- Delete specific plan
DELETE FROM plan_revisions WHERE plan_id = 'your-plan-id';
DELETE FROM plans WHERE id = 'your-plan-id';

-- Or delete all plans
TRUNCATE plan_revisions CASCADE;
TRUNCATE plans CASCADE;
```

### Clear Research Suggestions
```sql
TRUNCATE research_suggestions CASCADE;
```

### Clear Test Results
```sql
TRUNCATE test_results CASCADE;
```

### Reset Governor Events (Optional)
```sql
TRUNCATE orchestrator_events CASCADE;
```

## 2. GitHub Cleanup

### List Feature Branches
```bash
cd /home/vibes/vibepilot
git branch | grep -E "task/|module/"
```

### Delete Branches (Local)
```bash
# Delete specific branch
git branch -D task/your-task-name

# Delete all task branches
git branch | grep "task/" | xargs git branch -D

# Delete all module branches
git branch | grep "module/" | xargs git branch -D
```

### Delete Branches (Remote)
```bash
# Delete specific remote branch
git push origin --delete task/your-task-name

# Delete all task branches from remote
git branch -r | grep "origin/task/" | sed 's/origin\///' | xargs -I % git push origin --delete %
```

### Clean Up Merged Branches
```bash
# Prune remote branches that are already deleted
git remote prune origin

# Delete local branches that were merged
git branch --merged | grep -E "task/|module/" | xargs git branch -d
```

## 3. Local Governor Cleanup

### Stop Governor
```bash
pkill -f "governor.*governor"
# Or from the terminal where it's running: Ctrl+C
```

### Clear Governor Logs
```bash
rm -f /home/vibes/vibepilot/governor.log
```

### Clear Orphaned Sessions
```bash
# The governor's startup recovery handles this, but to manually clean:
# Check Supabase for sessions with status='orphaned'
DELETE FROM agent_sessions WHERE status = 'orphaned';
```

## 4. Full Reset Script

```bash
#!/bin/bash
# full-reset.sh - Clean everything for fresh test run

echo "=== Stopping Governor ==="
pkill -f "governor.*governor" 2>/dev/null
sleep 2

echo "=== Cleaning Supabase ==="
# Run via psql or Supabase SQL editor
cat << 'SQL' | supabase db reset --db-url "$DATABASE_URL" || echo "Run manually in Supabase SQL Editor:"
TRUNCATE task_runs CASCADE;
TRUNCATE tasks CASCADE;
TRUNCATE plan_revisions CASCADE;
TRUNCATE plans CASCADE;
TRUNCATE research_suggestions CASCADE;
TRUNCATE test_results CASCADE;
TRUNCATE orchestrator_events CASCADE;
DELETE FROM agent_sessions WHERE status = 'orphaned';
SQL

echo "=== Cleaning GitHub Branches ==="
cd /home/vibes/vibepilot
git branch | grep "task/" | xargs git branch -D 2>/dev/null
git branch | grep "module/" | xargs git branch -D 2>/dev/null
git remote prune origin

echo "=== Clearing Logs ==="
rm -f governor.log

echo "=== Restarting Governor ==="
# You'll need to start this manually in your terminal
echo "Run: cd /home/vibes/vibepilot/governor && ./bin/governor"
```

## 5. Partial Cleanup (Quick Testing)

### Just Tasks, Keep Plans
```sql
TRUNCATE task_runs CASCADE;
TRUNCATE tasks CASCADE;
```

### Just Failed Tasks
```sql
DELETE FROM task_runs WHERE status = 'failed';
DELETE FROM tasks WHERE status = 'failed';
```

### Reset Specific Task
```sql
-- Reset task to 'available' so it will be picked up again
UPDATE tasks SET status = 'available' WHERE id = 'your-task-id';
DELETE FROM task_runs WHERE task_id = 'your-task-id';
```

## 6. Verification

### Check Supabase is Clean
```sql
-- Should return 0 or empty
SELECT COUNT(*) FROM tasks;
SELECT COUNT(*) FROM plans;
SELECT COUNT(*) FROM task_runs;
```

### Check GitHub is Clean
```bash
cd /home/vibes/vibepilot
git branch | grep -E "task/|module/"
# Should return nothing
```

### Check Governor is Ready
```bash
ps aux | grep "[g]overnor"
# Should show governor process running
curl http://localhost:8080/webhooks
# Should return 404 or webhook response (server is up)
```

## Notes

- Governor's startup recovery automatically handles orphaned sessions
- GitHub PRs are NOT auto-deleted - close them manually from GitHub web UI
- Dashboard shows the current state - refresh after cleanup
- Wait 5-10 seconds after starting governor before creating tasks (realtime subscription needs to connect)
