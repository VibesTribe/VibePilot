-- Migration 143: Rewrite get_model_score_for_task to query task_runs directly
-- Replaces the old function that queried the empty model_scores table.
-- Now computes success rate per model per task type/category on the fly
-- from actual task execution data. Uses Bayesian averaging so models with
-- few runs don't get extreme scores.
--
-- Once this is applied, the governor's router will immediately start using
-- real data as tasks complete. First task finishes → routing learns.

-- Drop old version (takes 3 params)
DROP FUNCTION IF EXISTS get_model_score_for_task(TEXT, TEXT, TEXT) CASCADE;

CREATE OR REPLACE FUNCTION get_model_score_for_task(
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL,
  p_task_category TEXT DEFAULT NULL
)
RETURNS NUMERIC
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  v_success NUMERIC;
  v_total NUMERIC;
  v_score NUMERIC;
BEGIN
  -- Count successes and total runs for this model, filtered by task type/category
  SELECT
    COUNT(*) FILTER (WHERE tr.status = 'success')::NUMERIC,
    COUNT(*)::NUMERIC
  INTO v_success, v_total
  FROM task_runs tr
  JOIN tasks t ON t.id = tr.task_id
  WHERE tr.model_id = p_model_id
    AND tr.status IN ('success', 'failed')
    AND (p_task_type IS NULL OR t.type = p_task_type)
    AND (p_task_category IS NULL OR t.category = p_task_category);

  -- No data yet — neutral score
  IF v_total = 0 THEN
    RETURN 0.5;
  END IF;

  -- Bayesian average: blends observed rate with a prior of 0.5 (weight 5 runs)
  -- 0/0  → 0.50    1/1  → 0.58    10/10 → 0.95
  -- 5/10 → 0.50    0/5  → 0.25    9/10  → 0.77
  v_score := (v_success + 2.5) / (v_total + 5.0);

  RETURN v_score;
END;
$$;

-- Grant execute to service role
GRANT EXECUTE ON FUNCTION get_model_score_for_task(TEXT, TEXT, TEXT) TO anon, authenticated, service_role;

-- Verify
SELECT 'Migration 143 complete: get_model_score_for_task now queries task_runs directly with Bayesian averaging' AS status;
