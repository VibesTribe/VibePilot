-- 130: Add p_result parameter to update_courier_task_run
-- Previously this function only updated metadata (model_id, courier, platform, tokens).
-- Now it also writes the full execution result (files, summary, raw_output) to task_runs.result.
-- This is critical for supervisor review: handleTaskReview reads latestRun from task_runs.

CREATE OR REPLACE FUNCTION public.update_courier_task_run(
    p_task_id text,
    p_model_id text DEFAULT NULL,
    p_courier text DEFAULT NULL,
    p_platform text DEFAULT NULL,
    p_tokens_used integer DEFAULT 0,
    p_result jsonb DEFAULT NULL
) RETURNS void LANGUAGE plpgsql AS $function$
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
            tokens_used    = CASE WHEN p_tokens_used > 0 THEN p_tokens_used ELSE tokens_used END,
            result         = COALESCE(p_result, result)
        WHERE id = v_run_id;
    END IF;
END;
$function$;
