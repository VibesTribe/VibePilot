-- VibePilot Migration 066: Add Missing RPCs
-- Purpose: Add RPCs that Go code needs
-- Date: 2026-03-06
-- 
-- NOTE: Column additions were done in schema_v1.4_roi_enhanced.sql
-- This migration only adds the RPC functions

-- ============================================================================
-- RPC: update_task_assignment - Include routing_flag
-- ============================================================================

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
-- RPC: create_task_run - Full task_runs creation with all fields
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
    task_id,
    model_id,
    courier,
    platform,
    status,
    tokens_in,
    tokens_out,
    tokens_used,
    courier_model_id,
    courier_tokens,
    courier_cost_usd,
    platform_theoretical_cost_usd,
    total_actual_cost_usd,
    total_savings_usd,
    started_at,
    completed_at
  ) VALUES (
    p_task_id,
    p_model_id,
    p_courier,
    p_platform,
    p_status,
    p_tokens_in,
    p_tokens_out,
    p_tokens_used,
    p_courier_model_id,
    p_courier_tokens,
    p_courier_cost_usd,
    p_platform_theoretical_cost_usd,
    p_total_actual_cost_usd,
    p_total_savings_usd,
    p_started_at,
    COALESCE(p_completed_at, NOW())
  )
  RETURNING id INTO v_run_id;
  
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- RPC: record_model_success - Learning system
-- ============================================================================

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
-- RPC: record_model_failure - Learning system
-- ============================================================================

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
  
  -- If failure category indicates rate limit, set cooldown
  IF p_failure_category = 'rate_limit' THEN
    UPDATE models
    SET 
      status = 'paused',
      status_reason = 'Rate limit hit',
      cooldown_expires_at = NOW() + INTERVAL '1 hour'
    WHERE id = p_model_id;
  END IF;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- RPC: calculate_run_costs - Calculate ROI for a task run
-- ============================================================================

CREATE OR REPLACE FUNCTION calculate_run_costs(
  p_model_id TEXT,
  p_tokens_in INT,
  p_tokens_out INT,
  p_courier_cost_usd DECIMAL DEFAULT 0
) RETURNS JSONB AS $$
DECLARE
  v_model RECORD;
  v_theoretical_cost DECIMAL(10,6);
  v_actual_cost DECIMAL(10,6);
  v_savings DECIMAL(10,6);
BEGIN
  -- Get model costs
  SELECT cost_input_per_1k_usd, cost_output_per_1k_usd, subscription_cost_usd
  INTO v_model
  FROM models
  WHERE id = p_model_id;
  
  IF NOT FOUND THEN
    RETURN jsonb_build_object(
      'theoretical_cost_usd', 0,
      'actual_cost_usd', p_courier_cost_usd,
      'savings_usd', 0
    );
  END IF;
  
  -- Calculate theoretical cost (what API would cost)
  v_theoretical_cost := 
    (p_tokens_in::DECIMAL / 1000 * COALESCE(v_model.cost_input_per_1k_usd, 0)) +
    (p_tokens_out::DECIMAL / 1000 * COALESCE(v_model.cost_output_per_1k_usd, 0));
  
  -- Actual cost is courier cost (for web platforms) or prorated subscription
  v_actual_cost := p_courier_cost_usd;
  
  -- If subscription, prorate it
  IF v_model.subscription_cost_usd IS NOT NULL AND v_model.subscription_cost_usd > 0 THEN
    -- For now, actual cost is 0 for subscription models (already paid)
    v_actual_cost := 0;
  END IF;
  
  -- Savings = theoretical - actual
  v_savings := v_theoretical_cost - v_actual_cost;
  
  RETURN jsonb_build_object(
    'theoretical_cost_usd', v_theoretical_cost,
    'actual_cost_usd', v_actual_cost,
    'savings_usd', GREATEST(v_savings, 0)
  );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- GRANT PERMISSIONS
-- ============================================================================

GRANT EXECUTE ON FUNCTION update_task_assignment TO service_role;
GRANT EXECUTE ON FUNCTION create_task_run TO service_role;
GRANT EXECUTE ON FUNCTION record_model_success TO service_role;
GRANT EXECUTE ON FUNCTION record_model_failure TO service_role;
GRANT EXECUTE ON FUNCTION calculate_run_costs TO service_role;

-- ============================================================================
-- SUCCESS MESSAGE
-- ============================================================================

SELECT 'Migration 066 complete - RPCs added: update_task_assignment, create_task_run, record_model_success, record_model_failure, calculate_run_costs' AS status;
