-- VibePilot Migration 074: Add update_task_branch RPC
-- Purpose: Update task branch_name field when task execution starts

CREATE OR REPLACE FUNCTION update_task_branch(
  p_task_id UUID,
  p_branch_name TEXT
)
RETURNS JSONB AS $$
DECLARE
  result JSONB;
BEGIN
  UPDATE tasks SET 
    branch_name = p_branch_name,
    updated_at = NOW()
  WHERE id = p_task_id
  RETURNING jsonb_build_object('id', id, 'branch_name', branch_name) INTO result;
  RETURN result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION update_task_branch(UUID, TEXT) TO service_role;

SELECT 'Migration 074 complete: update_task_branch RPC added' AS status;
