-- VibePilot Migration 040: Add update_task_status RPC
-- Purpose: Simple RPC to update task status

CREATE OR REPLACE FUNCTION update_task_status(
  p_task_id UUID,
  p_status TEXT
)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET 
    status = p_status,
    updated_at = NOW()
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 040 complete - update_task_status RPC created' AS status;
