-- VIBESPILOT SCHEMA MIGRATION v1.4
-- Purpose: Enhanced ROI tracking for courier-based execution
-- Date: 2026-02-17
-- 
-- Run this AFTER schema_project_tracking.sql
-- 
-- Changes:
--   - Split tokens into tokens_in/tokens_out (output costs more)
--   - Track courier costs separately
--   - Slice-level ROI rollup
--   - Subscription renewal tracking
--   - Exchange rate support (CAD)

-- ============================================
-- TASK_RUNS ENHANCEMENTS
-- ============================================

-- Add token breakdown (input vs output - output usually costs 2-3x more)
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS tokens_in INT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS tokens_out INT DEFAULT 0;

-- Courier tracking (what drove the browser-use session)
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS courier_model_id TEXT;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS courier_tokens INT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS courier_cost_usd DECIMAL(10,6) DEFAULT 0;

-- Enhanced cost tracking
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS platform_theoretical_cost_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS total_actual_cost_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS total_savings_usd DECIMAL(10,6) DEFAULT 0;

-- ============================================
-- MODELS ENHANCEMENTS (subscription tracking)
-- ============================================

ALTER TABLE models ADD COLUMN IF NOT EXISTS subscription_cost_usd DECIMAL(10,2) DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS subscription_started_at TIMESTAMPTZ;
ALTER TABLE models ADD COLUMN IF NOT EXISTS subscription_ends_at TIMESTAMPTZ;
ALTER TABLE models ADD COLUMN IF NOT EXISTS subscription_status TEXT DEFAULT 'none' 
  CHECK (subscription_status IN ('none', 'active', 'expired', 'cancelled'));

-- API cost breakdown (input/output often differ)
ALTER TABLE models ADD COLUMN IF NOT EXISTS cost_input_per_1k_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS cost_output_per_1k_usd DECIMAL(10,6) DEFAULT 0;

-- ============================================
-- PLATFORMS ENHANCEMENTS (for theoretical cost)
-- ============================================

ALTER TABLE platforms ADD COLUMN IF NOT EXISTS theoretical_cost_input_per_1k_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE platforms ADD COLUMN IF NOT EXISTS theoretical_cost_output_per_1k_usd DECIMAL(10,6) DEFAULT 0;

-- ============================================
-- SLICE ROI VIEW
-- Note: Slices are derived from tasks.slice_id, no separate table needed
-- ============================================

CREATE OR REPLACE VIEW slice_roi AS
SELECT 
  t.slice_id,
  COALESCE(
    CASE 
      WHEN t.slice_id IS NOT NULL THEN 
        UPPER(SUBSTRING(t.slice_id, 1, 1)) || LOWER(SUBSTRING(t.slice_id FROM 2))
      ELSE 'General'
    END,
    'General'
  ) as slice_name,
  COUNT(DISTINCT t.id) as total_tasks,
  COUNT(DISTINCT CASE WHEN t.status = 'merged' THEN t.id END) as completed_tasks,
  COALESCE(SUM(r.tokens_in), 0) as total_tokens_in,
  COALESCE(SUM(r.tokens_out), 0) as total_tokens_out,
  COALESCE(SUM(r.courier_tokens), 0) as total_courier_tokens,
  ROUND(COALESCE(SUM(r.platform_theoretical_cost_usd), 0)::numeric, 4) as theoretical_cost_usd,
  ROUND(COALESCE(SUM(r.total_actual_cost_usd), 0)::numeric, 4) as actual_cost_usd,
  ROUND(COALESCE(SUM(r.total_savings_usd), 0)::numeric, 4) as savings_usd,
  CASE 
    WHEN COUNT(DISTINCT t.id) > 0 
    THEN ROUND(((COUNT(DISTINCT CASE WHEN t.status = 'merged' THEN t.id END)::FLOAT / COUNT(DISTINCT t.id)) * 100)::numeric, 1)
    ELSE 0 
  END as completion_pct
FROM tasks t
LEFT JOIN task_runs r ON r.task_id = t.id
GROUP BY t.slice_id;

-- ============================================
-- FUNCTION: Calculate enhanced task ROI
-- ============================================

CREATE OR REPLACE FUNCTION calculate_enhanced_task_roi(p_run_id UUID)
RETURNS VOID AS $$
DECLARE
  v_tokens_in INT;
  v_tokens_out INT;
  v_model_id TEXT;
  v_platform_id TEXT;
  v_courier_model_id TEXT;
  v_courier_tokens INT;
  
  v_model_cost_in DECIMAL(10,6);
  v_model_cost_out DECIMAL(10,6);
  v_platform_cost_in DECIMAL(10,6);
  v_platform_cost_out DECIMAL(10,6);
  v_courier_cost DECIMAL(10,6);
  
  v_theoretical DECIMAL(10,6);
  v_actual DECIMAL(10,6);
  v_savings DECIMAL(10,6);
