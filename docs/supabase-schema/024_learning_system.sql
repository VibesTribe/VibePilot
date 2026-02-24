-- VibePilot Migration: Learning System Phase 1
-- Version: 024
-- Purpose: Core learning infrastructure - heuristics, failures, solutions
-- Design: Orchestrator learns from outcomes, routes smarter
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- 1. LEARNED HEURISTICS (model preferences per task type/condition)
-- ============================================================================

CREATE TABLE IF NOT EXISTS learned_heuristics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- What task does this apply to?
  task_type TEXT,
  condition JSONB DEFAULT '{}',
  -- Example: {"language": "python", "complexity": "high"}
  
  -- What should we do?
  preferred_model TEXT,
  -- Model ID to prefer (soft preference, not requirement)
  
  action JSONB DEFAULT '{}',
  -- Example: {"timeout_adjustment": 60, "reroute_after_failures": 2}
  
  -- How confident? Should we auto-apply?
  confidence FLOAT DEFAULT 0.5 CHECK (confidence BETWEEN 0 AND 1),
  auto_apply BOOLEAN DEFAULT false,
  
  -- Tracking
  source TEXT,
  -- 'llm_analysis', 'statistical', 'human'
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  last_applied_at TIMESTAMPTZ,
  application_count INT DEFAULT 0,
  success_count INT DEFAULT 0,
  failure_count INT DEFAULT 0,
  success_rate FLOAT,
  
  -- Expiration (re-learn if stale)
  expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '7 days')
);

CREATE INDEX IF NOT EXISTS idx_heuristics_task_type ON learned_heuristics(task_type);
CREATE INDEX IF NOT EXISTS idx_heuristics_expires ON learned_heuristics(expires_at) WHERE expires_at IS NOT NULL;

COMMENT ON TABLE learned_heuristics IS 'LLM-discovered routing optimizations';
COMMENT ON COLUMN learned_heuristics.condition IS 'JSON conditions for when this heuristic applies';
COMMENT ON COLUMN learned_heuristics.action IS 'Additional actions like timeout adjustments';

-- ============================================================================
-- 2. FAILURE RECORDS (structured failure logging)
-- ============================================================================

CREATE TABLE IF NOT EXISTS failure_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  task_run_id UUID REFERENCES task_runs(id) ON DELETE SET NULL,
  
  -- Structured failure info
  failure_type TEXT NOT NULL,
  -- 'timeout', 'rate_limited', 'context_exceeded', 'platform_down', 
  -- 'quality_rejected', 'test_failed', 'empty_output', 'latency_high'
  
  failure_category TEXT NOT NULL,
  -- 'model_issue', 'platform_issue', 'quality_issue', 'task_issue'
  
  failure_details JSONB DEFAULT '{}',
  -- Example: {"timeout_sec": 300, "error_message": "..."}
  
  -- What was attempted
  model_id TEXT,
  platform TEXT,
  runner_id UUID REFERENCES runners(id) ON DELETE SET NULL,
  
  -- Context
  task_type TEXT,
  tokens_used INT,
  duration_sec INT,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_failure_type ON failure_records(failure_type);
CREATE INDEX IF NOT EXISTS idx_failure_category ON failure_records(failure_category);
CREATE INDEX IF NOT EXISTS idx_failure_task_type ON failure_records(task_type, failure_type);
CREATE INDEX IF NOT EXISTS idx_failure_model ON failure_records(model_id);
CREATE INDEX IF NOT EXISTS idx_failure_created ON failure_records(created_at DESC);

COMMENT ON TABLE failure_records IS 'Structured failure records for pattern analysis';
COMMENT ON COLUMN failure_records.failure_type IS 'Specific type of failure';
COMMENT ON COLUMN failure_records.failure_category IS 'High-level category for routing decisions';

-- ============================================================================
-- 3. PROBLEM SOLUTIONS (what fixed what)
-- ============================================================================

