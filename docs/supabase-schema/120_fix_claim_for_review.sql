-- Migration 120: Fix claim_for_review to accept tasks already in review status
-- The task executor transitions tasks to review after completion.
-- The supervisor then needs to claim_for_review, but it was checking
-- for status IN ('approved', 'merge_pending') which misses 'review'.

DROP FUNCTION IF EXISTS claim_review(UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS claim_for_review(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET processing_by = p_reviewer_id, processing_at = NOW(), updated_at = NOW()
  WHERE id = p_task_id AND status = 'review' AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
