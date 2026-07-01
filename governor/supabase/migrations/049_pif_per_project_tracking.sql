-- ============================================================================
-- Migration 049: PIF Phase C completion — per-project cost/token tracking
-- ============================================================================
-- Fixes 6 gaps that prevented per-project ROI tracking:
-- 1. record_internal_run / record_courier_result now populate task_runs.project_id
-- 2. increment_lifetime_counters now creates per-project counter rows
-- 3. project_snapshots gets project_id
-- 4. agent_sessions gets project_id
-- 5. get_all_projects_roi() SQL bug fixed
-- 6. update_project_cumulative() rolls up task costs into projects table
-- ============================================================================

BEGIN;

-- ========================================
-- 1. Fix record_internal_run: populate project_id from tasks table
-- ========================================
CREATE OR REPLACE FUNCTION record_internal_run(
    p_task_id uuid,
    p_model_id text,
    p_role text DEFAULT 'executor',
    p_status text DEFAULT 'success',
    p_tokens_in integer DEFAULT 0,
    p_tokens_out integer DEFAULT 0,
    p_token_source text DEFAULT 'exact'
) RETURNS uuid
LANGUAGE plpgsql
AS $func$
DECLARE
    v_run_id UUID;
    v_cost_result JSONB;
    v_project_id uuid;
BEGIN
    -- Get project_id from the task
    SELECT project_id INTO v_project_id FROM tasks WHERE id = p_task_id;

    INSERT INTO task_runs (
        task_id, courier, platform, model_id, status,
        tokens_in, tokens_out, role, token_source,
        project_id,
        started_at, completed_at
    ) VALUES (
        p_task_id, 'internal', 'internal', p_model_id, p_status,
        p_tokens_in, p_tokens_out, p_role, p_token_source,
        v_project_id,
        now(), now()
    ) RETURNING id INTO v_run_id;

    SELECT * INTO v_cost_result FROM calc_run_costs(
        p_model_id, p_tokens_in, p_tokens_out, 0
    );

    UPDATE task_runs SET
        platform_theoretical_cost_usd = COALESCE((v_cost_result->>'theoretical_cost_usd')::DOUBLE PRECISION, 0),
        total_actual_cost_usd = 0,
        courier_cost_usd = 0,
        total_savings_usd = COALESCE((v_cost_result->>'savings_usd')::DOUBLE PRECISION, 0)
    WHERE id = v_run_id;

    PERFORM increment_lifetime_counters(p_tokens_in + p_tokens_out, 0, v_project_id);

    RETURN v_run_id;
END;
$func$;

-- ========================================
-- 2. Fix record_courier_result: populate project_id
-- ========================================
CREATE OR REPLACE FUNCTION record_courier_result(
    p_task_id text,
    p_status text,
    p_result text DEFAULT NULL,
    p_error text DEFAULT NULL,
    p_tokens_in integer DEFAULT 0,
    p_tokens_out integer DEFAULT 0,
    p_model_id text DEFAULT NULL
) RETURNS void
LANGUAGE plpgsql
AS $func$
DECLARE
    v_run_id UUID;
    v_model_id TEXT;
    v_project_id uuid;
BEGIN
    -- Get project_id from the task
    SELECT project_id INTO v_project_id FROM tasks WHERE id::text = p_task_id;

    v_model_id := COALESCE(p_model_id, (
        SELECT model_id FROM task_runs
        WHERE task_id::text = p_task_id
        ORDER BY started_at DESC LIMIT 1
    ));

    SELECT id INTO v_run_id FROM task_runs
    WHERE task_id::text = p_task_id
    ORDER BY started_at DESC LIMIT 1;

    IF v_run_id IS NULL THEN
        INSERT INTO task_runs (task_id, status, result, error,
          tokens_in, tokens_out, model_id, project_id, started_at, completed_at)
        VALUES (p_task_id::uuid, p_status, p_result::jsonb, p_error,
          p_tokens_in, p_tokens_out, v_model_id, v_project_id, now(), now());
    ELSE
        UPDATE task_runs SET
          status = p_status,
          result = p_result::jsonb,
          error = p_error,
          tokens_in = p_tokens_in,
          tokens_out = p_tokens_out,
          model_id = COALESCE(v_model_id, model_id),
          project_id = COALESCE(v_project_id, project_id),
          completed_at = now()
        WHERE id = v_run_id;
    END IF;

    PERFORM increment_lifetime_counters(p_tokens_in + p_tokens_out, 0, v_project_id);
END;
$func$;

-- ========================================
-- 3. Fix increment_lifetime_counters: scope by project_id
-- ========================================
CREATE OR REPLACE FUNCTION increment_lifetime_counters(
    p_tokens integer,
    p_cost_usd numeric,
    p_project_id uuid DEFAULT NULL
) RETURNS void
LANGUAGE plpgsql
AS $func$
DECLARE
    v_proj_id uuid;
