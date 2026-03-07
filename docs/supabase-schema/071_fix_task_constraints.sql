-- VibePilot Migration 071: Fix conflicting task type constraints
-- Purpose: Remove duplicate/conflicting type constraints and create one authoritative one
--
-- Problem: Two CHECK constraints exist for tasks.type:
--   1. task_type_check from schema_safety_patches.sql (bugfix, ui_ux, etc)
--   2. tasks_type_check from 067 (bug, fix, typecheck, etc)
-- These conflict and cause insert failures

-- Drop ALL existing type constraints
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS task_type_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_type_check;

-- Create single authoritative constraint matching what planner generates
ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN (
    'feature', 'bug', 'fix', 'test', 'refactor', 
    'lint', 'typecheck', 'visual', 'accessibility', 'docs', 'setup'
  ));

-- Also fix status constraint - ensure all needed statuses are included
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS no_self_dependency;

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'escalated', 'blocked'
  ));

-- Self-dependency check for JSONB dependencies column
-- Note: dependencies is JSONB, not UUID[], so we need jsonb operations
-- The Go code should prevent self-dependencies, but we add a trigger for safety

CREATE OR REPLACE FUNCTION check_no_self_dependency()
RETURNS TRIGGER AS $$
BEGIN
  -- Check if new.id exists in the new.dependencies JSONB array
  IF NEW.dependencies::jsonb ? NEW.id::text THEN
    RAISE EXCEPTION 'Task cannot depend on itself';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_no_self_dependency ON tasks;
CREATE TRIGGER trg_no_self_dependency
  BEFORE INSERT OR UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION check_no_self_dependency();

SELECT 'Migration 071 complete: task constraints unified, self-dep check via trigger' AS status;
