-- VibePilot Utility: Clear All Processing Claims
-- Purpose: Manually clear stuck processing claims for testing/recovery
-- Run this when: Plans/tasks stuck in processing state during testing

-- Clear all processing claims
UPDATE plans SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE tasks SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

-- Show what was cleared
SELECT 'plans' as table_name, count(*) as cleared FROM plans WHERE processing_by IS NULL
UNION ALL
SELECT 'tasks', count(*) FROM tasks WHERE processing_by IS NULL;
