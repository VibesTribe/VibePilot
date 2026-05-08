-- Migration 133: Fix claim_task to accept 'available' status
-- Migration 130 added 'available' as a valid status for zero-dependency tasks,
-- but claim_task still only claimed tasks with status='pending'.
-- This caused web/courier routing to fail silently: router correctly selected
-- web routing but claim returned false (0 rows updated) because status was
-- 'available' not 'pending'. After 5 retries, fell through to internal execution.
-- Root cause: the router selected web routing correctly but the claim could never succeed.

DROP FUNCTION IF EXISTS claim_task(UUID, text, text, text, text);

CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_model_id TEXT,
  p_worker_id TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT NULL,
  p_routing_reason TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
  v_deps JSONB;
  v_unmet INT;
BEGIN
  -- Check dependency satisfaction
  SELECT dependencies INTO v_deps FROM tasks WHERE id = p_task_id;
  IF v_deps IS NOT NULL AND jsonb_array_length(v_deps) > 0 THEN
    SELECT COUNT(*) INTO v_unmet
    FROM jsonb_array_elements_text(v_deps) AS dep
    WHERE NOT EXISTS (
      SELECT 1 FROM tasks t WHERE t.task_number = dep AND t.status IN ('complete', 'merged')
    );
    IF v_unmet > 0 THEN RETURN FALSE; END IF;
  END IF;

  -- Claim: accept both 'pending' (has deps) and 'available' (no deps / deps met)
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_model_id,
      processing_by = p_worker_id,
      processing_at = NOW(),
      routing_flag = COALESCE(p_routing_flag, routing_flag),
      routing_flag_reason = p_routing_reason,
      updated_at = NOW()
  WHERE id = p_task_id
    AND status IN ('pending', 'available')
    AND processing_by IS NULL;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql VOLATILE SECURITY INVOKER;
