-- Migration 083: Atomic task state transitions
-- Purpose: Prevent race conditions by updating status and processing in one transaction
-- Date: 2026-03-11

-- ============================================================================
-- RPC: complete_task_transition
-- Atomically updates status AND clears processing_by
-- Replaces: clear_processing + update_task_status (two separate calls)
-- ============================================================================

CREATE OR REPLACE FUNCTION complete_task_transition(
  p_task_id UUID,
  p_new_status TEXT
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  -- Validate status
  IF p_new_status NOT IN ('review', 'testing', 'approval', 'merged', 'available', 'escalated') THEN
    RAISE EXCEPTION 'Invalid status for task transition: %', p_new_status;
    RETURN FALSE;
  END IF;

  -- Atomic update: status + clear processing + set completed_at if merged
  UPDATE tasks
  SET
    status = p_new_status,
    processing_by = NULL,
    processing_at = NULL,
    completed_at = CASE WHEN p_new_status = 'merged' THEN NOW() ELSE completed_at END,
    updated_at = NOW()
  WHERE id = p_task_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION complete_task_transition(UUID, TEXT) IS
'Atomically transition task to new status and clear processing lock. Prevents race conditions.';

-- ============================================================================
-- RPC: claim_task_for_execution
-- Atomically claims task and sets in_progress
-- Replaces: set_processing + update_task_assignment (two separate calls)
-- ============================================================================

CREATE OR REPLACE FUNCTION claim_task_for_execution(
  p_task_id UUID,
  p_processing_by TEXT,
  p_assigned_to TEXT,
  p_routing_flag TEXT DEFAULT 'internal',
  p_routing_flag_reason TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  -- Atomic claim: set processing + status + assignment
  UPDATE tasks
  SET
    processing_by = p_processing_by,
    processing_at = NOW(),
    status = 'in_progress',
    assigned_to = p_assigned_to,
    routing_flag = p_routing_flag,
    routing_flag_reason = COALESCE(p_routing_flag_reason, routing_flag_reason),
    started_at = CASE WHEN started_at IS NULL THEN NOW() ELSE started_at END,
    updated_at = NOW()
  WHERE id = p_task_id AND processing_by IS NULL;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION claim_task_for_execution(UUID, TEXT, TEXT, TEXT, TEXT) IS
'Atomically claim task for execution. Returns TRUE if claim succeeded, FALSE if already claimed.';

SELECT 'Migration 083 complete - atomic task transition RPCs created' AS status;
