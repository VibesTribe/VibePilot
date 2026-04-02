-- Cleanup all tasks, plans, and related data
-- Run this in Supabase SQL Editor

-- Delete task runs first (foreign key dependency)
TRUNCATE task_runs CASCADE;

-- Delete tasks
TRUNCATE tasks CASCADE;

-- Delete plan revisions
TRUNCATE plan_revisions CASCADE;

-- Delete plans
TRUNCATE plans CASCADE;

-- Optional: Clear research suggestions
TRUNCATE research_suggestions CASCADE;

-- Optional: Clear test results
TRUNCATE test_results CASCADE;

-- Verify cleanup
SELECT 'tasks' as table, COUNT(*) as remaining_count FROM tasks
UNION ALL
SELECT 'plans', COUNT(*) FROM plans
UNION ALL
SELECT 'task_runs', COUNT(*) FROM task_runs
UNION ALL
SELECT 'plan_revisions', COUNT(*) FROM plan_revisions;