CREATE TABLE IF NOT EXISTS problem_solutions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- The problem
  problem_pattern TEXT NOT NULL,
  -- Example: "timeout on complex python"
  
  problem_category TEXT,
  -- 'model_issue', 'platform_issue', 'task_issue'
  
  -- What worked
  solution_type TEXT NOT NULL,
  -- 'reroute', 'split_task', 'increase_timeout', 'clarify_prompt'
  
  solution_model TEXT,
  -- Which model succeeded (if reroute)
  
  solution_details JSONB DEFAULT '{}',
  -- Example: {"timeout_increased_to": 600}
  
  -- Proof it worked
  success_count INT DEFAULT 1,
  failure_count INT DEFAULT 0,
  success_rate FLOAT DEFAULT 1.0,
  
  -- For matching future problems
  keywords TEXT[] DEFAULT '{}',
  -- ['python', 'timeout', 'complex']
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_solution_pattern ON problem_solutions(problem_pattern);
CREATE INDEX IF NOT EXISTS idx_solution_category ON problem_solutions(problem_category);
CREATE INDEX IF NOT EXISTS idx_solution_keywords ON problem_solutions USING GIN(keywords);

COMMENT ON TABLE problem_solutions IS 'Proven solutions to recurring problems';
COMMENT ON COLUMN problem_solutions.solution_type IS 'Type of solution that worked';

-- ============================================================================
-- 4. RPC: record_failure
-- ============================================================================

