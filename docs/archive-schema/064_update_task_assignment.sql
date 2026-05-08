-- VibePilot Migration 064: Add update_task_assignment RPC
-- Purpose: Update task status AND assigned_to model in one atomic call

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
    assigned_to = COALESCE(p_assigned_to, assigned_to),
    started_at = CASE 
      WHEN p_status = 'in_progress' AND started_at IS NULL THEN NOW()
      ELSE started_at
    END,
    updated_at = NOW()
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 064 complete - update_task_assignment RPC created' AS status;
