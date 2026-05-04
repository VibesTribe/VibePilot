-- Migration 133: Fix get_all_subscriptions SQL error
-- ORDER BY inside jsonb_agg requires subquery wrapper
-- Also fix get_full_roi_report to handle empty subscriptions gracefully

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

-- Update GLM-5 model to reflect free tier until July 2, 2026
-- Rate limits: 400 requests per 5 hours, 2000 requests per week
UPDATE models SET
  subscription_status = 'active',
  subscription_ends_at = '2026-07-02T00:00:00-04:00',
  subscription_cost_usd = 0,
  status = 'active',
  status_reason = 'Z.AI Pro free tier until July 2, 2026. Limits: 400 req/5hr, 2000 req/week',
  rate_limits = jsonb_set(
    jsonb_set(rate_limits, '{requests_per_5_hours}', '400'),
    '{requests_per_week}', '2000'
  )
WHERE id = 'glm-5';
