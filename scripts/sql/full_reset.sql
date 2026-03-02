-- VibePilot Utility: Full System Reset for Testing
-- Purpose: Clear all processing claims, error states, and prepare for clean test
-- Run this before each major test session

-- ============================================================================
-- PART 1: Clear all processing claims
-- ============================================================================

UPDATE plans SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE tasks SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

-- ============================================================================
-- PART 2: Reset error states to draft (recoverable)
-- ============================================================================

UPDATE plans 
SET status = 'draft', 
    revision_round = 0,
    review_notes = NULL
WHERE status = 'error';

-- ============================================================================
-- PART 3: Reset stuck tasks
-- ============================================================================

UPDATE tasks 
SET status = 'pending',
    retry_count = 0,
    last_error = NULL,
    last_error_at = NULL
WHERE status IN ('error', 'blocked')
  AND retry_count < 3;

-- ============================================================================
-- PART 4: Show current state
-- ============================================================================

SELECT 'Plans Summary' as category, status, count(*) as count
FROM plans
GROUP BY status
UNION ALL
SELECT 'Tasks Summary', status, count(*)
FROM tasks
GROUP BY status
ORDER BY category, status;
