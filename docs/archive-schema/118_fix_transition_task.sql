DROP FUNCTION IF EXISTS transition_task(UUID, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_result JSONB DEFAULT NULL
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
