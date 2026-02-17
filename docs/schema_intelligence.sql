-- Intelligence Storage Schema for VibePilot
-- Tracks model performance for subscription advisor and routing optimization
-- Version: 1.0
-- Created: 2026-02-17

-- Model performance tracking (extends existing task_runs)
-- No new table needed - add columns to task_runs

ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS model_capability_tested TEXT;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS context_limit_observed INT;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS confusion_detected BOOLEAN DEFAULT FALSE;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS confusion_detected_at_tokens INT;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS task_type VARCHAR(50);
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS quality_score DECIMAL(3,2); -- 0.00 to 1.00

-- Model intelligence summary view
CREATE OR REPLACE VIEW model_intelligence AS
SELECT 
    model_id,
    COUNT(*) as total_runs,
    COUNT(*) FILTER (WHERE status = 'success') as successful_runs,
    ROUND(
        COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / 
        NULLIF(COUNT(*), 0) * 100, 2
    ) as success_rate_pct,
    AVG(duration_seconds) as avg_duration_seconds,
    SUM(tokens_in) as total_tokens_in,
    SUM(tokens_out) as total_tokens_out,
    
    -- Task type breakdown
    MODE() WITHIN GROUP (ORDER BY task_type) as most_common_task_type,
    COUNT(DISTINCT task_type) as task_types_handled,
    
    -- Context limit observations
    AVG(context_limit_observed) as avg_context_before_confusion,
    MAX(context_limit_observed) as max_safe_context,
    
    -- Quality metrics
    AVG(quality_score) as avg_quality_score,
    
    -- Cost efficiency (theoretical vs actual)
    SUM(platform_theoretical_cost_usd) as total_theoretical_cost,
    SUM(total_actual_cost_usd) as total_actual_cost,
    SUM(total_savings_usd) as total_savings,
    
    -- Last used
    MAX(created_at) as last_used_at,
    
    -- Recommendation flags
    CASE 
        WHEN COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / NULLIF(COUNT(*), 0) >= 0.85 
        THEN 'recommend_subscription'
        WHEN COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / NULLIF(COUNT(*), 0) < 0.50 
        THEN 'avoid'
        ELSE 'neutral'
    END as recommendation

FROM task_runs
WHERE model_id IS NOT NULL
GROUP BY model_id;

-- Platform intelligence view (for web platforms)
CREATE OR REPLACE VIEW platform_intelligence AS
SELECT 
    platform,
    COUNT(*) as total_runs,
    COUNT(*) FILTER (WHERE status = 'success') as successful_runs,
    ROUND(
        COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / 
        NULLIF(COUNT(*), 0) * 100, 2
    ) as success_rate_pct,
    AVG(duration_seconds) as avg_duration_seconds,
    
    -- Rate limit encounters
    COUNT(*) FILTER (WHERE error_code = 'RATE_LIMITED') as rate_limit_hits,
    
    -- Auth issues
    COUNT(*) FILTER (WHERE error_code LIKE '%auth%' OR error_code LIKE '%login%') as auth_failures,
    
    -- Task type fit
    MODE() WITHIN GROUP (ORDER BY task_type) as best_for_task_type,
    
    -- Last successful run
    MAX(created_at) FILTER (WHERE status = 'success') as last_successful_at,
    
    -- Recommendation
    CASE 
        WHEN COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / NULLIF(COUNT(*), 0) >= 0.90 
        THEN 'primary'
        WHEN COUNT(*) FILTER (WHERE status = 'success')::DECIMAL / NULLIF(COUNT(*), 0) >= 0.70 
        THEN 'secondary'
        ELSE 'backup'
    END as tier_recommendation

FROM task_runs
WHERE platform IS NOT NULL AND platform != ''
GROUP BY platform;

-- Intelligence report function (for weekly reports)
CREATE OR REPLACE FUNCTION generate_intelligence_report(
    report_days INT DEFAULT 7
)
RETURNS TABLE (
    report_type TEXT,
    item_id TEXT,
    metric_name TEXT,
    metric_value DECIMAL,
    recommendation TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Model recommendations
    RETURN QUERY
    SELECT 
        'model'::TEXT,
        mi.model_id::TEXT,
        'success_rate'::TEXT,
        mi.success_rate_pct,
        mi.recommendation::TEXT
    FROM model_intelligence mi
    WHERE mi.total_runs >= 10;  -- Need at least 10 samples
    
    -- Platform recommendations
    RETURN QUERY
    SELECT 
        'platform'::TEXT,
        pi.platform::TEXT,
        'success_rate'::TEXT,
        pi.success_rate_pct,
        pi.tier_recommendation::TEXT
    FROM platform_intelligence pi
    WHERE pi.total_runs >= 5;  -- Need at least 5 samples
    
    -- Context limit warnings
    RETURN QUERY
    SELECT 
        'context_warning'::TEXT,
        tr.model_id::TEXT,
        'context_at_confusion'::TEXT,
        tr.confusion_detected_at_tokens::DECIMAL,
        'Reduce context limit for this model'::TEXT
    FROM task_runs tr
    WHERE tr.confusion_detected = TRUE
    AND tr.created_at > NOW() - (report_days || ' days')::INTERVAL;
END;
$$;

-- Weekly intelligence summary (call this for Vibes briefings)
CREATE OR REPLACE FUNCTION get_weekly_intelligence_summary()
RETURNS JSON
LANGUAGE plpgsql
AS $$
DECLARE
    result JSON;
BEGIN
    SELECT json_build_object(
        'report_date', NOW(),
        'total_runs_week', (SELECT COUNT(*) FROM task_runs 
            WHERE created_at > NOW() - INTERVAL '7 days'),
        'top_performing_models', (
            SELECT json_agg(row_to_json(mi))
            FROM (
                SELECT model_id, success_rate_pct, total_runs
                FROM model_intelligence
                WHERE total_runs >= 5
                ORDER BY success_rate_pct DESC
                LIMIT 3
            ) mi
        ),
        'struggling_models', (
            SELECT json_agg(row_to_json(mi))
            FROM (
                SELECT model_id, success_rate_pct, total_runs
                FROM model_intelligence
                WHERE total_runs >= 5 AND success_rate_pct < 70
                ORDER BY success_rate_pct ASC
                LIMIT 3
            ) mi
        ),
        'platform_tiers', (
            SELECT json_agg(row_to_json(pi))
            FROM (
                SELECT platform, success_rate_pct, tier_recommendation
                FROM platform_intelligence
                ORDER BY success_rate_pct DESC
            ) pi
        ),
        'subscription_recommendations', (
            SELECT json_agg(json_build_object(
                'model_id', model_id,
                'recommendation', recommendation,
                'runs', total_runs,
                'savings', total_savings
            ))
            FROM model_intelligence
            WHERE recommendation = 'recommend_subscription'
        ),
        'credit_status', (
            SELECT json_build_object(
                'deepseek_remaining', 
                (SELECT credit_remaining_usd FROM models WHERE id = 'deepseek-api' LIMIT 1),
                'openrouter_remaining',
                (SELECT credit_remaining_usd FROM models WHERE id = 'openrouter-api' LIMIT 1)
            )
        )
    ) INTO result;
    
    RETURN result;
END;
$$;

-- Example usage:
-- SELECT * FROM generate_intelligence_report(7);
-- SELECT get_weekly_intelligence_summary();
