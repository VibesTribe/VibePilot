-- VibePilot Migration 042: Add update_task_assignment RPC
-- Purpose: Update task status AND assigned_to model in one atomic call

-- This replaces the separate status and assigned_to updates

CREATE OR REPLACE FUNCTION update_task_assignment(
  p_task_id UUID,
  p_status TEXT,
  p_assigned_to TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET 
    status = p_status,
    assigned_to = COALESCE(p_assigned_to, assigned_to, p_status),
    started_at = COALESCE(p_assigned_to, NOW(), ELSE started_at IS NULL END,
    updated_at = NOW()
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 042 complete - update_task_assignment RPC created' AS status;