CREATE OR REPLACE FUNCTION record_failure(
  p_task_id UUID DEFAULT NULL,
  p_task_run_id UUID DEFAULT NULL,
  p_failure_type TEXT DEFAULT NULL,
  p_failure_category TEXT DEFAULT NULL,
  p_failure_details JSONB DEFAULT '{}',
  p_model_id TEXT DEFAULT NULL,
  p_platform TEXT DEFAULT NULL,
  p_runner_id UUID DEFAULT NULL,
  p_task_type TEXT DEFAULT NULL,
  p_tokens_used INT DEFAULT NULL,
  p_duration_sec INT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO failure_records (
    task_id, task_run_id, failure_type, failure_category, failure_details,
    model_id, platform, runner_id, task_type, tokens_used, duration_sec
  ) VALUES (
    p_task_id, p_task_run_id, p_failure_type, p_failure_category, p_failure_details,
    p_model_id, p_platform, p_runner_id, p_task_type, p_tokens_used, p_duration_sec
  ) RETURNING id INTO v_id;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 5. RPC: get_heuristic
-- ============================================================================

CREATE OR REPLACE FUNCTION get_heuristic(
  p_task_type TEXT DEFAULT NULL,
  p_condition JSONB DEFAULT '{}'
) RETURNS TABLE(
  id UUID,
  preferred_model TEXT,
  action JSONB,
  confidence FLOAT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    lh.id,
    lh.preferred_model,
    lh.action,
    lh.confidence
  FROM learned_heuristics lh
  WHERE 
    (lh.task_type IS NULL OR lh.task_type = p_task_type)
    AND (lh.expires_at IS NULL OR lh.expires_at > NOW())
    AND lh.confidence >= 0.5
  ORDER BY 
    -- Exact task type match first
    CASE WHEN lh.task_type = p_task_type THEN 0 ELSE 1 END,
    lh.confidence DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 6. RPC: get_problem_solution
-- ============================================================================

CREATE OR REPLACE FUNCTION get_problem_solution(
  p_failure_type TEXT DEFAULT NULL,
  p_task_type TEXT DEFAULT NULL,
  p_keywords TEXT[] DEFAULT '{}'
) RETURNS TABLE(
  id UUID,
  solution_type TEXT,
  solution_model TEXT,
  solution_details JSONB,
  success_rate FLOAT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    ps.id,
    ps.solution_type,
    ps.solution_model,
    ps.solution_details,
    ps.success_rate
  FROM problem_solutions ps
  WHERE 
    ps.problem_pattern ILIKE '%' || p_failure_type || '%'
    OR p_failure_type = ANY(ps.keywords)
    OR ps.keywords && p_keywords
  ORDER BY 
    ps.success_rate DESC,
    ps.success_count DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 7. RPC: record_heuristic_result
-- ============================================================================

CREATE OR REPLACE FUNCTION record_heuristic_result(
  p_heuristic_id UUID,
  p_success BOOLEAN
) RETURNS VOID AS $$
BEGIN
  UPDATE learned_heuristics
  SET 
    application_count = application_count + 1,
    success_count = success_count + CASE WHEN p_success THEN 1 ELSE 0 END,
    failure_count = failure_count + CASE WHEN p_success THEN 0 ELSE 1 END,
    success_rate = (success_count + CASE WHEN p_success THEN 1 ELSE 0 END)::FLOAT / 
                   (application_count + 1),
    last_applied_at = NOW()
  WHERE id = p_heuristic_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 8. RPC: record_solution_result
-- ============================================================================

CREATE OR REPLACE FUNCTION record_solution_result(
  p_solution_id UUID,
  p_success BOOLEAN
) RETURNS VOID AS $$
BEGIN
  UPDATE problem_solutions
  SET 
    success_count = success_count + CASE WHEN p_success THEN 1 ELSE 0 END,
    failure_count = failure_count + CASE WHEN p_success THEN 0 ELSE 1 END,
    success_rate = (success_count + CASE WHEN p_success THEN 1 ELSE 0 END)::FLOAT / 
                   (success_count + failure_count + 1),
    last_used_at = NOW()
  WHERE id = p_solution_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 9. RPC: get_recent_failures (for routing exclusions)
-- ============================================================================

CREATE OR REPLACE FUNCTION get_recent_failures(
  p_task_type TEXT DEFAULT NULL,
  p_since TIMESTAMPTZ DEFAULT (NOW() - INTERVAL '1 hour')
) RETURNS TABLE(
  model_id TEXT,
  platform TEXT,
  failure_type TEXT,
  failure_count BIGINT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    fr.model_id,
    fr.platform,
    fr.failure_type,
    COUNT(*) as failure_count
  FROM failure_records fr
  WHERE 
    fr.created_at >= p_since
    AND (p_task_type IS NULL OR fr.task_type = p_task_type)
    AND fr.model_id IS NOT NULL
  GROUP BY fr.model_id, fr.platform, fr.failure_type
  HAVING COUNT(*) >= 2;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 10. RPC: upsert_heuristic (for LLM analysis)
-- ============================================================================

CREATE OR REPLACE FUNCTION upsert_heuristic(
  p_task_type TEXT DEFAULT NULL,
  p_condition JSONB DEFAULT '{}',
  p_preferred_model TEXT DEFAULT NULL,
  p_action JSONB DEFAULT '{}',
  p_confidence FLOAT DEFAULT 0.5,
  p_source TEXT DEFAULT 'llm_analysis'
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  -- Check if similar heuristic exists
  SELECT id INTO v_id
  FROM learned_heuristics
  WHERE task_type = p_task_type
    AND condition = p_condition
    AND (expires_at IS NULL OR expires_at > NOW());
  
  IF v_id IS NOT NULL THEN
    -- Update existing
    UPDATE learned_heuristics
    SET 
      preferred_model = p_preferred_model,
      action = p_action,
      confidence = p_confidence,
      source = p_source,
      expires_at = NOW() + INTERVAL '7 days'
    WHERE id = v_id;
  ELSE
    -- Create new
    INSERT INTO learned_heuristics (
      task_type, condition, preferred_model, action, confidence, source
    ) VALUES (
      p_task_type, p_condition, p_preferred_model, p_action, p_confidence, p_source
    ) RETURNING id INTO v_id;
  END IF;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 11. GRANTS
-- ============================================================================

GRANT SELECT, INSERT, UPDATE ON learned_heuristics TO authenticated;
GRANT SELECT, INSERT, UPDATE ON failure_records TO authenticated;
GRANT SELECT, INSERT, UPDATE ON problem_solutions TO authenticated;

GRANT EXECUTE ON FUNCTION record_failure TO authenticated;
GRANT EXECUTE ON FUNCTION get_heuristic TO authenticated;
GRANT EXECUTE ON FUNCTION get_problem_solution TO authenticated;
GRANT EXECUTE ON FUNCTION record_heuristic_result TO authenticated;
GRANT EXECUTE ON FUNCTION record_solution_result TO authenticated;
GRANT EXECUTE ON FUNCTION get_recent_failures TO authenticated;
GRANT EXECUTE ON FUNCTION upsert_heuristic TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE 'Migration 024 complete';
  RAISE NOTICE '  - learned_heuristics table';
  RAISE NOTICE '  - failure_records table';
  RAISE NOTICE '  - problem_solutions table';
  RAISE NOTICE '  - 7 new RPCs';
END $$;

COMMIT;
