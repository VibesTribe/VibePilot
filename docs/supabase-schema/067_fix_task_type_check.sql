-- VibePilot Migration 067: Fix Task Type Check Constraint
-- Purpose: Ensure task type values match what planner generates
-- 
-- Valid types:
--   - feature: New functionality
--   - bug: Bug fix
--   - fix: General fix
--   - test: Testing-related
--   - refactor: Code refactoring
--   - lint: Lint fixes
--   - typecheck: Type fixes
--   - visual: Visual/UI changes
--   - accessibility: Accessibility improvements

-- Drop existing constraint if it exists
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_type_check;

-- Add new constraint with valid types
ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility'));

-- Verify migration
SELECT 'Migration 067 complete. Valid task types:' AS status;
SELECT unnest(ARRAY['feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility']) AS valid_type;
