-- Migration 047: Add model_id to record_courier_result
-- Phase 2 of Model Intelligence System Recovery
-- Date: 2026-06-18
-- Backward compatible: old couriers that don't send model_id still work

CREATE OR REPLACE FUNCTION record_courier_result(
    p_task_id text,
    p_status text,
    p_result text DEFAULT NULL,
    p_error text DEFAULT NULL,
    p_tokens_in integer DEFAULT 0,
    p_tokens_out integer DEFAULT 0,
    p_model_id text DEFAULT NULL  -- NEW
) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
  v_run_id UUID;
  v_model_id TEXT;
BEGIN
  -- Use provided model_id, or look up from existing row if not provided
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
      tokens_in, tokens_out, model_id, started_at, completed_at)
    VALUES (p_task_id::uuid, p_status, p_result::jsonb, p_error,
      p_tokens_in, p_tokens_out, v_model_id, now(), now());
  ELSE
    UPDATE task_runs SET
      status = p_status,
      result = p_result::jsonb,
      error = p_error,
      tokens_in = p_tokens_in,
      tokens_out = p_tokens_out,
      model_id = COALESCE(v_model_id, model_id),  -- don't overwrite with NULL
      completed_at = now()
    WHERE id = v_run_id;
  END IF;

  PERFORM increment_lifetime_counters(p_tokens_in + p_tokens_out, 0);
END;
$$;
