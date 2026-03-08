-- VibePilot Migration 076: Add append_failure_notes RPC
-- Purpose: Append failure notes to tasks.failure_notes for learning and routing improvements

CREATE OR REPLACE FUNCTION append_failure_notes(
  p_task_id UUID,
  p_notes TEXT
)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET 
    failure_notes = COALESCE(failure_notes || E'\n', '') || p_notes,
    updated_at = NOW()
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION append_failure_notes TO service_role;

SELECT 'Migration 076 complete - append_failure_notes RPC created' AS status;