BEGIN
    -- Default to vibepilot if not provided
    IF p_project_id IS NULL THEN
        SELECT id INTO v_proj_id FROM projects WHERE slug = 'vibepilot';
    ELSE
        v_proj_id := p_project_id;
    END IF;

    -- Global counter (existing behavior, for backward compat)
    INSERT INTO system_counters (id, total_tokens, total_cost_usd, total_runs, updated_at)
    VALUES ('global', GREATEST(p_tokens, 0), COALESCE(p_cost_usd, 0), 1, NOW())
    ON CONFLICT (id) DO UPDATE SET
      total_tokens = system_counters.total_tokens + GREATEST(p_tokens, 0),
      total_cost_usd = system_counters.total_cost_usd + COALESCE(p_cost_usd, 0),
      total_runs = system_counters.total_runs + 1,
      updated_at = NOW();

    -- Per-project counter
    INSERT INTO system_counters (id, project_id, total_tokens, total_cost_usd, total_runs, updated_at)
    VALUES ('project:' || v_proj_id::text, v_proj_id, GREATEST(p_tokens, 0), COALESCE(p_cost_usd, 0), 1, NOW())
    ON CONFLICT (id) DO UPDATE SET
      total_tokens = system_counters.total_tokens + GREATEST(p_tokens, 0),
      total_cost_usd = system_counters.total_cost_usd + COALESCE(p_cost_usd, 0),
      total_runs = system_counters.total_runs + 1,
      updated_at = NOW();
END;
$func$;

COMMIT;

-- ========================================
-- 4. project_snapshots: add project_id
-- ========================================
BEGIN;
ALTER TABLE project_snapshots ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE project_snapshots SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE project_snapshots ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_project_snapshots_project_id ON project_snapshots(project_id);
COMMIT;

-- ========================================
-- 5. agent_sessions: add project_id
-- ========================================
BEGIN;
ALTER TABLE agent_sessions ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE agent_sessions SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE agent_sessions ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_agent_sessions_project_id ON agent_sessions(project_id);
COMMIT;

-- ========================================
-- 6. Fix get_all_projects_roi() — broken SQL
-- ========================================
CREATE OR REPLACE FUNCTION get_all_projects_roi()
RETURNS jsonb
LANGUAGE plpgsql
AS $func$
BEGIN
    RETURN (
        SELECT COALESCE(jsonb_agg(row_to_json(r)), '[]'::jsonb)
        FROM (
            SELECT
                id,
                slug,
                COALESCE(display_name, slug) as project_name,
                status,
                COALESCE(completed_tasks, 0) as completed_tasks,
                COALESCE(total_tasks, 0) as total_tasks,
                COALESCE(total_tokens_used, 0) as total_tokens_used,
                COALESCE(total_actual_cost, 0) as total_actual_cost,
                COALESCE(total_theoretical_cost, 0) as total_theoretical_cost,
                COALESCE(total_savings, 0) as total_savings,
                CASE WHEN COALESCE(total_tasks, 0) > 0
                     THEN ROUND(((COALESCE(completed_tasks, 0)::FLOAT / total_tasks) * 100)::numeric, 1)
                     ELSE 0 END as completion_rate
            FROM projects
            ORDER BY created_at DESC
        ) r
    );
END;
$func$;

-- ========================================
-- 7. New RPC: update_project_cumulative — roll up task costs into projects table
--    Called after every task completion to keep the projects table current.
-- ========================================
CREATE OR REPLACE FUNCTION update_project_cumulative(p_project_id uuid)
RETURNS jsonb
LANGUAGE plpgsql
AS $func$
DECLARE
    v_total_tasks int;
    v_completed_tasks int;
    v_total_tokens bigint;
    v_theoretical_cost double precision;
    v_actual_cost double precision;
BEGIN
    SELECT
        count(*),
        count(*) FILTER (WHERE status IN ('complete', 'merged', 'merge_pending')),
        COALESCE(SUM(total_tokens_in + total_tokens_out), 0),
        COALESCE(SUM(total_api_cost_usd), 0),
        COALESCE(SUM(total_cost_usd), 0)
    INTO v_total_tasks, v_completed_tasks, v_total_tokens, v_theoretical_cost, v_actual_cost
    FROM tasks
    WHERE project_id = p_project_id;

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
$func$;

-- ========================================
-- 8. Fix get_project_roi to use correct column names
-- ========================================
CREATE OR REPLACE FUNCTION get_project_roi(p_project_id uuid)
RETURNS jsonb
LANGUAGE plpgsql
AS $func$
DECLARE
    v_project record;
BEGIN
    SELECT * INTO v_project FROM projects WHERE id = p_project_id;
    IF NOT FOUND THEN
        RETURN jsonb_build_object('error', 'project not found');
    END IF;

    RETURN jsonb_build_object(
        'project_id', v_project.id,
        'slug', v_project.slug,
        'project_name', COALESCE(v_project.display_name, v_project.slug),
        'status', v_project.status,
        'total_tasks', COALESCE(v_project.total_tasks, 0),
        'completed_tasks', COALESCE(v_project.completed_tasks, 0),
        'completion_rate', CASE WHEN COALESCE(v_project.total_tasks, 0) > 0
                                THEN ROUND(((COALESCE(v_project.completed_tasks, 0)::FLOAT / v_project.total_tasks) * 100)::numeric, 1)
                                ELSE 0 END,
        'total_tokens', COALESCE(v_project.total_tokens_used, 0),
        'theoretical_cost', COALESCE(v_project.total_theoretical_cost, 0),
        'actual_cost', COALESCE(v_project.total_actual_cost, 0),
        'savings', COALESCE(v_project.total_savings, 0),
        'roi_percentage', CASE WHEN COALESCE(v_project.total_theoretical_cost, 0) > 0
                               THEN ROUND(((COALESCE(v_project.total_savings, 0) / v_project.total_theoretical_cost) * 100)::numeric, 1)
                               ELSE 0 END
    );
END;
$func$;
