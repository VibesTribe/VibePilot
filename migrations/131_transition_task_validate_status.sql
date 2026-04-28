-- Migration 131: Add status validation to transition_task RPC
--
-- Problem: transition_task accepted any string for p_new_status.
-- Typos or invented statuses got written to the database silently.
--
-- Fix: Validate against the 10 dashboard-defined statuses before UPDATE.
-- Raise exception on invalid status to surface errors in Go code.
-- The CHECK constraint on tasks.status catches it at the DB level too,
-- but this gives a clear error message and prevents the attempt entirely.

CREATE OR REPLACE FUNCTION public.transition_task(
  p_task_id uuid,
  p_new_status text,
  p_result jsonb DEFAULT NULL::jsonb,
  p_failure_reason text DEFAULT NULL::text
)
RETURNS boolean
LANGUAGE plpgsql
AS $function$
DECLARE
  v_updated INT;
  v_old_status TEXT;
  v_valid_statuses TEXT[] := ARRAY[
    'pending', 'in_progress', 'received', 'review', 'testing',
    'complete', 'merge_pending', 'merged', 'failed', 'human_review'
  ];
BEGIN
  -- Validate status value against dashboard-defined set
  IF NOT p_new_status = ANY(v_valid_statuses) THEN
    RAISE EXCEPTION 'Invalid task status "%". Valid: %', p_new_status, array_to_string(v_valid_statuses, ', ');
  END IF;

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
        WHEN p_new_status = 'pending' AND v_old_status != 'pending' THEN
          COALESCE(attempts, 0) + 1
        ELSE COALESCE(attempts, 0)
      END,
      started_at = CASE WHEN p_new_status = 'in_progress' AND started_at IS NULL THEN NOW() ELSE started_at END,
      completed_at = CASE WHEN p_new_status IN ('complete', 'merged') THEN NOW() ELSE completed_at END,
      updated_at = NOW()
  WHERE id = p_task_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;

  IF v_updated > 0 AND p_new_status IN ('complete', 'merged') THEN
    PERFORM unlock_dependent_tasks(p_task_id);
  END IF;

  RETURN v_updated > 0;
END;
$function$;
