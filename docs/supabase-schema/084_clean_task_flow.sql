-- Migration 084: Clean Task Flow - Atomic Operations
-- Purpose: Simple, race-condition-free task flow
-- Date: 2026-03-11
--
-- FLOW:
--   1. Task available → Agent claims (claim_task)
--   2. Agent executes → Output committed to task branch
--   3. Agent done → Task to review (transition_task)
--   4. Supervisor reviews → approved: testing, fail: available with notes
--   5. Tester tests → approved: merge to module, fail: available with notes
--   6. Merged = DONE forever
--
-- FAILURE HANDLING:
--   - Notes explain WHY it failed
--   - System learns from patterns
--   - Next attempt may use different model or modified prompt
--   - NO endless loops with same model doing same thing

-- Drop old duplicate/conflicting functions
DROP FUNCTION IF EXISTS claim_next_task CASCADE;
DROP FUNCTION IF EXISTS create_task_with_packet CASCADE;
DROP FUNCTION IF EXISTS create_task_if_not_exists CASCADE;
DROP FUNCTION IF EXISTS update_task_assignment CASCADE;
DROP FUNCTION IF EXISTS update_task_status CASCADE;
DROP FUNCTION IF EXISTS clear_processing CASCADE;
DROP FUNCTION IF EXISTS set_processing CASCADE;
DROP FUNCTION IF EXISTS complete_task_transition CASCADE;
DROP FUNCTION IF EXISTS claim_task_for_execution CASCADE;
DROP FUNCTION IF EXISTS claim_for_review CASCADE;
DROP FUNCTION IF EXISTS complete_review CASCADE;

-- ============================================================================
-- CLAIM_TASK: Atomically claim task for execution
-- ============================================================================

CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_worker_id TEXT,
  p_model_id TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT 'internal',
  p_routing_reason TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET 
    status = 'in_progress',
    processing_by = p_worker_id,
    processing_at = NOW(),
    assigned_to = COALESCE(p_model_id, assigned_to),
    routing_flag = p_routing_flag,
    routing_flag_reason = COALESCE(p_routing_reason, routing_flag_reason),
    started_at = COALESCE(started_at, NOW()),
    attempts = attempts + 1,
    updated_at = NOW()
  WHERE id = p_task_id 
    AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- CLAIM_FOR_REVIEW: Claim task for supervisor/tester (status unchanged)
-- ============================================================================

CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET 
    processing_by = p_reviewer_id,
    processing_at = NOW(),
    updated_at = NOW()
  WHERE id = p_task_id 
    AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- TRANSITION_TASK: Atomically set status and release lock
-- ============================================================================

CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_failure_reason TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  IF p_new_status NOT IN ('review', 'testing', 'approval', 'merged', 'available', 'awaiting_human') THEN
    RAISE EXCEPTION 'Invalid status: %', p_new_status;
  END IF;

  UPDATE tasks
  SET 
    status = p_new_status,
    processing_by = NULL,
    processing_at = NULL,
    completed_at = CASE WHEN p_new_status = 'merged' THEN NOW() ELSE completed_at END,
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

-- ============================================================================
-- UNLOCK_DEPENDENTS: When task merges, unlock dependent tasks
-- ============================================================================

CREATE OR REPLACE FUNCTION unlock_dependents(
  p_completed_task_id UUID
) RETURNS INT AS $$
DECLARE
  v_count INT;
BEGIN
  UPDATE tasks
  SET status = 'available', updated_at = NOW()
  WHERE p_completed_task_id = ANY(dependencies)
    AND status = 'pending'
    AND (
      dependencies = ARRAY[p_completed_task_id]
      OR NOT EXISTS (
        SELECT 1 FROM tasks t2 
        WHERE t2.id != p_completed_task_id 
        AND t2.id = ANY(tasks.dependencies)
        AND t2.status != 'merged'
      )
    );
  
  GET DIAGNOSTICS v_count = ROW_COUNT;
  RETURN v_count;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

SELECT '084_clean_task_flow applied' AS status;
