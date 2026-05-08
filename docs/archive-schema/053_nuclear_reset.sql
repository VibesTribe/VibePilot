-- VibePilot Migration 053: Nuclear Reset for Clean Test
-- Purpose: DELETE all test data, keep only clean-flow-test PRD
-- Run this when: Need a completely clean slate for testing

-- ============================================================================
-- STEP 1: Delete all test tasks
-- ============================================================================

DELETE FROM tasks 
WHERE plan_id IN (
  SELECT id FROM plans 
  WHERE prd_path LIKE '%governor-startup-message%' 
     OR prd_path LIKE '%governor-log-timestamps%'
     OR prd_path LIKE '%governor-heartbeat-log%'
     OR prd_path LIKE '%vibepilot-flow-test%'
     OR prd_path LIKE '%test-autonomous-flow%'
);

-- ============================================================================
-- STEP 2: Delete all test plans
-- ============================================================================

DELETE FROM plans 
WHERE prd_path LIKE '%governor-startup-message%' 
   OR prd_path LIKE '%governor-log-timestamps%'
   OR prd_path LIKE '%governor-heartbeat-log%'
   OR prd_path LIKE '%vibepilot-flow-test%'
   OR prd_path LIKE '%test-autonomous-flow%';

-- ============================================================================
-- STEP 3: Clear all processing claims
-- ============================================================================

UPDATE plans SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE tasks SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

-- ============================================================================
-- STEP 4: Show remaining PRDs
-- ============================================================================

SELECT 
  'Remaining PRDs' as info,
  COUNT(*) as count
FROM plans;

SELECT 
  'Remaining Tasks' as info,
  COUNT(*) as count  
FROM tasks;

SELECT 'Migration 053 complete - Nuclear reset done, only clean-flow-test PRD remains' AS status;
