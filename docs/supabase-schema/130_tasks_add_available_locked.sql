-- Migration 130: Add 'available' and 'locked' to tasks status CHECK constraint
-- These statuses are needed for dependency management:
--   'available' = task with no unmet dependencies, ready to be claimed
--   'locked' = task with unmet dependencies, waiting for deps to complete
-- Also fix unlock_dependent_tasks to work with the new statuses

-- 1. Update tasks CHECK constraint to include 'available' and 'locked'
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check CHECK (
  status = ANY (ARRAY[
    'pending'::text,
    'available'::text,
    'locked'::text,
    'in_progress'::text,
    'received'::text,
    'review'::text,
    'testing'::text,
    'complete'::text,
    'merge_pending'::text,
    'merged'::text,
    'failed'::text,
    'human_review'::text
  ])
);

-- 2. Fix unlock_dependent_tasks — drop and recreate with correct logic
-- Was broken: looked for status='locked' (not in old constraint), set status='available' (not in old constraint)
DROP FUNCTION IF EXISTS unlock_dependent_tasks(UUID) CASCADE;

CREATE OR REPLACE FUNCTION unlock_dependent_tasks(p_completed_task_id UUID)
RETURNS SETOF VOID AS $$
DECLARE
  unlocked_id UUID;
BEGIN
  FOR unlocked_id IN
    SELECT t.id
    FROM tasks t
    WHERE t.status IN ('locked', 'pending')
    AND t.dependencies ? p_completed_task_id::TEXT
    AND (
      SELECT COUNT(*) = 0
      FROM jsonb_array_elements_text(t.dependencies) AS dep_id
      JOIN tasks dep ON dep.id::TEXT = dep_id
      WHERE dep.status NOT IN ('complete', 'merged')
    )
  LOOP
    UPDATE tasks
    SET status = 'available',
        updated_at = NOW()
    WHERE id = unlocked_id;
  END LOOP;
  RETURN;
END;
$$ LANGUAGE plpgsql;

-- 3. Fix unlock_dependents (alternate name) — drop and recreate
DROP FUNCTION IF EXISTS unlock_dependents(UUID) CASCADE;

CREATE OR REPLACE FUNCTION unlock_dependents(p_completed_task_id UUID)
RETURNS SETOF VOID AS $$
DECLARE
  unlocked_id UUID;
BEGIN
  FOR unlocked_id IN
    SELECT t.id
    FROM tasks t
    WHERE t.status IN ('locked', 'pending')
    AND t.dependencies ? p_completed_task_id::TEXT
    AND (
      SELECT COUNT(*) = 0
      FROM jsonb_array_elements_text(t.dependencies) AS dep_id
      JOIN tasks dep ON dep.id::TEXT = dep_id
      WHERE dep.status NOT IN ('complete', 'merged')
    )
  LOOP
    UPDATE tasks
    SET status = 'available',
        updated_at = NOW()
    WHERE id = unlocked_id;
  END LOOP;
  RETURN;
END;
$$ LANGUAGE plpgsql;