BEGIN
  -- Get run data
  SELECT 
    r.tokens_in, r.tokens_out, r.model_id, r.platform, 
    r.courier_model_id, r.courier_tokens
  INTO 
    v_tokens_in, v_tokens_out, v_model_id, v_platform_id,
    v_courier_model_id, v_courier_tokens
  FROM task_runs r 
  WHERE r.id = p_run_id;
  
  -- Get platform theoretical costs (what it WOULD cost via API)
  SELECT 
    COALESCE(theoretical_cost_input_per_1k_usd, 0),
    COALESCE(theoretical_cost_output_per_1k_usd, 0)
  INTO v_platform_cost_in, v_platform_cost_out
  FROM platforms 
  WHERE id = v_platform_id;
  
  -- Calculate theoretical cost (what platform API would charge)
  v_theoretical := 
    (COALESCE(v_tokens_in, 0) / 1000.0) * v_platform_cost_in +
    (COALESCE(v_tokens_out, 0) / 1000.0) * v_platform_cost_out;
  
  -- Calculate courier cost (what we paid for the courier model)
  IF v_courier_model_id IS NOT NULL THEN
    SELECT 
      COALESCE(cost_input_per_1k_usd, 0),
      COALESCE(cost_output_per_1k_usd, 0)
    INTO v_model_cost_in, v_model_cost_out
    FROM models 
    WHERE id = v_courier_model_id;
    
    -- Courier cost = courier tokens × courier model rate
    v_courier_cost := (COALESCE(v_courier_tokens, 0) / 1000.0) * 
      GREATEST(v_model_cost_in, v_model_cost_out);
  ELSE
    v_courier_cost := 0;
  END IF;
  
  -- Total actual cost = courier cost (subscriptions handled separately)
  v_actual := v_courier_cost;
  
  -- Savings = theoretical - actual
  v_savings := v_theoretical - v_actual;
  
  -- Update run with ROI
  UPDATE task_runs
  SET 
    platform_theoretical_cost_usd = v_theoretical,
    total_actual_cost_usd = v_actual,
    total_savings_usd = v_savings
  WHERE id = p_run_id;
  
  -- Update project cumulative totals
  UPDATE projects p
  SET 
    total_tokens_used = total_tokens_used + COALESCE(v_tokens_in, 0) + COALESCE(v_tokens_out, 0),
    total_theoretical_cost = total_theoretical_cost + v_theoretical,
    total_actual_cost = total_actual_cost + v_actual,
    total_savings = total_savings + v_savings,
    updated_at = NOW()
  FROM tasks t
  WHERE t.id = (SELECT task_id FROM task_runs WHERE id = p_run_id)
    AND p.id = t.project_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- FUNCTION: Get subscription ROI (for renewal decisions)
-- ============================================

CREATE OR REPLACE FUNCTION get_subscription_roi(p_model_id TEXT)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
  v_sub_cost DECIMAL(10,2);
  v_sub_start TIMESTAMPTZ;
  v_sub_end TIMESTAMPTZ;
  v_days_used INT;
  v_days_total INT;
  v_prorated_cost DECIMAL(10,6);
BEGIN
  -- Get subscription details
  SELECT 
    subscription_cost_usd, 
    subscription_started_at, 
    subscription_ends_at
  INTO v_sub_cost, v_sub_start, v_sub_end
  FROM models WHERE id = p_model_id;
  
  -- Calculate days
  v_days_used := EXTRACT(DAY FROM NOW() - COALESCE(v_sub_start, NOW()));
  v_days_total := EXTRACT(DAY FROM COALESCE(v_sub_end, NOW()) - COALESCE(v_sub_start, NOW()));
  v_days_total := GREATEST(v_days_total, 1);
  
  -- Prorated cost so far
  v_prorated_cost := (v_sub_cost / v_days_total) * v_days_used;
  
  -- Build result with task stats
  SELECT jsonb_build_object(
    'model_id', p_model_id,
    'model_name', m.name,
    'subscription_cost_usd', m.subscription_cost_usd,
    'subscription_started_at', m.subscription_started_at,
    'subscription_ends_at', m.subscription_ends_at,
    'subscription_status', m.subscription_status,
    'days_used', v_days_used,
    'days_total', v_days_total,
    'days_remaining', v_days_total - v_days_used,
    'prorated_cost_usd', ROUND(v_prorated_cost::numeric, 2),
    'tasks_completed', m.tasks_completed,
    'tasks_failed', m.tasks_failed,
    'tokens_used', m.tokens_used,
    'cost_per_task', CASE 
      WHEN m.tasks_completed > 0 
      THEN ROUND((v_prorated_cost / m.tasks_completed)::numeric, 4)
      ELSE 0 
    END,
    'success_rate', m.success_rate,
    'recommendation', CASE
      WHEN m.subscription_ends_at < NOW() THEN 'expired'
      WHEN m.subscription_ends_at < NOW() + INTERVAL '7 days' THEN 'renew_soon'
      WHEN m.tasks_completed > 0 AND (v_prorated_cost / m.tasks_completed) < (m.cost_input_per_1k_usd * 10) THEN 'good_value_renew'
      ELSE 'evaluate'
    END
  ) INTO v_result
  FROM models m
  WHERE m.id = p_model_id;
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- FUNCTION: Get all subscriptions summary
-- ============================================

