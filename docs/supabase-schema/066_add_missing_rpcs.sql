-- VibePilot Migration 066: Fix RPC Signatures
-- 
-- PROBLEM: Existing functions have different signatures than Go code expects
-- SOLUTION: Drop old functions with exact signatures, create new ones
--
-- Run in Supabase SQL Editor

-- ============================================================================
-- 1. update_task_assignment
-- EXISTS: (UUID, TEXT, TEXT) RETURNS VOID
-- NEEDS:  (UUID, TEXT, TEXT, TEXT, TEXT) RETURNS JSONB
-- ============================================================================

DROP FUNCTION IF EXISTS update_task_assignment(UUID, TEXT, TEXT);

CREATE OR REPLACE FUNCTION update_task_assignment(
  p_task_id UUID,
  p_status TEXT,
  p_assigned_to TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT 'internal',
  p_routing_flag_reason TEXT DEFAULT NULL
) RETURNS JSONB AS $$
DECLARE
  result JSONB;
BEGIN
  UPDATE tasks
  SET 
    status = COALESCE(p_status, status),
    assigned_to = COALESCE(p_assigned_to, assigned_to),
    routing_flag = COALESCE(p_routing_flag, routing_flag),
    routing_flag_reason = COALESCE(p_routing_flag_reason, routing_flag_reason),
    updated_at = NOW()
  WHERE id = p_task_id
  RETURNING jsonb_build_object(
    'id', id,
    'status', status,
    'assigned_to', assigned_to,
    'routing_flag', routing_flag
  ) INTO result;
  RETURN result;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 2. record_model_success
-- EXISTS: (TEXT, TEXT, FLOAT) RETURNS VOID
-- NEEDS:  (TEXT, TEXT, DECIMAL, INT) RETURNS VOID
-- ============================================================================

DROP FUNCTION IF EXISTS record_model_success(TEXT, TEXT, FLOAT);

CREATE OR REPLACE FUNCTION record_model_success(
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL,
  p_duration_seconds DECIMAL DEFAULT NULL,
  p_tokens_used INT DEFAULT 0
) RETURNS VOID AS $$
BEGIN
  UPDATE models
  SET 
    tasks_completed = COALESCE(tasks_completed, 0) + 1,
    tokens_used = COALESCE(tokens_used, 0) + p_tokens_used,
    success_rate = CASE 
      WHEN COALESCE(tasks_completed, 0) + COALESCE(tasks_failed, 0) + 1 > 0 
      THEN (COALESCE(tasks_completed, 0) + 1)::DECIMAL / (COALESCE(tasks_completed, 0) + COALESCE(tasks_failed, 0) + 1)
      ELSE 1.0
    END,
    updated_at = NOW()
  WHERE id = p_model_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 3. record_model_failure
-- EXISTS: (TEXT, TEXT, UUID) RETURNS JSONB  (params: model_id, failure_type, task_id)
-- NEEDS:  (TEXT, UUID, TEXT, TEXT) RETURNS VOID (params: model_id, task_id, failure_type, failure_category)
-- ============================================================================

DROP FUNCTION IF EXISTS record_model_failure(TEXT, TEXT, UUID);

CREATE OR REPLACE FUNCTION record_model_failure(
  p_model_id TEXT,
  p_task_id UUID DEFAULT NULL,
  p_failure_type TEXT DEFAULT NULL,
  p_failure_category TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
  UPDATE models
  SET 
    tasks_failed = COALESCE(tasks_failed, 0) + 1,
    success_rate = CASE 
      WHEN COALESCE(tasks_completed, 0) + COALESCE(tasks_failed, 0) + 1 > 0 
      THEN COALESCE(tasks_completed, 0)::DECIMAL / (COALESCE(tasks_completed, 0) + COALESCE(tasks_failed, 0) + 1)
      ELSE 0.0
    END,
    updated_at = NOW()
  WHERE id = p_model_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 4. create_task_run (NEW - doesn't exist yet)
-- ============================================================================

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
  p_completed_at TIMESTAMPTZ DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (
    task_id, model_id, courier, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
    started_at, completed_at
  ) VALUES (
    p_task_id, p_model_id, p_courier, p_platform, p_status,
    p_tokens_in, p_tokens_out, p_tokens_used,
    p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
    p_platform_theoretical_cost_usd, p_total_actual_cost_usd, p_total_savings_usd,
    p_started_at, COALESCE(p_completed_at, NOW())
  )
  RETURNING id INTO v_run_id;
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 5. calculate_run_costs (NEW - doesn't exist yet)
-- ============================================================================

CREATE OR REPLACE FUNCTION calculate_run_costs(
  p_model_id TEXT,
  p_tokens_in INT,
  p_tokens_out INT,
  p_courier_cost_usd DECIMAL DEFAULT 0
) RETURNS JSONB AS $$
DECLARE
  v_cost_input DECIMAL(10,6);
  v_cost_output DECIMAL(10,6);
  v_subscription DECIMAL(10,2);
  v_theoretical DECIMAL(10,6);
  v_actual DECIMAL(10,6);
  v_savings DECIMAL(10,6);
BEGIN
  SELECT cost_input_per_1k_usd, cost_output_per_1k_usd, subscription_cost_usd
  INTO v_cost_input, v_cost_output, v_subscription
  FROM models WHERE id = p_model_id;
  
  v_theoretical := (p_tokens_in::DECIMAL / 1000 * COALESCE(v_cost_input, 0)) +
                   (p_tokens_out::DECIMAL / 1000 * COALESCE(v_cost_output, 0));
  
  v_actual := p_courier_cost_usd;
  
  IF v_subscription IS NOT NULL AND v_subscription > 0 THEN
    v_actual := 0;
  END IF;
  
  v_savings := v_theoretical - v_actual;
  
  RETURN jsonb_build_object(
    'theoretical_cost_usd', v_theoretical,
    'actual_cost_usd', v_actual,
    'savings_usd', GREATEST(v_savings, 0)
  );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- GRANTS
-- ============================================================================

GRANT EXECUTE ON FUNCTION update_task_assignment TO service_role;
GRANT EXECUTE ON FUNCTION record_model_success TO service_role;
GRANT EXECUTE ON FUNCTION record_model_failure TO service_role;
GRANT EXECUTE ON FUNCTION create_task_run TO service_role;
GRANT EXECUTE ON FUNCTION calculate_run_costs TO service_role;

SELECT 'Migration 066 complete' AS status;
