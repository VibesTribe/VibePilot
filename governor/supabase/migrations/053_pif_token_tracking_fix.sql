-- ============================================================================
-- Migration 053: PIF — Fix token tracking chain (create_task_run + ROI)
-- ============================================================================
-- Fixes two issues:
-- 1. create_task_run didn't set project_id on task_runs rows, and called the
--    wrong increment_lifetime_counters overload (no project_id)
-- 2. update_project_cumulative only read from tasks table (which has 0 tokens
--    for tasks completed before the SSD restore), causing ROI to show 0 tokens
-- ============================================================================

DROP FUNCTION IF EXISTS create_task_run(uuid, text, text, text, text, integer, integer, integer, text, integer, numeric, numeric, numeric, numeric, timestamp with time zone, timestamp with time zone, jsonb);

CREATE FUNCTION create_task_run(
    p_task_id uuid,
    p_model_id text,
    p_courier text,
    p_platform text,
    p_status text,
    p_tokens_in integer DEFAULT 0,
    p_tokens_out integer DEFAULT 0,
    p_tokens_used integer DEFAULT 0,
    p_courier_model_id text DEFAULT NULL,
    p_courier_tokens integer DEFAULT 0,
    p_courier_cost_usd numeric DEFAULT 0,
    p_platform_theoretical_cost_usd numeric DEFAULT 0,
    p_total_actual_cost_usd numeric DEFAULT 0,
    p_total_savings_usd numeric DEFAULT 0,
    p_started_at timestamp with time zone DEFAULT NULL,
    p_completed_at timestamp with time zone DEFAULT NULL,
    p_result jsonb DEFAULT NULL
) RETURNS uuid AS $$
DECLARE
    v_run_id uuid;
    v_project_id uuid;
BEGIN
    -- Resolve project_id from the task
    SELECT project_id INTO v_project_id FROM tasks WHERE id = p_task_id;

    INSERT INTO task_runs (
        task_id, model_id, courier, platform, status,
        tokens_in, tokens_out, tokens_used,
        courier_model_id, courier_tokens, courier_cost_usd,
        platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
        started_at, completed_at, result, project_id
    ) VALUES (
        p_task_id, p_model_id, p_courier, p_platform, p_status,
        p_tokens_in, p_tokens_out, p_tokens_used,
        p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
        p_platform_theoretical_cost_usd, p_total_actual_cost_usd, p_total_savings_usd,
        p_started_at, COALESCE(p_completed_at, NOW()), p_result, v_project_id::text
    )
    RETURNING id INTO v_run_id;

    -- Increment per-project counters (3-arg overload sets project_id)
    PERFORM increment_lifetime_counters(
        p_tokens_used + p_courier_tokens,
        p_total_actual_cost_usd,
        v_project_id
    );

    RETURN v_run_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================

CREATE OR REPLACE FUNCTION update_project_cumulative(p_project_id uuid)
RETURNS jsonb AS $$
DECLARE
    v_total_tasks int;
    v_completed_tasks int;
    v_task_tokens bigint;
    v_counter_tokens bigint;
    v_total_tokens bigint;
    v_theoretical_cost double precision;
    v_actual_cost double precision;
    v_project_cost double precision;
BEGIN
    -- Get task-derived stats
    SELECT
        count(*),
        count(*) FILTER (WHERE status IN ('complete', 'merged', 'merge_pending')),
        COALESCE(SUM(total_tokens_in + total_tokens_out), 0),
        COALESCE(SUM(total_api_cost_usd), 0),
        COALESCE(SUM(total_cost_usd), 0)
    INTO v_total_tasks, v_completed_tasks, v_task_tokens, v_theoretical_cost, v_actual_cost
    FROM tasks
    WHERE project_id = p_project_id;

    -- Get system_counters tokens (from gateway/courier tracking)
    SELECT COALESCE(total_tokens, 0) INTO v_counter_tokens
    FROM system_counters WHERE project_id = p_project_id AND id = 'global';

    -- Use whichever is higher (task_runs may be empty for restored DBs)
    v_total_tokens := GREATEST(v_task_tokens, v_counter_tokens);

    -- Get manual project_costs
    SELECT COALESCE(sum(amount_usd), 0) INTO v_project_cost
    FROM project_costs WHERE project_id = p_project_id;

    -- Use the higher of task-derived cost or manual project costs
    v_actual_cost := GREATEST(v_actual_cost, v_project_cost);

    UPDATE projects SET
        total_tasks = v_total_tasks,
        completed_tasks = v_completed_tasks,
        total_tokens_used = v_total_tokens,
        total_theoretical_cost = v_theoretical_cost,
        total_actual_cost = v_actual_cost,
        total_savings = v_theoretical_cost - v_actual_cost,
        updated_at = now()
    WHERE id = p_project_id;

    RETURN jsonb_build_object(
        'project_id', p_project_id,
        'total_tasks', v_total_tasks,
        'completed_tasks', v_completed_tasks,
        'total_tokens', v_total_tokens,
        'theoretical_cost', v_theoretical_cost,
        'actual_cost', v_actual_cost,
        'savings', v_theoretical_cost - v_actual_cost
    );
END;
$$ LANGUAGE plpgsql;

-- Sync existing data
UPDATE projects SET
    total_tokens_used = (SELECT COALESCE(total_tokens, 0) FROM system_counters WHERE project_id = p1.id AND id = 'global'),
    total_actual_cost = (SELECT COALESCE(sum(amount_usd), 0) FROM project_costs WHERE project_id = p1.id),
    updated_at = NOW()
FROM projects p1
WHERE projects.id = p1.id;

DO $$ BEGIN
    RAISE NOTICE 'Migration 053 complete: token tracking chain fixed.';
END $$;
