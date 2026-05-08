-- VibePilot Migration: Depreciation/Revival System
-- Version: 026
-- Purpose: Track model performance, auto-archive underperformers, enable revival
-- Design: Configurable thresholds in governor.yaml, SQL handles scoring
--
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- 1. ADD 'archived' STATUS TO RUNNERS
-- ============================================================================

ALTER TABLE runners DROP CONSTRAINT IF EXISTS runners_status_check;
ALTER TABLE runners ADD CONSTRAINT runners_status_check 
  CHECK (status IN ('active', 'cooldown', 'rate_limited', 'paused', 'benched', 'archived'));

-- ============================================================================
-- 2. ADD DEPRECIATION TRACKING COLUMNS
-- ============================================================================

ALTER TABLE runners ADD COLUMN IF NOT EXISTS depreciation_score FLOAT DEFAULT 0;
ALTER TABLE runners ADD COLUMN IF NOT EXISTS depreciation_reasons JSONB DEFAULT '[]';
ALTER TABLE runners ADD COLUMN IF NOT EXISTS last_boosted_at TIMESTAMPTZ;
ALTER TABLE runners ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE runners ADD COLUMN IF NOT EXISTS archive_reason TEXT;
ALTER TABLE runners ADD COLUMN IF NOT EXISTS last_success_at TIMESTAMPTZ;

COMMENT ON COLUMN runners.depreciation_score IS '0=good, 1=terrible. Calculated as 1 - success_rate';
COMMENT ON COLUMN runners.depreciation_reasons IS 'Array of reasons for depreciation: ["high_failure_rate", "timeout_issues", etc.]';
COMMENT ON COLUMN runners.last_boosted_at IS 'When this runner was last manually boosted/reset';
COMMENT ON COLUMN runners.archived_at IS 'When this runner was archived';
COMMENT ON COLUMN runners.archive_reason IS 'Why this runner was archived';
COMMENT ON COLUMN runners.last_success_at IS 'When this runner last had a successful task';

-- Index for finding runners to archive
CREATE INDEX IF NOT EXISTS idx_runners_depreciation 
  ON runners(depreciation_score DESC) WHERE status = 'active';

-- ============================================================================
-- 3. RPC: update_depreciation_score
-- Called by record_runner_result to recalculate score
-- ============================================================================

CREATE OR REPLACE FUNCTION update_depreciation_score(p_runner_id UUID)
RETURNS VOID AS $$
DECLARE
  v_total_success INT;
  v_total_fail INT;
  v_total INT;
  v_success_rate FLOAT;
  v_new_score FLOAT;
  v_reasons JSONB := '[]'::jsonb;
BEGIN
  -- Sum all task types
  SELECT 
    COALESCE(SUM((task_ratings->>key->>'success')::int), 0),
    COALESCE(SUM((task_ratings->>key->>'fail')::int), 0)
  INTO v_total_success, v_total_fail
  FROM runners, jsonb_object_keys(task_ratings) AS key
  WHERE id = p_runner_id;
  
  -- Fallback: try single aggregate if above returns null
  IF v_total_success IS NULL THEN v_total_success := 0; END IF;
  IF v_total_fail IS NULL THEN v_total_fail := 0; END IF;
  
  v_total := v_total_success + v_total_fail;
  
  IF v_total > 0 THEN
    v_success_rate := v_total_success::float / v_total;
    v_new_score := 1.0 - v_success_rate;
    
    -- Build reasons based on patterns
    IF v_success_rate < 0.3 THEN
      v_reasons := v_reasons || '"low_success_rate"'::jsonb;
    END IF;
    
    -- Check for timeout patterns in failure_records
    IF EXISTS (
      SELECT 1 FROM failure_records 
      WHERE model_id = (SELECT model_id FROM runners WHERE id = p_runner_id)
      AND failure_type = 'timeout'
      AND created_at > NOW() - INTERVAL '24 hours'
      HAVING COUNT(*) >= 3
    ) THEN
      v_reasons := v_reasons || '"timeout_issues"'::jsonb;
    END IF;
    
    -- Check for rate limit patterns
    IF EXISTS (
      SELECT 1 FROM failure_records 
      WHERE model_id = (SELECT model_id FROM runners WHERE id = p_runner_id)
      AND failure_type = 'rate_limited'
      AND created_at > NOW() - INTERVAL '1 hour'
      HAVING COUNT(*) >= 2
    ) THEN
      v_reasons := v_reasons || '"rate_limit_issues"'::jsonb;
    END IF;
    
    UPDATE runners
    SET 
      depreciation_score = v_new_score,
      depreciation_reasons = v_reasons,
      updated_at = NOW()
    WHERE id = p_runner_id;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 4. RPC: archive_runner
-- Move runner to archived status
-- ============================================================================

CREATE OR REPLACE FUNCTION archive_runner(
  p_runner_id UUID,
  p_reason TEXT DEFAULT 'depreciation_threshold'
) RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET 
    status = 'archived',
    archived_at = NOW(),
    archive_reason = p_reason,
    updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 5. RPC: boost_runner
