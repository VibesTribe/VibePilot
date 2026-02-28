-- VibePilot Migration: Model Scoring RPC
-- Version: 033
-- Purpose: Compute model score for task type based on success rate
-- Design: Simple success/failure ratio, no complex weights, computed on-the-fly
--
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- RPC: get_model_score_for_task
-- Returns: 0.0-1.0 based on success rate for this model + task_type
-- ============================================================================

CREATE OR REPLACE FUNCTION get_model_score_for_task(
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL
) RETURNS JSONB AS $$
DECLARE
  v_success_count INT := 0;
  v_failure_count INT := 0;
  v_total INT := 0;
  v_score FLOAT := 0.5;
BEGIN
  -- Count successes and failures from task_runs
  SELECT 
    COUNT(*) FILTER (WHERE tr.status = 'success'),
    COUNT(*) FILTER (WHERE tr.status IN ('failed', 'timeout'))
  INTO v_success_count, v_failure_count
  FROM task_runs tr
  JOIN tasks t ON t.id = tr.task_id
  WHERE tr.model_id = p_model_id
    AND (p_task_type IS NULL OR t.type = p_task_type);
  
  v_total := v_success_count + v_failure_count;
  
  -- Compute score
  IF v_total > 0 THEN
    v_score := v_success_count::FLOAT / v_total::FLOAT;
  ELSE
    -- No history = neutral
    v_score := 0.5;
  END IF;
  
  RETURN jsonb_build_object(
    'score', v_score,
    'success_count', v_success_count,
    'failure_count', v_failure_count,
    'total_runs', v_total
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- GRANT
-- ============================================================================

GRANT EXECUTE ON FUNCTION get_model_score_for_task TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE 'Migration 033 complete - Model Scoring RPC';
  RAISE NOTICE '  - get_model_score_for_task(model_id, task_type)';
  RAISE NOTICE '  - Returns score 0.0-1.0 based on success rate';
END $$;

COMMIT;
