-- Supabase Performance Fixes
-- Addresses: unindexed foreign keys + unused indexes
-- Version: 1.0
-- Created: 2026-02-17

-- ============================================
-- PART 1: Add missing indexes for foreign keys
-- ============================================

-- lessons_learned.task_id
CREATE INDEX IF NOT EXISTS idx_lessons_learned_task_id ON lessons_learned(task_id);

-- task_packets.task_id
CREATE INDEX IF NOT EXISTS idx_task_packets_task_id ON task_packets(task_id);

-- task_runs.model_id
CREATE INDEX IF NOT EXISTS idx_task_runs_model_id ON task_runs(model_id);

-- tasks.project_id
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);

-- ============================================
-- PART 2: Remove unused indexes
-- (Safe to remove - they've never been used)
-- ============================================

DROP INDEX IF EXISTS idx_tasks_slice;
DROP INDEX IF EXISTS idx_tasks_routing;
DROP INDEX IF EXISTS idx_tasks_number;
DROP INDEX IF EXISTS idx_models_access_type;
DROP INDEX IF EXISTS idx_tasks_deps;
DROP INDEX IF EXISTS idx_runs_status;
DROP INDEX IF EXISTS idx_council_reviews_plan_round;
DROP INDEX IF EXISTS idx_platforms_status;
DROP INDEX IF EXISTS idx_platforms_success;
DROP INDEX IF EXISTS idx_task_backlog_status;
DROP INDEX IF EXISTS idx_task_backlog_created;
DROP INDEX IF EXISTS idx_agent_tasks_status;
DROP INDEX IF EXISTS idx_agent_tasks_agent;
DROP INDEX IF EXISTS idx_tasks_priority;

-- ============================================
-- PART 3: Add useful indexes (for actual usage patterns)
-- ============================================

-- These will actually be used based on our query patterns
CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority) WHERE status IN ('pending', 'ready', 'in_progress');
CREATE INDEX IF NOT EXISTS idx_task_runs_task_created ON task_runs(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_runs_platform_status ON task_runs(platform, status) WHERE platform IS NOT NULL;

-- ============================================
-- VERIFICATION
-- ============================================

-- Check indexes on foreign keys now exist
SELECT 
    tablename,
    indexname
FROM pg_indexes
WHERE schemaname = 'public'
AND indexname IN (
    'idx_lessons_learned_task_id',
    'idx_task_packets_task_id',
    'idx_task_runs_model_id',
    'idx_tasks_project_id'
);

-- Should return 4 rows if successful
