-- Migration 122: Complete pipeline alignment
-- Fixes ALL mismatches found in cross-reference audit.
-- Rerunnable: auto-drops conflicting signatures before recreating.
--
-- BUGS FIXED:
-- 1. claim_for_review: matched status IN ('approved','merge_pending') but Go sets 'review'
--    → supervisor NEVER ran → tasks stuck at review forever
-- 2. unlock_dependent_tasks: referenced 'depends_on' (doesn't exist, real: 'dependencies')
--    and checked status='blocked' (real: 'pending') → deps never unlocked
-- 3. claim_task: no dependency guard → T002 claimed before T001 complete
-- 4. transition_task: no attempts increment → infinite retries, no escalation
-- 5. transition_task: no auto-unlock on completion → deps never resolved

-- ============================================================
-- 1. claim_for_review: match correct status
-- ============================================================
DO $$
DECLARE sig RECORD;
BEGIN
  FOR sig IN SELECT p.oid::regprocedure AS s FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'claim_for_review'
  LOOP EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.s; END LOOP;
END; $$;

CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE v_updated INT;
BEGIN
  UPDATE tasks
  SET processing_by = p_reviewer_id, processing_at = NOW(), updated_at = NOW()
  WHERE id = p_task_id
    AND status = 'review'
    AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- 2. unlock_dependent_tasks: correct column + status + task_number matching
-- ============================================================
DO $$
DECLARE sig RECORD;
BEGIN
  FOR sig IN SELECT p.oid::regprocedure AS s FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'unlock_dependent_tasks'
  LOOP EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.s; END LOOP;
END; $$;

CREATE OR REPLACE FUNCTION unlock_dependent_tasks(
  p_completed_task_id UUID
)
RETURNS INT AS $$
DECLARE
  v_unlocked INT;
  v_task_number TEXT;
BEGIN
  SELECT task_number INTO v_task_number FROM tasks WHERE id = p_completed_task_id;
  IF v_task_number IS NULL THEN RETURN 0; END IF;

  UPDATE tasks
  SET status = 'available', updated_at = NOW()
  WHERE status = 'pending'
    AND dependencies @> to_jsonb(ARRAY[v_task_number])
    AND NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements_text(dependencies) AS dep
      WHERE dep != v_task_number
        AND NOT EXISTS (
          SELECT 1 FROM tasks t2 WHERE t2.task_number = dep AND t2.status IN ('complete', 'merged')
        )
    );
  GET DIAGNOSTICS v_unlocked = ROW_COUNT;
  RETURN v_unlocked;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================
-- 3. claim_task: dependency guard
-- ============================================================
DO $$
DECLARE sig RECORD;
BEGIN
  FOR sig IN SELECT p.oid::regprocedure AS s FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'claim_task'
  LOOP EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.s; END LOOP;
END; $$;

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
  v_deps JSONB;
  v_unmet INT;
BEGIN
  -- Reject if dependencies not met
  SELECT dependencies INTO v_deps FROM tasks WHERE id = p_task_id;
  IF v_deps IS NOT NULL AND jsonb_array_length(v_deps) > 0 THEN
    SELECT COUNT(*) INTO v_unmet
    FROM jsonb_array_elements_text(v_deps) AS dep
    WHERE NOT EXISTS (
      SELECT 1 FROM tasks t WHERE t.task_number = dep AND t.status IN ('complete', 'merged')
    );
    IF v_unmet > 0 THEN RETURN FALSE; END IF;
  END IF;

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
-- 4. transition_task: attempts increment + auto-unlock dependents
-- ============================================================
DO $$
DECLARE sig RECORD;
BEGIN
  FOR sig IN SELECT p.oid::regprocedure AS s FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid
    WHERE n.nspname = 'public' AND p.proname = 'transition_task'
  LOOP EXECUTE 'DROP FUNCTION IF EXISTS ' || sig.s; END LOOP;
END; $$;

CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_failure_reason TEXT DEFAULT NULL,
  p_result JSONB DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
  v_old_status TEXT;
BEGIN
  SELECT status INTO v_old_status FROM tasks WHERE id = p_task_id;

  UPDATE tasks
  SET status = p_new_status,
      result = COALESCE(p_result, result),
      processing_by = NULL,
      processing_at = NULL,
      failure_notes = CASE
        WHEN p_failure_reason IS NOT NULL THEN
          COALESCE(failure_notes || E'\n', '') || p_failure_reason || ' at ' || NOW()::text
        ELSE failure_notes
      END,
      attempts = CASE
        WHEN p_new_status IN ('available', 'pending') AND v_old_status NOT IN ('available', 'pending') THEN
          COALESCE(attempts, 0) + 1
        ELSE COALESCE(attempts, 0)
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

  -- Auto-unlock dependents on completion
  IF v_updated > 0 AND p_new_status IN ('complete', 'merged') THEN
    PERFORM unlock_dependent_tasks(p_task_id);
  END IF;

  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
