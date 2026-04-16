-- Migration 112: Fix update_plan_status to include p_plan_path parameter
-- The 111 migration dropped the 4-param version and recreated with only 3 params
-- Go code sends p_plan_id, p_status, p_plan_path -- needs 3-param version that accepts plan_path

DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT, JSONB) CASCADE;
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT, JSONB, TEXT) CASCADE;
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT) CASCADE;

CREATE OR REPLACE FUNCTION update_plan_status(
  p_plan_id UUID,
  p_status TEXT,
  p_plan_path TEXT DEFAULT NULL,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE plans
  SET status = p_status,
      plan_path = COALESCE(p_plan_path, plan_path),
      processing_by = NULL,
      processing_at = NULL,
      review_notes = COALESCE(p_review_notes, review_notes),
      updated_at = NOW(),
      approved_at = CASE WHEN p_status = 'approved' THEN NOW() ELSE approved_at END
  WHERE id = p_plan_id;

  -- If approved, flip all pending tasks to available
  IF p_status = 'approved' THEN
    UPDATE tasks
    SET status = 'available', updated_at = NOW()
    WHERE plan_id = p_plan_id AND status = 'pending';
  END IF;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION update_plan_status TO authenticated;

SELECT 'Migration 112 complete - update_plan_status with plan_path' AS status;
