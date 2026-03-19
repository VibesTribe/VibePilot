-- Migration 092: Fix transition_task and task constraint to include complete/merge_pending
-- CRITICAL BUG FIX: transition_task was missing 'complete' and 'merge_pending' statuses
-- This caused infinite loops: testing → complete (FAILED) → testing → recovery → available → repeat
--
-- Date: 2026-03-19

-- ============================================================================
-- PART 1: Fix transition_task to accept ALL valid statuses
-- ============================================================================

CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_failure_reason TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  -- All valid task statuses (must match constraint)
  IF p_new_status NOT IN (
    'pending', 'available', 'pending_resources',
    'in_progress', 'review', 'testing',
    'complete', 'merged', 'merge_pending',
    'approval', 'awaiting_human',
    'failed', 'escalated', 'council_review', 'blocked'
  ) THEN
    RAISE EXCEPTION 'Invalid status: %', p_new_status;
  END IF;

  UPDATE tasks
  SET 
    status = p_new_status,
    processing_by = NULL,
    processing_at = NULL,
    completed_at = CASE 
      WHEN p_new_status IN ('complete', 'merged', 'merge_pending') THEN NOW() 
      ELSE completed_at 
    END,
    failure_notes = CASE 
      WHEN p_failure_reason IS NOT NULL THEN 
        COALESCE(failure_notes || E'\n', '') || p_failure_reason || ' (' || NOW()::text || ')'
      ELSE failure_notes 
    END,
    updated_at = NOW()
  WHERE id = p_task_id;
  
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION transition_task(UUID, TEXT, TEXT) IS 
'Atomically transition task to new status and release processing lock. 
Valid statuses: pending, available, pending_resources, in_progress, review, testing, 
complete, merged, merge_pending, approval, awaiting_human, failed, escalated, council_review, blocked.
complete/merged/merge_pending set completed_at timestamp.';

-- ============================================================================
-- PART 2: Fix task constraint to include ALL valid statuses
-- ============================================================================

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending',           -- Created, waiting
    'available',         -- Ready to be claimed
    'pending_resources', -- Waiting for agent capacity
    'in_progress',       -- Agent actively working
    'review',            -- Supervisor reviewing output
    'testing',           -- Automated tests running
    'complete',          -- Tests passed, task done (before merge)
    'merged',            -- Code merged to module branch (FINAL)
    'merge_pending',     -- Tests passed but merge failed (will retry)
    'approval',          -- Waiting for approval (rare)
    'awaiting_human',    -- Needs human decision (visual UI, etc.)
    'failed',            -- Failed after max attempts
    'escalated',         -- Escalated to higher priority
    'council_review',    -- Council of agents reviewing
    'blocked'            -- Blocked by external dependency
  ));

-- ============================================================================
-- PART 3: Add status constants helper for documentation
-- ============================================================================

COMMENT ON COLUMN tasks.status IS 
'Task status flow:
pending → available → in_progress → review → testing → complete → merged
                                                        ↓
                                                 merge_pending (if merge fails)

Terminal states: merged, failed, awaiting_human
Human review ONLY for: visual UI/UX, paid API credit, complex researcher suggestions';

SELECT '092_fix_transition_task_statuses applied - added complete, merge_pending to transition_task and constraint' AS status;
