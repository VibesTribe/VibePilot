-- VibePilot Migration 039: Remove duplicate update_plan_status function
-- Purpose: Drop the old 3-parameter version, keep only the 4-parameter version with p_plan_path

-- Drop old function with 3 parameters
DROP FUNCTION IF EXISTS public.update_plan_status(uuid, text, jsonb);

SELECT 'Migration 039 complete - old update_plan_status dropped' AS status;
