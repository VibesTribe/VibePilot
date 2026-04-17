-- Migration 119: Fix claim_task (set assigned_to) and create create_task_run RPC
-- 
-- Dashboard reads tasks.assigned_to to show which model is working
-- Dashboard reads task_runs for token counts, costs, ROI
-- See docs/HOW_DASHBOARD_WORKS.md

-- 1. claim_task: set assigned_to = model ID, routing_flag for dashboard
CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_worker_id TEXT DEFAULT NULL,
  p_model_id TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT NULL,
  p_routing_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_model_id,
      processing_by = p_worker_id,
      processing_at = NOW(),
      routing_flag = COALESCE(p_routing_flag, routing_flag),
      routing_flag_reason = p_routing_reason,
      updated_at = NOW()
  WHERE id = p_task_id AND status = 'available' AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. create_task_run: record execution results for dashboard token/cost/ROI display
-- Columns that exist: id, task_id, model_id, platform, status, tokens_in, tokens_out, 
--   tokens_used, courier_tokens, courier_cost_usd, courier_model_id,
--   platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
--   started_at, completed_at, result, chat_url
DROP FUNCTION IF EXISTS create_task_run(UUID, TEXT, TEXT, TEXT, TEXT, INTEGER, INTEGER, INTEGER, TEXT, INTEGER, DECIMAL, DECIMAL, DECIMAL, DECIMAL, TIMESTAMPTZ, TIMESTAMPTZ, TEXT);
CREATE OR REPLACE FUNCTION create_task_run(
  p_task_id UUID,
  p_model_id TEXT DEFAULT NULL,
  p_courier TEXT DEFAULT NULL,
  p_platform TEXT DEFAULT NULL,
  p_status TEXT DEFAULT 'success',
  p_tokens_in INTEGER DEFAULT 0,
  p_tokens_out INTEGER DEFAULT 0,
  p_tokens_used INTEGER DEFAULT 0,
  p_courier_model_id TEXT DEFAULT NULL,
  p_courier_tokens INTEGER DEFAULT 0,
  p_courier_cost_usd DECIMAL DEFAULT 0,
  p_platform_theoretical_cost_usd DECIMAL DEFAULT 0,
  p_total_actual_cost_usd DECIMAL DEFAULT 0,
  p_total_savings_usd DECIMAL DEFAULT 0,
  p_started_at TIMESTAMPTZ DEFAULT NOW(),
  p_completed_at TIMESTAMPTZ DEFAULT NOW(),
  p_result JSONB DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (
    task_id, model_id, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd,
    total_actual_cost_usd, total_savings_usd,
    started_at, completed_at, result
  ) VALUES (
    p_task_id, p_model_id, p_platform, p_status,
    p_tokens_in, p_tokens_out, p_tokens_used,
    p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
    p_platform_theoretical_cost_usd,
    p_total_actual_cost_usd, p_total_savings_usd,
    p_started_at, p_completed_at, p_result
  )
  RETURNING id INTO v_run_id;
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
