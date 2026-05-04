-- Migration 137: Include chat usage costs in ROI report

CREATE OR REPLACE FUNCTION get_full_roi_report()
RETURNS jsonb
LANGUAGE plpgsql
AS $function$
DECLARE
  v_projects JSONB;
  v_slices JSONB;
  v_subscriptions JSONB;
  v_totals JSONB;
  v_chat_costs JSONB;
BEGIN
  -- Get all projects ROI
  SELECT COALESCE(jsonb_agg(to_jsonb(roi_dashboard)), '[]'::jsonb) INTO v_projects
  FROM roi_dashboard;
  
  -- Get all slices ROI
  SELECT COALESCE(jsonb_agg(to_jsonb(slice_roi)), '[]'::jsonb) INTO v_slices
  FROM slice_roi;
  
  -- Get active subscriptions
  SELECT get_all_subscriptions() INTO v_subscriptions;
  
  -- Calculate grand totals from projects
  SELECT jsonb_build_object(
    'total_tokens', COALESCE(SUM(total_tokens_used), 0),
    'total_theoretical_usd', ROUND(COALESCE(SUM(total_theoretical_cost), 0)::numeric, 2),
    'total_actual_usd', ROUND(COALESCE(SUM(total_actual_cost), 0)::numeric, 2),
    'total_savings_usd', ROUND(COALESCE(SUM(total_savings), 0)::numeric, 2),
    'total_tasks', COALESCE(SUM(total_tasks), 0),
    'total_completed', COALESCE(SUM(completed_tasks), 0)
  ) INTO v_totals
  FROM projects;
  
  -- Get chat usage summary
  SELECT COALESCE(jsonb_agg(row_to_json(summary)), '[]'::jsonb) INTO v_chat_costs
  FROM (
    SELECT 
      model_id,
      count(*) as total_chats,
      sum(tokens_in) as total_tokens_in,
      sum(tokens_out) as total_tokens_out,
      round(sum(theoretical_cost_usd)::numeric, 6) as total_cost_usd,
      min(created_at) as first_chat,
      max(created_at) as last_chat
    FROM chat_usage
    GROUP BY model_id
    ORDER BY sum(theoretical_cost_usd) DESC
  ) summary;
  
  RETURN jsonb_build_object(
    'generated_at', NOW(),
    'totals', v_totals,
    'projects', v_projects,
    'slices', v_slices,
    'subscriptions', v_subscriptions,
    'chat_usage_summary', v_chat_costs
  );
END;
$function$;
