-- Migration 134: Fix ROI calculations
-- 1. get_subscription_roi: add api_equivalent_cost_usd (tokens * input rate / 1000)
-- 2. get_all_subscriptions: add api_equivalent_cost_usd, fix ROUND cast
-- 3. Restore GLM-5 historical subscription cost ($45) and start date

-- Fix get_subscription_roi with api_equivalent_cost_usd
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
  v_tokens BIGINT;
  v_input_cost_per_1k DECIMAL(10,6);
  v_output_cost_per_1k DECIMAL(10,6);
  v_api_equivalent DECIMAL(10,6);
BEGIN
  SELECT 
    subscription_cost_usd, 
    subscription_started_at, 
    subscription_ends_at,
    tokens_used,
    cost_input_per_1k_usd,
    cost_output_per_1k_usd
  INTO v_sub_cost, v_sub_start, v_sub_end, v_tokens, v_input_cost_per_1k, v_output_cost_per_1k
  FROM models WHERE id = p_model_id;
  
  v_days_used := EXTRACT(DAY FROM NOW() - COALESCE(v_sub_start, NOW()));
  v_days_total := EXTRACT(DAY FROM COALESCE(v_sub_end, NOW()) - COALESCE(v_sub_start, NOW()));
  v_days_total := GREATEST(v_days_total, 1);
  v_prorated_cost := (v_sub_cost / v_days_total) * v_days_used;
  
  -- API equivalent: what these tokens would cost at pay-per-use rates
  v_api_equivalent := (COALESCE(v_tokens, 0) * COALESCE(v_input_cost_per_1k, 0)) / 1000.0;
  
  SELECT jsonb_build_object(
    'model_id', p_model_id,
    'model_name', m.name,
    'subscription_cost_usd', m.subscription_cost_usd,
    'api_equivalent_cost_usd', ROUND(v_api_equivalent::numeric, 4),
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
    'roi_percentage', CASE
      WHEN v_sub_cost > 0 AND v_api_equivalent > 0
      THEN ROUND(((v_api_equivalent - v_sub_cost) / v_sub_cost * 100)::numeric, 1)
      WHEN v_sub_cost = 0 AND v_api_equivalent > 0
      THEN 9999
      ELSE 0
    END,
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

-- Fix get_all_subscriptions with api_equivalent_cost_usd
CREATE OR REPLACE FUNCTION get_all_subscriptions()
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  SELECT jsonb_agg(sub) INTO v_result
  FROM (
    SELECT jsonb_build_object(
      'model_id', id,
      'model_name', name,
      'subscription_cost_usd', subscription_cost_usd,
      'api_equivalent_cost_usd', ROUND((COALESCE(tokens_used, 0) * COALESCE(cost_input_per_1k_usd, 0) / 1000.0)::numeric, 4),
      'subscription_ends_at', subscription_ends_at,
      'subscription_status', subscription_status,
      'days_remaining', EXTRACT(DAY FROM subscription_ends_at - NOW())::INT,
      'tasks_completed', tasks_completed,
      'tokens_used', tokens_used
    ) AS sub
    FROM models
    WHERE subscription_status = 'active'
    ORDER BY subscription_ends_at ASC
  ) agg;
  
  RETURN COALESCE(v_result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql;

-- Restore GLM-5 historical subscription cost
UPDATE models SET
  subscription_cost_usd = 45,
  subscription_started_at = '2026-02-01T00:00:00-05:00'
WHERE id = 'glm-5';
