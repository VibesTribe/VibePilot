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

-- Re-add self-dependency check (was dropped with other constraints)
ALTER TABLE tasks ADD CONSTRAINT no_self_dependency 
  CHECK (NOT (id = ANY(dependencies)));

SELECT 'Migration 071 complete: task constraints unified' AS status;
