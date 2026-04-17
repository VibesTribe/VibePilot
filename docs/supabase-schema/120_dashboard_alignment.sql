-- Migration 120: Comprehensive dashboard alignment
-- Fixes all gaps between Go governor and what the dashboard reads from Supabase.
-- Rerunnable: uses CREATE OR REPLACE + auto-drop for conflicting signatures.
--
-- Dashboard reads from:
--   tasks: id, title, status, priority, slice_id, phase, task_number,
--     routing_flag, routing_flag_reason, assigned_to, dependencies, result,
--     confidence, category, failure_notes, created_at, updated_at, started_at, completed_at
--   task_runs: tokens, costs, ROI data
--   models: learned JSONB (best_for_task_types, avoid_for_task_types, failure_rate_by_type)
--   orchestrator_events: event_type, task_id, model_id, reason, details, created_at
--     - event_type "failure" → dashboard marks task quality as "fail"
--     - reason → dashboard shows as reasonCode
--
-- See: docs/HOW_DASHBOARD_WORKS.md, dashboard/lib/vibepilotAdapter.ts

-- ============================================================
-- 1. claim_task: set assigned_to = model ID for dashboard agent display
-- ============================================================
CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_worker_id TEXT DEFAULT NULL,
  p_model_id TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT NULL,
  p_routing_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_model_id,
      processing_by = p_worker_id,
      processing_at = NOW(),
      routing_flag = COALESCE(p_routing_flag, routing_flag),
      routing_flag_reason = p_routing_reason,
      updated_at = NOW()
  WHERE id = p_task_id AND status = 'available' AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- 2. claim_for_review: accept tasks already in review status
-- Executor transitions to review after completion. Supervisor then claims.
-- Was checking status IN ('approved','merge_pending') - wrong.
-- ============================================================
DROP FUNCTION IF EXISTS claim_review(UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS claim_for_review(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET processing_by = p_reviewer_id, processing_at = NOW(), updated_at = NOW()
  WHERE id = p_task_id AND status = 'review' AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- 3. create_task_run: record execution results for dashboard ROI panel
-- Governor calls this after task execution completes.
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
    task_id, model_id, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd,
    total_actual_cost_usd, total_savings_usd,
    started_at, completed_at, result
  ) VALUES (
    p_task_id, p_model_id, p_platform, p_status,
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

-- ============================================================
-- 4. update_model_learning: build institutional model competency knowledge
-- Writes to models.learned JSONB:
--   best_for_task_types: where model has >80% success with 3+ attempts
--   avoid_for_task_types: where model has <40% success with 3+ attempts
--   failure_rate_by_type: {task_type: {attempts, successes, failures, rate}}
-- Router will use this for intelligent model selection.
-- ============================================================
DO $$
DECLARE
  sig RECORD;
BEGIN
  FOR sig IN
    SELECT p.oid::regprocedure AS signature
    FROM pg_proc p
    JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'update_model_learning'
  LOOP
    EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.signature;
  END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION update_model_learning(
  p_model_id TEXT,
  p_task_type TEXT,
  p_outcome TEXT DEFAULT 'success',
  p_failure_class TEXT DEFAULT NULL,
  p_failure_category TEXT DEFAULT NULL,
  p_failure_detail TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_learned JSONB;
  v_type_key TEXT;
  v_current_rate JSONB;
  v_new_rate NUMERIC;
  v_attempts INT;
  v_successes INT;
BEGIN
  SELECT learned INTO v_learned FROM models WHERE id = p_model_id;
  IF v_learned IS NULL THEN
    v_learned := '{}'::JSONB;
  END IF;

  IF v_learned->'failure_rate_by_type' IS NULL THEN
    v_learned := jsonb_set(v_learned, '{failure_rate_by_type}', '{}'::JSONB);
  END IF;

  v_type_key := COALESCE(p_task_type, 'unknown');

  v_current_rate := v_learned->'failure_rate_by_type'->v_type_key;
  IF v_current_rate IS NULL THEN
    v_current_rate := '{"attempts": 0, "successes": 0, "failures": 0, "rate": 0}'::JSONB;
  END IF;

  v_attempts := COALESCE((v_current_rate->>'attempts')::INT, 0) + 1;

  IF p_outcome = 'success' THEN
    v_successes := COALESCE((v_current_rate->>'successes')::INT, 0) + 1;
    v_new_rate := ROUND((v_successes::NUMERIC / v_attempts::NUMERIC) * 100, 1);

    v_learned := jsonb_set(v_learned,
      ARRAY['failure_rate_by_type', v_type_key],
      jsonb_build_object(
        'attempts', v_attempts,
        'successes', v_successes,
        'failures', COALESCE((v_current_rate->>'failures')::INT, 0),
        'rate', v_new_rate
      )
    );

    IF v_new_rate >= 80 AND v_attempts >= 3 THEN
      IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(v_learned->'best_for_task_types') elem WHERE elem = v_type_key) THEN
        IF v_learned->'best_for_task_types' IS NULL THEN
          v_learned := jsonb_set(v_learned, '{best_for_task_types}', jsonb_build_array(v_type_key));
        ELSE
          v_learned := jsonb_set(v_learned, '{best_for_task_types}', (v_learned->'best_for_task_types') || to_jsonb(v_type_key));
        END IF;
      END IF;
      IF v_learned->'avoid_for_task_types' IS NOT NULL THEN
        v_learned := jsonb_set(v_learned, '{avoid_for_task_types}',
          (SELECT jsonb_agg(elem) FROM jsonb_array_elements_text(v_learned->'avoid_for_task_types') elem WHERE elem != v_type_key)
        );
      END IF;
    END IF;
  ELSE
    v_successes := COALESCE((v_current_rate->>'successes')::INT, 0);
    v_new_rate := ROUND((v_successes::NUMERIC / v_attempts::NUMERIC) * 100, 1);

    v_learned := jsonb_set(v_learned,
      ARRAY['failure_rate_by_type', v_type_key],
      jsonb_build_object(
        'attempts', v_attempts,
        'successes', v_successes,
        'failures', COALESCE((v_current_rate->>'failures')::INT, 0) + 1,
        'rate', v_new_rate,
        'last_failure_class', p_failure_class,
        'last_failure_detail', p_failure_detail
      )
    );

    IF v_new_rate < 40 AND v_attempts >= 3 THEN
      IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(v_learned->'avoid_for_task_types') elem WHERE elem = v_type_key) THEN
        IF v_learned->'avoid_for_task_types' IS NULL THEN
          v_learned := jsonb_set(v_learned, '{avoid_for_task_types}', jsonb_build_array(v_type_key));
        ELSE
          v_learned := jsonb_set(v_learned, '{avoid_for_task_types}', (v_learned->'avoid_for_task_types') || to_jsonb(v_type_key));
        END IF;
      END IF;
      IF v_learned->'best_for_task_types' IS NOT NULL THEN
        v_learned := jsonb_set(v_learned, '{best_for_task_types}',
          (SELECT jsonb_agg(elem) FROM jsonb_array_elements_text(v_learned->'best_for_task_types') elem WHERE elem != v_type_key)
        );
      END IF;
    END IF;
  END IF;

  UPDATE models SET learned = v_learned, updated_at = NOW() WHERE id = p_model_id;
  RETURN FOUND;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
