-- VibePilot: Simple Reset (Safe Version)
-- Purpose: Clear processing claims only - uses only columns that exist in base schema
-- Run this when: Need to clear stuck processing claims

-- Clear all processing claims (these columns exist from migration 042)
UPDATE plans SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE tasks SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

-- Show current state
SELECT 'Plans with processing claims cleared' as action, COUNT(*) as remaining_with_claims FROM plans WHERE processing_by IS NOT NULL
UNION ALL
SELECT 'Tasks with processing claims cleared', COUNT(*) FROM tasks WHERE processing_by IS NOT NULL;

SELECT 'Simple reset complete' AS status;
