-- Migration 132: Cost tracking overhaul - subscription history, project snapshots, task cost aggregation
-- Part of the cost tracking & ROI overhaul plan

-- 1. subscription_history: permanent record of subscriptions, archivable
CREATE TABLE IF NOT EXISTS subscription_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id TEXT NOT NULL,
    provider TEXT,
    cost_usd DOUBLE PRECISION NOT NULL,
    period_type TEXT NOT NULL DEFAULT 'monthly', -- 'monthly', 'quarterly', 'annual', 'one_time'
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    tokens_consumed BIGINT DEFAULT 0,
    tasks_completed INT DEFAULT 0,
    api_equivalent_cost_usd DOUBLE PRECISION DEFAULT 0,
    roi_percentage DOUBLE PRECISION DEFAULT 0,
    archived_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. project_snapshots: frozen snapshots for archival + comparison
CREATE TABLE IF NOT EXISTS project_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    label TEXT NOT NULL,
    total_tokens BIGINT DEFAULT 0,
    total_cost_usd DOUBLE PRECISION DEFAULT 0,
    total_api_equivalent_usd DOUBLE PRECISION DEFAULT 0,
    total_tasks INT DEFAULT 0,
    total_commits INT DEFAULT 0,
    model_breakdown JSONB DEFAULT '{}',
    subscription_breakdown JSONB DEFAULT '[]',
    total_spent_usd DOUBLE PRECISION DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 3. Add role column to task_runs (planner, executor, supervisor, tester, analyst)
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'executor';
COMMENT ON COLUMN task_runs.role IS 'Role of this model invocation: planner, executor, supervisor, tester, analyst, courier';

-- 4. Add token_source column to task_runs
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS token_source TEXT DEFAULT 'exact';
COMMENT ON COLUMN task_runs.token_source IS 'How tokens were determined: exact (from API response) or estimated (from char count)';

-- 5. Add aggregated cost columns to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS total_tokens_in BIGINT DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS total_tokens_out BIGINT DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS total_cost_usd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS total_api_cost_usd DOUBLE PRECISION DEFAULT 0;
COMMENT ON COLUMN tasks.total_cost_usd IS 'Actual cost (courier cost or $0 for internal)';
COMMENT ON COLUMN tasks.total_api_cost_usd IS 'What all tokens would cost at API rates';

-- 6. Fix GLM-5 subscription cost (was $30, actually $45 for 3 months)
UPDATE models SET subscription_cost_usd = 45 WHERE id = 'glm-5' AND subscription_cost_usd = 30;

-- 7. Insert historical GLM-5 subscription into history
INSERT INTO subscription_history (model_id, provider, cost_usd, period_type, started_at, ended_at, tokens_consumed, tasks_completed, api_equivalent_cost_usd, roi_percentage, notes)
VALUES (
    'glm-5',
    'zhipu',
    45.0,
    'quarterly',
    '2026-02-01 00:00:00-05',
    '2026-04-30 20:00:00-04',
    786646115,
    487,
    1439.56,
    3099.0,
    'Z.AI Pro subscription, $15/mo. Grandfathered pricing expiring Apr 30. New price $200/mo. 786.6M tokens via subscription vs $1,439.56 at API rates.'
);

