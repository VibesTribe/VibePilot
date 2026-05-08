-- Migration 128: Fix courier result RPCs
-- BUG 2 fix: Consolidate record_courier_result to single text overload that always
-- calls increment_lifetime_counters (previously text overload skipped it).
-- Also adds update_courier_task_run for BUG 3 (avoid duplicate task_run rows).

-- Drop both old overloads
DROP FUNCTION IF EXISTS public.record_courier_result(text, text, text, text, integer, integer);
DROP FUNCTION IF EXISTS public.record_courier_result(text, text, jsonb, text, integer, integer);

-- Single overload: accepts text (what Go's pgx sends), casts to jsonb, always increments counters
CREATE OR REPLACE FUNCTION public.record_courier_result(
    p_task_id text,
    p_status text,
    p_result text DEFAULT NULL,
    p_error text DEFAULT NULL,
    p_tokens_in integer DEFAULT 0,
    p_tokens_out integer DEFAULT 0
)
RETURNS void
LANGUAGE plpgsql
AS $function$
DECLARE
    v_run_id UUID;
BEGIN
    SELECT id INTO v_run_id FROM task_runs
    WHERE task_id::text = p_task_id
    ORDER BY started_at DESC LIMIT 1;

    IF v_run_id IS NULL THEN
        INSERT INTO task_runs (task_id, status, result, error, tokens_in, tokens_out, started_at, completed_at)
        VALUES (p_task_id::uuid, p_status, p_result::jsonb, p_error, p_tokens_in, p_tokens_out, now(), now());
    ELSE
        UPDATE task_runs SET
            status = p_status,
            result = p_result::jsonb,
            error = p_error,
            tokens_in = p_tokens_in,
            tokens_out = p_tokens_out,
            completed_at = now()
        WHERE id = v_run_id;
    END IF;

    PERFORM increment_lifetime_counters(p_tokens_in + p_tokens_out, 0);
END;
$function$;

-- New RPC for updating courier task_run metadata (avoids duplicate inserts)
CREATE OR REPLACE FUNCTION public.update_courier_task_run(
    p_task_id text,
    p_model_id text DEFAULT NULL,
    p_courier text DEFAULT NULL,
    p_platform text DEFAULT NULL,
    p_tokens_used integer DEFAULT 0
)
RETURNS void
LANGUAGE plpgsql
AS $function$
DECLARE
    v_run_id UUID;
BEGIN
    SELECT id INTO v_run_id FROM task_runs
    WHERE task_id::text = p_task_id
    ORDER BY started_at DESC LIMIT 1;

    IF v_run_id IS NOT NULL THEN
        UPDATE task_runs SET
            model_id       = COALESCE(p_model_id, model_id),
            courier        = COALESCE(p_courier, courier),
            platform       = COALESCE(p_platform, platform),
            tokens_used    = CASE WHEN p_tokens_used > 0 THEN p_tokens_used ELSE tokens_used END
        WHERE id = v_run_id;
    END IF;
END;
$function$;
