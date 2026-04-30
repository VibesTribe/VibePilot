-- Migration 121: Fix transition_task param mismatch + task_runs.courier nullable
-- CRITICAL: Go sends p_failure_reason TEXT in ~12 call sites but Supabase only
-- accepts p_result JSONB. This causes ALL failure transitions to silently fail.
-- Also: task_runs.courier is NOT NULL but dashboard expects string | null.
-- Rerunnable: auto-drops conflicting signatures before recreating.
--
-- Bug trace: handlers_task.go, handlers_maint.go, handlers_testing.go, recovery.go
-- all send {"p_task_id": ..., "p_new_status": ..., "p_failure_reason": "..."}

-- ============================================================
-- 1. transition_task: accept BOTH p_failure_reason TEXT and p_result JSONB
-- ============================================================
DO $$
DECLARE
  sig RECORD;
BEGIN
  FOR sig IN
    SELECT p.oid::regprocedure AS signature
    FROM pg_proc p
    JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'transition_task'
  LOOP
    EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.signature;
  END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_failure_reason TEXT DEFAULT NULL,
  p_result JSONB DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = p_new_status,
      result = COALESCE(p_result, result),
      processing_by = NULL,
      processing_at = NULL,
      failure_notes = CASE
        WHEN p_failure_reason IS NOT NULL THEN
          COALESCE(failure_notes || E'\n', '') ||
          p_failure_reason || ' at ' || NOW()::text
        ELSE failure_notes
      END,
      started_at = CASE
        WHEN p_new_status = 'in_progress' AND started_at IS NULL THEN NOW()
        ELSE started_at
      END,
      completed_at = CASE
        WHEN p_new_status IN ('complete', 'merged', 'cancelled') THEN NOW()
        ELSE completed_at
      END,
      updated_at = NOW()
  WHERE id = p_task_id;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- 2. task_runs.courier: make nullable (dashboard expects string | null)
-- ============================================================
ALTER TABLE task_runs ALTER COLUMN courier DROP NOT NULL;

-- ============================================================
-- 3. create_task_run: include courier in INSERT
-- ============================================================
DO $$
DECLARE
  sig RECORD;
BEGIN
  FOR sig IN
    SELECT p.oid::regprocedure AS signature
    FROM pg_proc p
    JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'create_task_run'
  LOOP
    EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.signature;
  END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION create_task_run(
  p_task_id UUID,
  p_model_id TEXT DEFAULT NULL,
  p_courier TEXT DEFAULT NULL,
  p_platform TEXT DEFAULT NULL,
  p_status TEXT DEFAULT 'success',
  p_tokens_in INTEGER DEFAULT 0,
  p_tokens_out INTEGER DEFAULT 0,
  p_tokens_used INTEGER DEFAULT 0,
  p_courier_model_id TEXT DEFAULT NULL,
  p_courier_tokens INTEGER DEFAULT 0,
  p_courier_cost_usd DECIMAL DEFAULT 0,
  p_platform_theoretical_cost_usd DECIMAL DEFAULT 0,
  p_total_actual_cost_usd DECIMAL DEFAULT 0,
  p_total_savings_usd DECIMAL DEFAULT 0,
  p_started_at TIMESTAMPTZ DEFAULT NOW(),
  p_completed_at TIMESTAMPTZ DEFAULT NOW(),
  p_result JSONB DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (
    task_id, model_id, courier, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd,
    total_actual_cost_usd, total_savings_usd,
    started_at, completed_at, result
  ) VALUES (
    p_task_id, p_model_id, p_courier, p_platform, p_status,
    p_tokens_in, p_tokens_out, p_tokens_used,
    p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
    p_platform_theoretical_cost_usd,
    p_total_actual_cost_usd, p_total_savings_usd,
    p_started_at, p_completed_at, p_result
  )
  RETURNING id INTO v_run_id;
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
