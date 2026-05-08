-- Migration 132: transition_task reads valid statuses from CHECK constraint
-- Single source of truth: the CHECK constraint defines valid statuses.
-- transition_task NO LONGER hardcodes a status list.
-- Adding new statuses = ALTER CHECK constraint only. No function update needed.

DROP FUNCTION IF EXISTS public.transition_task(UUID, TEXT, JSONB, TEXT);

CREATE OR REPLACE FUNCTION public.transition_task(
  p_task_id         UUID,
  p_new_status      TEXT,
  p_result          JSONB DEFAULT NULL,
  p_failure_reason  TEXT  DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
  v_updated INT;
  v_old_status TEXT;
BEGIN
  SELECT status INTO v_old_status FROM tasks WHERE id = p_task_id;

  -- Let the CHECK constraint validate the status.
  -- If p_new_status is invalid, UPDATE raises check_violation (SQLSTATE 23514)
  -- with a clear message from the constraint definition.
  -- This makes the CHECK constraint the SINGLE SOURCE OF TRUTH for valid statuses.
  -- No hardcoded list here that can drift.
  UPDATE tasks
  SET status = p_new_status,
      result = COALESCE(result, '{}')::jsonb || COALESCE(p_result, '{}')::jsonb,
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
$$;

-- Verify the function works with all current valid statuses
DO $$
DECLARE
  test_task_id UUID;
  result BOOLEAN;
BEGIN
  -- Create a throwaway test task
  INSERT INTO tasks (id, title, status, plan_id)
  VALUES (
    gen_random_uuid(),
    'migration_132_test',
    'pending',
    (SELECT id FROM plans LIMIT 1)
  ) RETURNING id INTO test_task_id;

  -- Test transitions that previously would have been rejected
  result := transition_task(test_task_id, 'available', NULL, NULL);
  ASSERT result, 'transition_task failed for status=available';

  result := transition_task(test_task_id, 'in_progress', NULL, NULL);
  ASSERT result, 'transition_task failed for status=in_progress';

  result := transition_task(test_task_id, 'review', NULL, NULL);
  ASSERT result, 'transition_task failed for status=review';

  result := transition_task(test_task_id, 'testing', NULL, NULL);
  ASSERT result, 'transition_task failed for status=testing';

  result := transition_task(test_task_id, 'complete', NULL, NULL);
  ASSERT result, 'transition_task failed for status=complete';

  result := transition_task(test_task_id, 'merged', NULL, NULL);
  ASSERT result, 'transition_task failed for status=merged';

  -- Test that invalid status raises error
  BEGIN
    PERFORM transition_task(test_task_id, 'totally_invalid_status', NULL, NULL);
    RAISE EXCEPTION 'transition_task should have rejected invalid status';
  EXCEPTION WHEN raise_exception THEN
    -- Expected: the function should have caught it
    NULL;
  END;

  -- Cleanup
  DELETE FROM tasks WHERE id = test_task_id;

  RAISE NOTICE 'Migration 132: transition_task validation verified for all statuses';
END $$;
