
-- Migration 124: Model orchestrator RPCs
-- Run entire script in Supabase SQL Editor

-- Drop old functions with wrong signatures
DROP FUNCTION IF EXISTS check_platform_availability(TEXT);
DROP FUNCTION IF EXISTS get_model_score_for_task(TEXT, TEXT, TEXT);
DROP FUNCTION IF EXISTS update_model_usage(TEXT, JSONB, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER, TEXT, TEXT, INTEGER, JSONB);

CREATE OR REPLACE FUNCTION check_platform_availability(p_platform_id TEXT)
RETURNS JSONB
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_model RECORD;
    v_available BOOLEAN := true;
    v_reason TEXT := '';
    v_cooldown_remaining INTEGER := 0;
BEGIN
    SELECT id, status, status_reason, cooldown_expires_at, credit_remaining_usd, credit_alert_threshold
    INTO v_model
    FROM models WHERE id = p_platform_id;

    IF NOT FOUND THEN
        RETURN jsonb_build_object('available', true, 'reason', 'not_registered');
    END IF;

    IF v_model.status = 'paused' THEN
        v_available := false;
        v_reason := v_model.status_reason;
    ELSIF v_model.status != 'active' THEN
        v_available := false;
        v_reason := 'status_' || v_model.status;
    END IF;

    IF v_model.cooldown_expires_at IS NOT NULL AND v_model.cooldown_expires_at > NOW() THEN
        v_available := false;
        v_reason := 'cooldown';
        v_cooldown_remaining := EXTRACT(EPOCH FROM (v_model.cooldown_expires_at - NOW()))::INTEGER;
    END IF;

    IF v_model.credit_remaining_usd IS NOT NULL
       AND v_model.credit_alert_threshold IS NOT NULL
       AND v_model.credit_remaining_usd < v_model.credit_alert_threshold THEN
        v_available := false;
        v_reason := 'credit_low';
    END IF;

    RETURN jsonb_build_object(
        'available', v_available,
        'reason', v_reason,
        'cooldown_remaining_seconds', v_cooldown_remaining
    );
END;
$$;

CREATE OR REPLACE FUNCTION get_model_score_for_task(
    p_model_id TEXT,
    p_task_type TEXT DEFAULT NULL,
    p_task_category TEXT DEFAULT NULL
)
RETURNS JSONB
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_model RECORD;
    v_score NUMERIC := 0.5;
    v_success_rate NUMERIC;
    v_tasks_completed INTEGER;
    v_tasks_failed INTEGER;
BEGIN
    SELECT id, status, success_rate, tasks_completed, tasks_failed, learned
    INTO v_model
    FROM models WHERE id = p_model_id;

    IF NOT FOUND THEN
        RETURN jsonb_build_object('score', 0.0, 'reason', 'not_registered');
    END IF;

    IF v_model.status != 'active' THEN
        RETURN jsonb_build_object('score', 0.0, 'reason', 'status_' || v_model.status);
    END IF;

    v_success_rate := COALESCE(v_model.success_rate, 0.5);
    v_tasks_completed := COALESCE(v_model.tasks_completed, 0);
    v_tasks_failed := COALESCE(v_model.tasks_failed, 0);

    v_score := v_success_rate;

    IF v_tasks_completed + v_tasks_failed > 5 THEN
        v_score := v_score * 0.8 + 0.2;
    END IF;

    IF v_model.learned IS NOT NULL THEN
        IF p_task_type IS NOT NULL THEN
            IF v_model.learned->'best_for_task_types' ? p_task_type THEN
                v_score := v_score + 0.15;
            END IF;
            IF v_model.learned->'avoid_for_task_types' ? p_task_type THEN
                v_score := v_score - 0.2;
            END IF;
        END IF;
    END IF;

    IF v_score < 0 THEN v_score := 0; END IF;
    IF v_score > 1.0 THEN v_score := 1.0; END IF;

    RETURN jsonb_build_object('score', v_score);
END;
$$;

CREATE OR REPLACE FUNCTION update_model_usage(
    p_model_id TEXT,
    p_usage_windows JSONB DEFAULT NULL,
    p_cooldown_expires_at TIMESTAMPTZ DEFAULT NULL,
    p_last_rate_limit_at TIMESTAMPTZ DEFAULT NULL,
    p_rate_limit_count INTEGER DEFAULT NULL,
    p_status TEXT DEFAULT NULL,
    p_status_reason TEXT DEFAULT NULL,
    p_tokens_used INTEGER DEFAULT NULL,
    p_learned JSONB DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    UPDATE models SET
        usage_windows = COALESCE(p_usage_windows, usage_windows),
        cooldown_expires_at = COALESCE(p_cooldown_expires_at, cooldown_expires_at),
        last_rate_limit_at = COALESCE(p_last_rate_limit_at, last_rate_limit_at),
        rate_limit_count = COALESCE(p_rate_limit_count, rate_limit_count),
        status = COALESCE(p_status, status),
        status_reason = COALESCE(p_status_reason, status_reason),
        tokens_used = COALESCE(p_tokens_used, tokens_used),
        learned = COALESCE(p_learned, learned),
        updated_at = NOW()
    WHERE id = p_model_id;

    RETURN FOUND;
END;
$$;

GRANT EXECUTE ON FUNCTION check_platform_availability(TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION get_model_score_for_task(TEXT, TEXT, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION update_model_usage(TEXT, JSONB, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER, TEXT, TEXT, INTEGER, JSONB) TO anon, authenticated, service_role;
