-- VIBESPILOT SCHEMA MIGRATION v1.1
-- Purpose: Add slice-based planning and routing flags
-- Date: 2026-02-16
-- 
-- Run this AFTER schema_v1_core.sql
-- 
-- Changes:
--   - Add slice_id to tasks (modular vertical grouping)
--   - Add routing_flag to tasks (internal/web/mcp constraint)
--   - Add routing_flag_reason to tasks (why this flag)
--   - Add phase to tasks (P1, P2, P3 within slice)
--   - Add task_number to tasks (human-readable ID like AUTH-P1-T001)

-- Add new columns to tasks table
ALTER TABLE tasks 
  ADD COLUMN IF NOT EXISTS slice_id TEXT,
  ADD COLUMN IF NOT EXISTS phase TEXT,
  ADD COLUMN IF NOT EXISTS task_number TEXT,
  ADD COLUMN IF NOT EXISTS routing_flag TEXT DEFAULT 'web' 
    CHECK (routing_flag IN ('internal', 'web', 'mcp')),
  ADD COLUMN IF NOT EXISTS routing_flag_reason TEXT;

-- Create index for slice-based queries (dashboard grouping)
CREATE INDEX IF NOT EXISTS idx_tasks_slice ON tasks(slice_id);

-- Create index for routing decisions
CREATE INDEX IF NOT EXISTS idx_tasks_routing ON tasks(routing_flag, status);

-- Create index for task_number lookups
CREATE INDEX IF NOT EXISTS idx_tasks_number ON tasks(task_number);

-- Update claim_next_task to respect routing flags
-- New function signature includes routing constraint
CREATE OR REPLACE FUNCTION claim_next_task(
  p_courier TEXT,
  p_platform TEXT,
  p_model_id TEXT,
  p_routing_constraint TEXT DEFAULT NULL  -- NULL = any, 'internal' = exclude web
) RETURNS UUID AS $$
DECLARE
  v_task_id UUID;
BEGIN
  IF p_routing_constraint = 'internal' THEN
    -- Only claim tasks that require internal routing (Q or M flagged)
    UPDATE tasks
    SET status = 'in_progress',
        assigned_to = p_model_id,
        attempts = attempts + 1,
        started_at = COALESCE(started_at, NOW()),
        updated_at = NOW()
    WHERE id = (
      SELECT id FROM tasks
      WHERE status = 'available'
        AND routing_flag IN ('internal', 'mcp')
        AND (
          dependencies = '{}'
          OR dependencies <@ (
            SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
          )
        )
      ORDER BY priority ASC, created_at ASC
      LIMIT 1
      FOR UPDATE SKIP LOCKED
    )
    RETURNING id INTO v_task_id;
  ELSE
    -- Claim any available task (default behavior)
    UPDATE tasks
    SET status = 'in_progress',
        assigned_to = p_model_id,
        attempts = attempts + 1,
        started_at = COALESCE(started_at, NOW()),
        updated_at = NOW()
    WHERE id = (
      SELECT id FROM tasks
      WHERE status = 'available'
        AND (
          dependencies = '{}'
          OR dependencies <@ (
            SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
          )
        )
      ORDER BY priority ASC, created_at ASC
      LIMIT 1
      FOR UPDATE SKIP LOCKED
    )
    RETURNING id INTO v_task_id;
  END IF;
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql;

-- Function: Get tasks by slice (for dashboard)
CREATE OR REPLACE FUNCTION get_tasks_by_slice(p_slice_id TEXT)
RETURNS TABLE (
  id UUID,
  task_number TEXT,
  title TEXT,
  phase TEXT,
  status TEXT,
  routing_flag TEXT,
  assigned_to TEXT,
  priority INT,
  created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.id, t.task_number, t.title, t.phase, t.status, 
    t.routing_flag, t.assigned_to, t.priority, t.created_at
  FROM tasks t
  WHERE t.slice_id = p_slice_id
  ORDER BY t.phase, t.priority, t.created_at;
END;
$$ LANGUAGE plpgsql;

-- Function: Get slice summary (for dashboard)
CREATE OR REPLACE FUNCTION get_slice_summary()
RETURNS TABLE (
  slice_id TEXT,
  total_tasks BIGINT,
  completed_tasks BIGINT,
  in_progress_tasks BIGINT,
  pending_tasks BIGINT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.slice_id,
    COUNT(*) AS total_tasks,
    COUNT(*) FILTER (WHERE t.status = 'merged') AS completed_tasks,
    COUNT(*) FILTER (WHERE t.status = 'in_progress') AS in_progress_tasks,
    COUNT(*) FILTER (WHERE t.status IN ('pending', 'available')) AS pending_tasks
  FROM tasks t
  WHERE t.slice_id IS NOT NULL
  GROUP BY t.slice_id
  ORDER BY t.slice_id;
END;
$$ LANGUAGE plpgsql;

-- Function: Get available tasks filtered by routing capability
-- For orchestrator to find tasks it can actually execute
CREATE OR REPLACE FUNCTION get_available_for_routing(
  p_can_web BOOLEAN DEFAULT TRUE,
  p_can_internal BOOLEAN DEFAULT TRUE,
  p_can_mcp BOOLEAN DEFAULT FALSE
)
RETURNS TABLE (
  id UUID,
  task_number TEXT,
  title TEXT,
  slice_id TEXT,
  routing_flag TEXT,
  priority INT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.id, t.task_number, t.title, t.slice_id, t.routing_flag, t.priority
  FROM tasks t
  WHERE t.status = 'available'
    AND (
      dependencies = '{}'
      OR dependencies <@ (
        SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
      )
    )
    AND (
      (p_can_web AND t.routing_flag = 'web')
      OR (p_can_internal AND t.routing_flag = 'internal')
      OR (p_can_mcp AND t.routing_flag = 'mcp')
    )
  ORDER BY t.priority ASC, t.created_at ASC;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your Supabase setup)
-- GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO authenticated;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO authenticated;

-- Verify migration
SELECT 'Migration v1.1 complete. Tasks table columns:' AS status;
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'tasks' 
  AND column_name IN ('slice_id', 'phase', 'task_number', 'routing_flag', 'routing_flag_reason');
