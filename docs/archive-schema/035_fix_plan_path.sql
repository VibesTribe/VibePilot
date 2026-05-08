-- VibePilot Migration 035: Fix update_plan_status to set plan_path
-- Purpose: Update plan_path column when planner creates plan

CREATE OR REPLACE FUNCTION update_plan_status(
  p_plan_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL,
  p_plan_path TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET 
    status = p_status,
    plan_path = COALESCE(p_plan_path, plan_path),
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
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 035 complete - update_plan_status now sets plan_path' AS status;
