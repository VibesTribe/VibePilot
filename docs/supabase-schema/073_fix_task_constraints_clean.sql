-- VibePilot Migration 073: Fix any remaining task constraint issues
-- Purpose: Ensure all constraints are correct and there are no conflicts

-- First, drop ALL check constraints on tasks to start fresh
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint 
              WHERE conrelid = 'public.tasks'::regclass 
              AND contype = 'c')
    LOOP
        EXECUTE 'ALTER TABLE public.tasks DROP CONSTRAINT IF EXISTS ' || r.conname;
    END LOOP;
END $$;

-- Now recreate only the constraints we actually want

-- Priority must be 1-10
ALTER TABLE tasks ADD CONSTRAINT tasks_priority_check 
  CHECK (priority BETWEEN 1 AND 10);

-- Valid task types (matching what planner generates)
ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN (
    'feature', 'bug', 'fix', 'test', 'refactor', 
    'lint', 'typecheck', 'visual', 'accessibility', 'docs', 'setup',
    'bugfix', 'ui_ux', 'api', 'infrastructure'
  ));

-- Valid task statuses
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'escalated', 'blocked'
  ));

-- Confidence must be between 0 and 1 IF NOT NULL (allow null for backward compatibility)
ALTER TABLE tasks ADD CONSTRAINT tasks_confidence_check 
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1));

-- Max attempts must be positive
ALTER TABLE tasks ADD CONSTRAINT tasks_max_attempts_check 
  CHECK (max_attempts > 0 AND max_attempts <= 10);

SELECT 'Migration 073 complete: all task constraints recreated cleanly' AS status;