-- Reset depreciation, give runner fresh start
-- ============================================================================

CREATE OR REPLACE FUNCTION boost_runner(
  p_runner_id UUID
) RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET 
    depreciation_score = 0,
    depreciation_reasons = '[]'::jsonb,
    last_boosted_at = NOW(),
    status = 'active',
    archived_at = NULL,
    archive_reason = NULL,
    updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 6. RPC: revive_runner
-- Bring archived runner back to active
-- ============================================================================

CREATE OR REPLACE FUNCTION revive_runner(
  p_runner_id UUID,
  p_reason TEXT DEFAULT 'manual_revival'
) RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET 
    status = 'active',
    depreciation_score = 0,
    depreciation_reasons = '[]'::jsonb,
    archived_at = NULL,
    archive_reason = NULL,
    last_boosted_at = NOW(),
    updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 7. RPC: get_runners_to_archive
-- Find runners that meet archive criteria
-- ============================================================================

CREATE OR REPLACE FUNCTION get_runners_to_archive(
  p_threshold FLOAT DEFAULT 0.7,
  p_min_attempts INT DEFAULT 5,
  p_cooldown_hours INT DEFAULT 24
) RETURNS TABLE(
  id UUID,
  model_id TEXT,
  depreciation_score FLOAT,
  total_attempts INT,
  last_success_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    r.id,
    r.model_id,
    r.depreciation_score,
    (
      COALESCE((r.task_ratings->>'general'->>'success')::int, 0) +
      COALESCE((r.task_ratings->>'general'->>'fail')::int, 0)
    ) as total_attempts,
    r.last_success_at
  FROM runners r
  WHERE r.status = 'active'
    AND r.depreciation_score >= p_threshold
    AND (
      COALESCE((r.task_ratings->>'general'->>'success')::int, 0) +
      COALESCE((r.task_ratings->>'general'->>'fail')::int, 0)
    ) >= p_min_attempts
    AND (
      r.last_success_at IS NULL 
      OR r.last_success_at < NOW() - (p_cooldown_hours || ' hours')::interval
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 8. RPC: record_runner_success_timestamp
-- Update last_success_at when task succeeds
-- ============================================================================

CREATE OR REPLACE FUNCTION record_runner_success_timestamp(p_runner_id UUID)
RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET last_success_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 9. MODIFY record_runner_result TO UPDATE DEPRECIATION
-- ============================================================================

-- We need to recreate the function to add the depreciation update
-- First, let's update it to also track last_success_at

CREATE OR REPLACE FUNCTION record_runner_result(
  p_runner_id UUID,
  p_task_type TEXT,
  p_success BOOLEAN,
  p_tokens_used INT
)
RETURNS VOID AS $$
DECLARE
  v_success_inc INT;
  v_fail_inc INT;
  v_task_type TEXT;
BEGIN
  v_success_inc := CASE WHEN p_success THEN 1 ELSE 0 END;
  v_fail_inc := CASE WHEN p_success THEN 0 ELSE 1 END;
  v_task_type := COALESCE(p_task_type, 'general');
  
  -- Update runner's task_ratings (learning)
  UPDATE runners r
  SET 
    task_ratings = jsonb_set(
      COALESCE(r.task_ratings, '{}'::jsonb),
      ARRAY[v_task_type],
      jsonb_build_object(
        'success', COALESCE((r.task_ratings->v_task_type->>'success')::int, 0) + v_success_inc,
        'fail', COALESCE((r.task_rates->v_task_type->>'fail')::int, 0) + v_fail_inc
      )
    ),
    daily_used = r.daily_used + 1,
    updated_at = NOW()
  WHERE id = p_runner_id;
  
  -- Update last_success_at on success
  IF p_success THEN
    UPDATE runners SET last_success_at = NOW() WHERE id = p_runner_id;
  END IF;
  
  -- Update model's task counts
  UPDATE models m
  SET 
    tokens_used = COALESCE(m.tokens_used, 0) + COALESCE(p_tokens_used, 0),
    tasks_completed = COALESCE(m.tasks_completed, 0) + v_success_inc,
    tasks_failed = COALESCE(m.tasks_failed, 0) + v_fail_inc,
    updated_at = NOW()
  FROM runners r
  WHERE r.id = p_runner_id AND m.id = r.model_id;
  
  -- Update depreciation score
  PERFORM update_depreciation_score(p_runner_id);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 10. GRANTS
-- ============================================================================

GRANT EXECUTE ON FUNCTION update_depreciation_score(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION archive_runner(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION boost_runner(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION revive_runner(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_runners_to_archive(FLOAT, INT, INT) TO authenticated;
GRANT EXECUTE ON FUNCTION record_runner_success_timestamp(UUID) TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE 'Migration 026 complete';
  RAISE NOTICE '  - Added archived status';
  RAISE NOTICE '  - Added depreciation tracking columns';
  RAISE NOTICE '  - Added 6 new RPCs';
  RAISE NOTICE '  - Modified record_runner_result to track depreciation';
END $$;

COMMIT;