CREATE OR REPLACE FUNCTION get_all_subscriptions()
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  SELECT jsonb_agg(
    jsonb_build_object(
      'model_id', id,
      'model_name', name,
      'subscription_cost_usd', subscription_cost_usd,
      'subscription_ends_at', subscription_ends_at,
      'subscription_status', subscription_status,
      'days_remaining', EXTRACT(DAY FROM subscription_ends_at - NOW())::INT,
      'tasks_completed', tasks_completed,
      'tokens_used', tokens_used
    )
  ) INTO v_result
  FROM models
  WHERE subscription_status = 'active'
  ORDER BY subscription_ends_at ASC;
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- FUNCTION: Get full ROI report (project + slices + subscriptions)
-- ============================================

CREATE OR REPLACE FUNCTION get_full_roi_report()
RETURNS JSONB AS $$
DECLARE
  v_projects JSONB;
  v_slices JSONB;
  v_subscriptions JSONB;
  v_totals JSONB;
BEGIN
  -- Get all projects ROI
  SELECT COALESCE(jsonb_agg(to_jsonb(roi_dashboard)), '[]'::jsonb) INTO v_projects
  FROM roi_dashboard;
  
  -- Get all slices ROI
  SELECT COALESCE(jsonb_agg(to_jsonb(slice_roi)), '[]'::jsonb) INTO v_slices
  FROM slice_roi;
  
  -- Get active subscriptions
  SELECT get_all_subscriptions() INTO v_subscriptions;
  
  -- Calculate grand totals
  SELECT jsonb_build_object(
    'total_tokens', COALESCE(SUM(total_tokens_used), 0),
    'total_theoretical_usd', ROUND(COALESCE(SUM(total_theoretical_cost), 0)::numeric, 2),
    'total_actual_usd', ROUND(COALESCE(SUM(total_actual_cost), 0)::numeric, 2),
    'total_savings_usd', ROUND(COALESCE(SUM(total_savings), 0)::numeric, 2),
    'total_tasks', COALESCE(SUM(total_tasks), 0),
    'total_completed', COALESCE(SUM(completed_tasks), 0)
  ) INTO v_totals
  FROM projects;
  
  RETURN jsonb_build_object(
    'generated_at', NOW(),
    'totals', v_totals,
    'projects', v_projects,
    'slices', v_slices,
    'subscriptions', v_subscriptions
  );
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- EXCHANGE RATE TABLE (for CAD conversion)
-- ============================================

CREATE TABLE IF NOT EXISTS exchange_rates (
  id TEXT PRIMARY KEY DEFAULT 'usd_cad',
  rate DECIMAL(10,6) NOT NULL,
  fetched_at TIMESTAMPTZ DEFAULT NOW(),
  source TEXT DEFAULT 'manual'
);

-- Seed default rate (will be updated by dashboard)
INSERT INTO exchange_rates (id, rate, source)
VALUES ('usd_cad', 1.36, 'seed')
ON CONFLICT (id) DO NOTHING;

-- Function to get current rate
CREATE OR REPLACE FUNCTION get_exchange_rate(p_from TEXT DEFAULT 'usd', p_to TEXT DEFAULT 'cad')
RETURNS DECIMAL(10,6) AS $$
DECLARE
  v_rate DECIMAL(10,6);
BEGIN
  SELECT rate INTO v_rate 
  FROM exchange_rates 
  WHERE id = lower(p_from || '_' || p_to)
  ORDER BY fetched_at DESC 
  LIMIT 1;
  
  RETURN COALESCE(v_rate, 1.36); -- fallback
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- VERIFICATION
-- ============================================

SELECT 'Migration v1.4 complete - Enhanced ROI tracking' AS status;

-- Show what was added
SELECT 
  'task_runs' as table_name,
  column_name,
  data_type
FROM information_schema.columns 
WHERE table_name = 'task_runs' 
  AND column_name IN ('tokens_in', 'tokens_out', 'courier_model_id', 'courier_tokens', 'courier_cost_usd', 'platform_theoretical_cost_usd', 'total_actual_cost_usd', 'total_savings_usd')
UNION ALL
SELECT 
  'models' as table_name,
  column_name,
  data_type
FROM information_schema.columns 
WHERE table_name = 'models' 
  AND column_name IN ('subscription_cost_usd', 'subscription_started_at', 'subscription_ends_at', 'subscription_status', 'cost_input_per_1k_usd', 'cost_output_per_1k_usd');
