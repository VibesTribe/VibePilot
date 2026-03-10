-- ============================================================================
-- 080_problem_solutions_rpc.sql
-- Purpose: Add RPC to record solutions when tasks succeed after failures
-- ============================================================================

-- RPC: record_solution_on_success
-- Called when a task succeeds - checks for previous failures and records solution
CREATE OR REPLACE FUNCTION record_solution_on_success(
  p_task_id UUID,
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_failure failure_records%ROWTYPE;
  v_solution_id UUID;
  v_keywords TEXT[] := '{}';
BEGIN
  -- Find the most recent failure for this task
  SELECT * INTO v_failure
  FROM failure_records
  WHERE task_id = p_task_id
  ORDER BY created_at DESC
  LIMIT 1;
  
  -- No previous failure, nothing to record
  IF NOT FOUND THEN
    RETURN NULL;
  END IF;
  
  -- Build keywords from failure info
  IF v_failure.failure_type IS NOT NULL THEN
    v_keywords := array_append(v_keywords, v_failure.failure_type);
  END IF;
  IF v_failure.failure_category IS NOT NULL THEN
    v_keywords := array_append(v_keywords, v_failure.failure_category);
  END IF;
  IF p_task_type IS NOT NULL THEN
    v_keywords := array_append(v_keywords, p_task_type);
  END IF;
  
  -- Check if similar solution already exists
  SELECT id INTO v_solution_id
  FROM problem_solutions
  WHERE problem_pattern = v_failure.failure_type
    AND solution_type = 'reroute'
    AND solution_model = p_model_id;
  
  IF v_solution_id IS NOT NULL THEN
    -- Update existing solution
    UPDATE problem_solutions
    SET 
      success_count = success_count + 1,
      success_rate = (success_count + 1)::FLOAT / (success_count + failure_count + 1),
      last_used_at = NOW()
    WHERE id = v_solution_id;
    
    RETURN v_solution_id;
  END IF;
  
  -- Create new solution record
  INSERT INTO problem_solutions (
    problem_pattern,
    problem_category,
    solution_type,
    solution_model,
    solution_details,
    keywords,
    success_count,
    success_rate
  ) VALUES (
    v_failure.failure_type,
    v_failure.failure_category,
    'reroute',
    p_model_id,
    jsonb_build_object(
      'original_failure', v_failure.failure_details,
      'task_type', p_task_type
    ),
    v_keywords,
    1,
    1.0
  ) RETURNING id INTO v_solution_id;
  
  RETURN v_solution_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_solution_on_success(UUID, TEXT, TEXT) IS 
'Records a solution when a task succeeds after previous failure. Links failure pattern to successful model.';

-- Grant access
GRANT EXECUTE ON FUNCTION record_solution_on_success TO authenticated;
