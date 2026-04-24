-- Migration 129: Fix update_plan_status RPC type mismatch
-- p_review_notes was jsonb but review_notes column is text, causing COALESCE to fail
-- This prevented plan status updates from ever succeeding

CREATE OR REPLACE FUNCTION update_plan_status(
    p_plan_id UUID,
    p_status TEXT,
    p_plan_path TEXT DEFAULT NULL,
    p_review_notes TEXT DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
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

  IF p_status = 'approved' THEN
    UPDATE tasks SET status = 'available', updated_at = NOW()
    WHERE plan_id = p_plan_id AND status = 'pending';
  END IF;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$;
