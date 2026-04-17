-- Migration 116: Fix claim_task parameter names to match Go code
-- Go sends: p_task_id, p_worker_id, p_model_id, p_routing_flag, p_routing_reason

DROP FUNCTION IF EXISTS claim_task(UUID, TEXT, TEXT, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_worker_id TEXT DEFAULT NULL,
  p_model_id TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT NULL,
  p_routing_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      processing_by = p_worker_id,
      processing_at = NOW(),
      model_id = COALESCE(p_model_id, model_id),
      updated_at = NOW()
  WHERE id = p_task_id AND status = 'available' AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Also update transition_task to handle in_progress → review properly
DROP FUNCTION IF EXISTS transition_task(UUID, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_result TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = p_new_status,
      result = COALESCE(p_result, result),
      processing_by = NULL,
      processing_at = NULL,
      updated_at = NOW()
  WHERE id = p_task_id;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;

SELECT 'Migration 116 complete - claim_task params fixed' AS status;
