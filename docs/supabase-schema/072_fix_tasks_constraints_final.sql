-- VibePilot Migration 072: Debug and fix tasks constraints
-- Run the SELECT first to see what constraints exist, then the fixes

-- STEP 1: See what constraints actually exist
SELECT conname, pg_get_constraintdef(oid) 
FROM pg_constraint 
WHERE conrelid = 'public.tasks'::regclass 
ORDER BY conname;

-- STEP 2: Drop specific constraints by name (if they exist from old migrations)
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_priority_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_type_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS task_type_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_confidence_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_max_attempts_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_no_self_dependency;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS no_self_dependency;

-- STEP 3: Recreate only what we need
ALTER TABLE tasks ADD CONSTRAINT tasks_priority_check 
  CHECK (priority BETWEEN 1 AND 10);

ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility','docs','setup','bugfix','ui_ux','api','infrastructure'));

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN ('pending','available','in_progress','review','testing','approval','merged','escalated','blocked'));

ALTER TABLE tasks ADD CONSTRAINT tasks_confidence_check 
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1));

ALTER TABLE tasks ADD CONSTRAINT tasks_max_attempts_check 
  CHECK (max_attempts > 0 AND max_attempts <= 10);

-- Self-dependency via trigger (JSONB compatible)
DROP TRIGGER IF EXISTS trg_no_self_dependency ON tasks;

CREATE OR REPLACE FUNCTION check_no_self_dependency()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.dependencies IS NOT NULL 
     AND jsonb_typeof(NEW.dependencies) = 'array'
     AND jsonb_array_length(NEW.dependencies) > 0
     AND NEW.dependencies ? NEW.id::text THEN
    RAISE EXCEPTION 'Task cannot depend on itself';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_no_self_dependency
  BEFORE INSERT OR UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION check_no_self_dependency();

SELECT 'Migration 072 complete' AS status;
