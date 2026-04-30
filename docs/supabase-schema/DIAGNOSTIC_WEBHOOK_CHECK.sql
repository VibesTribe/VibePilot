-- VibePilot Webhook Flow Diagnostic
-- Purpose: Verify the complete webhook flow is working end-to-end
-- Run in Supabase SQL Editor

-- ============================================================================
-- CHECK 1: Were plans created by GitHub webhooks?
-- ============================================================================
SELECT 
    id,
    prd_path,
    status,
    created_at,
    EXTRACT(EPOCH FROM (NOW() - created_at))/60 as minutes_ago
FROM plans 
WHERE prd_path LIKE '%webhook_test%'
ORDER BY created_at DESC;

-- Expected: Should see docs/prd/webhook_test_2.md and docs/prd/test_webhook.md
-- If empty: GitHub webhook handler failed to create plans
-- If found: GitHub webhook working ✅

-- ============================================================================
-- CHECK 2: Are there any plans in draft status (waiting for planner)?
-- ============================================================================
SELECT 
    id,
    prd_path,
    status,
    created_at,
    EXTRACT(EPOCH FROM (NOW() - created_at))/60 as minutes_ago
FROM plans 
WHERE status = 'draft'
ORDER BY created_at DESC
LIMIT 10;

-- Expected: If webhook flow works, these should be picked up by planner
-- If many old drafts: Planner not being invoked (Supabase webhook issue)
-- If empty or recent: Planner might be working ✅

-- ============================================================================
-- CHECK 3: Were tasks created from plans?
-- ============================================================================
SELECT 
    t.id as task_id,
    t.title,
    t.status as task_status,
    t.plan_id,
    p.prd_path,
    p.status as plan_status
FROM tasks t
RIGHT JOIN plans p ON t.plan_id = p.id
WHERE p.prd_path LIKE '%webhook_test%'
ORDER BY t.created_at DESC;

-- Expected: Should see tasks linked to the test plans
-- If no tasks: Planner not invoked or failed
-- If tasks exist: Full flow working ✅

-- ============================================================================
-- CHECK 4: Verify all required tables exist
-- ============================================================================
SELECT 
    table_name,
    CASE WHEN table_name IS NOT NULL THEN '✅ EXISTS' ELSE '❌ MISSING' END as status
FROM information_schema.tables 
WHERE table_schema = 'public' 
  AND table_name IN ('plans', 'tasks', 'research_suggestions', 'maintenance_commands', 'test_results')
ORDER BY table_name;

-- Expected: All 5 tables should show ✅ EXISTS
-- If any missing: Run the corresponding migration

-- ============================================================================
-- CHECK 5: Count records in each table
-- ============================================================================
SELECT 'plans' as table_name, COUNT(*) as count FROM plans
UNION ALL
SELECT 'tasks', COUNT(*) FROM tasks
UNION ALL
SELECT 'research_suggestions', COUNT(*) FROM research_suggestions
UNION ALL
SELECT 'maintenance_commands', COUNT(*) FROM maintenance_commands
UNION ALL
SELECT 'test_results', COUNT(*) FROM test_results
ORDER BY table_name;

-- Expected: Should see counts for all tables
-- plans: At least 2 (webhook_test files)
-- tasks: 0+ (depends on if planner ran)

-- ============================================================================
-- CHECK 6: Check for recent activity (last hour)
-- ============================================================================
SELECT 
    'plans' as table_name,
    COUNT(*) as recent_records,
    MAX(created_at) as last_created
FROM plans 
WHERE created_at > NOW() - INTERVAL '1 hour'
UNION ALL
SELECT 
    'tasks',
    COUNT(*),
    MAX(created_at)
FROM tasks 
WHERE created_at > NOW() - INTERVAL '1 hour';

-- Expected: Should see recent plan activity if webhooks working
-- If plans=0: GitHub webhook not creating plans
-- If plans>0 but tasks=0: Supabase webhook not triggering planner

-- ============================================================================
-- INTERPRETATION GUIDE
-- ============================================================================
-- 
-- Scenario 1: Plans exist, tasks exist
-- ✅ Full webhook flow working
-- → No action needed, just monitor
--
-- Scenario 2: Plans exist, no tasks
-- ⚠️ GitHub webhook working, Supabase webhook NOT firing
-- → Check Supabase webhook configuration
-- → Verify webhook URL: http://34.45.124.117:8080/webhooks
-- → Check Supabase webhook logs
--
-- Scenario 3: No plans, no tasks
-- ❌ GitHub webhook NOT creating plans
-- → Check governor logs: sudo journalctl -u vibepilot-governor -n 50
-- → Look for "Created plan for PRD" messages
-- → Verify RPC call to create_plan succeeded
--
-- Scenario 4: Plans stuck in 'draft' status
-- ⚠️ Plans created but planner not invoked
-- → Supabase webhook may not be firing on INSERT
-- → Check webhook is configured for INSERT events
-- → Check webhook delivery logs in Supabase
