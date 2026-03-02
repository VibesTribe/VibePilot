-- VibePilot Migration 052: Full System Reset Utility
-- Purpose: Clear processing claims and reset error states
-- Run this when: System has stuck plans/tasks from testing

-- ============================================================================
-- PART 1: Clear all processing claims
-- ============================================================================

UPDATE plans SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE tasks SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

-- ============================================================================
-- PART 2: Reset error states to draft
-- ============================================================================

UPDATE plans 
SET status = 'draft', 
    revision_round = 0,
    latest_feedback = NULL
WHERE status = 'error';

-- ============================================================================
-- PART 3: Show current state
-- ============================================================================

SELECT 
  'plans' as table_name,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE status = 'draft') as draft,
  COUNT(*) FILTER (WHERE status = 'error') as error,
  COUNT(*) FILTER (WHERE processing_by IS NOT NULL) as processing
FROM plans
UNION ALL
SELECT 
  'tasks' as table_name,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE status = 'pending') as pending,
  COUNT(*) FILTER (WHERE status = 'available') as available,
  COUNT(*) FILTER (WHERE processing_by IS NOT NULL) as processing
FROM tasks;

SELECT 'Migration 052 complete - Full reset utility ready' AS status;