-- 8. RPC: aggregate_task_costs - sums all task_runs for a task and updates tasks table
CREATE OR REPLACE FUNCTION aggregate_task_costs(p_task_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_tokens_in BIGINT;
    v_tokens_out BIGINT;
    v_actual_cost DOUBLE PRECISION;
    v_api_cost DOUBLE PRECISION;
BEGIN
    SELECT 
        COALESCE(SUM(tokens_in), 0),
        COALESCE(SUM(tokens_out), 0),
        COALESCE(SUM(total_actual_cost_usd), 0),
        COALESCE(SUM(platform_theoretical_cost_usd), 0)
    INTO v_tokens_in, v_tokens_out, v_actual_cost, v_api_cost
    FROM task_runs
    WHERE task_id = p_task_id;

    UPDATE tasks SET
        total_tokens_in = v_tokens_in,
        total_tokens_out = v_tokens_out,
        total_cost_usd = v_actual_cost,
        total_api_cost_usd = v_api_cost
    WHERE id = p_task_id;

    RETURN jsonb_build_object(
        'task_id', p_task_id,
        'tokens_in', v_tokens_in,
        'tokens_out', v_tokens_out,
        'actual_cost_usd', v_actual_cost,
        'api_cost_usd', v_api_cost
    );
END;
$$ LANGUAGE plpgsql;

-- 9. RPC: check_subscription_thresholds - returns models approaching limits
CREATE OR REPLACE FUNCTION check_subscription_thresholds()
RETURNS TABLE (
    model_id TEXT,
    alert_type TEXT,
    current_value DOUBLE PRECISION,
    threshold_value DOUBLE PRECISION,
    message TEXT
) AS $$
BEGIN
    -- Credit remaining < 20% (assuming original was credit_remaining_usd when > 0, or use credit_alert_threshold)
    RETURN QUERY
    SELECT 
        m.id,
        'credit_low'::TEXT,
        m.credit_remaining_usd::DOUBLE PRECISION,
        m.credit_alert_threshold::DOUBLE PRECISION,
        format('Model %s has $%.2f credit remaining (threshold: $%.2f)', m.id, m.credit_remaining_usd, m.credit_alert_threshold)::TEXT
    FROM models m
    WHERE m.credit_remaining_usd IS NOT NULL
      AND m.credit_alert_threshold IS NOT NULL
      AND m.credit_remaining_usd > 0
      AND m.credit_remaining_usd <= m.credit_alert_threshold;

    -- Subscription ending within 20% of period
    RETURN QUERY
    SELECT 
        m.id,
        'subscription_expiring'::TEXT,
        EXTRACT(DAY FROM (m.subscription_ends_at - NOW()))::DOUBLE PRECISION,
        (EXTRACT(DAY FROM (m.subscription_ends_at - m.subscription_started_at)) * 0.2)::DOUBLE PRECISION,
        format('Model %s subscription ends %s (%.0f days remaining)', m.id, m.subscription_ends_at::date, EXTRACT(DAY FROM (m.subscription_ends_at - NOW())))::TEXT
    FROM models m
    WHERE m.subscription_status = 'active'
      AND m.subscription_ends_at IS NOT NULL
      AND m.subscription_ends_at > NOW()
      AND (m.subscription_ends_at - NOW()) < (m.subscription_ends_at - m.subscription_started_at) * 0.2;

    RETURN;
END;
$$ LANGUAGE plpgsql;

-- 10. RPC: create_project_snapshot - freezes current state
CREATE OR REPLACE FUNCTION create_project_snapshot(p_label TEXT)
RETURNS UUID AS $$
DECLARE
    v_snapshot_id UUID;
    v_total_tokens BIGINT;
    v_total_api_cost DOUBLE PRECISION;
    v_total_actual_cost DOUBLE PRECISION;
    v_total_tasks INT;
    v_model_breakdown JSONB;
    v_sub_breakdown JSONB;
BEGIN
    -- Aggregate from task_runs
    SELECT 
        COALESCE(SUM(tokens_in + tokens_out), 0),
        COALESCE(SUM(platform_theoretical_cost_usd), 0),
        COALESCE(SUM(total_actual_cost_usd), 0),
        COUNT(DISTINCT task_id)
    INTO v_total_tokens, v_total_api_cost, v_total_actual_cost, v_total_tasks
    FROM task_runs;

    -- Per-model breakdown
    SELECT jsonb_object_agg(model_id, jsonb_build_object(
        'tokens_in', SUM(tokens_in),
        'tokens_out', SUM(tokens_out),
        'runs', COUNT(*),
        'api_cost', SUM(platform_theoretical_cost_usd),
        'actual_cost', SUM(total_actual_cost_usd)
    ))
    INTO v_model_breakdown
    FROM task_runs GROUP BY model_id;

    -- Active subscriptions
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'model_id', id,
        'cost', subscription_cost_usd,
        'ends', subscription_ends_at,
        'tokens_used', tokens_used,
        'tasks', tasks_completed
    )), '[]'::jsonb)
    INTO v_sub_breakdown
    FROM models
    WHERE subscription_status = 'active';

    INSERT INTO project_snapshots (label, total_tokens, total_cost_usd, total_api_equivalent_usd, total_tasks, model_breakdown, subscription_breakdown)
    VALUES (p_label, v_total_tokens, v_total_actual_cost, v_total_api_cost, v_total_tasks, COALESCE(v_model_breakdown, '{}'), v_sub_breakdown)
    RETURNING id INTO v_snapshot_id;

    RETURN v_snapshot_id;
END;
$$ LANGUAGE plpgsql;
