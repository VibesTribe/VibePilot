-- Fix: Add result column to create_task_run INSERT
-- The task_runs.result column ALREADY EXISTS (from schema_v1_core.sql)
-- This fix updates the function to actually WRITE to it

DROP FUNCTION IF EXISTS create_task_run(
  UUID, TEXT, TEXT, TEXT, TEXT, INT, INT, INT,
  TEXT, INT, DECIMAL, DECIMAL, DECIMAL, DECIMAL, DECIMAL,
  TIMESTAMPTZ, TIMESTAMPTZ
);

CREATE OR REPLACE FUNCTION create_task_run(
  p_task_id UUID,
  p_model_id TEXT DEFAULT NULL,
  p_courier TEXT DEFAULT NULL,
  p_platform TEXT DEFAULT NULL,
  p_status TEXT DEFAULT 'success',
  p_tokens_in INT DEFAULT 0,
  p_tokens_out INT DEFAULT 0,
  p_tokens_used INT DEFAULT 0,
  p_courier_model_id TEXT DEFAULT NULL,
  p_courier_tokens INT DEFAULT 0,
  p_courier_cost_usd DECIMAL DEFAULT 0,
  p_platform_theoretical_cost_usd DECIMAL DEFAULT 0,
  p_total_actual_cost_usd DECIMAL DEFAULT 0,
  p_total_savings_usd DECIMAL DEFAULT 0,
  p_started_at TIMESTAMPTZ DEFAULT NOW(),
  p_completed_at TIMESTAMPTZ DEFAULT NULL,
  p_result JSONB DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (
    task_id, model_id, courier, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
    started_at, completed_at, result
  ) VALUES (
    p_task_id, p_model_id, p_courier, p_platform, p_status,
    p_tokens_in, p_tokens_out, p_tokens_used,
    p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
    p_platform_theoretical_cost_usd, p_total_actual_cost_usd, p_total_savings_usd,
    p_started_at, COALESCE(p_completed_at, NOW()), p_result
  )
  RETURNING id INTO v_run_id;
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION create_task_run TO service_role;

COMMENT ON FUNCTION create_task_run IS
'Create a task_run record with execution results.
The result field contains the task output (files_created, summary) for supervisor review.';

SELECT 'create_task_run now includes result column' AS status;
