-- VibePilot Migration 038: Remove duplicate create_task_with_packet function
-- Purpose: Drop the old function with UUID[] dependencies, keep only JSONB version

-- Drop old function with UUID[] dependencies
DROP FUNCTION IF EXISTS public.create_task_with_packet(
  uuid, text, text, text, text, text, integer, double precision, text, text, text, uuid[], text, jsonb
);

SELECT 'Migration 038 complete - old create_task_with_packet dropped' AS status;
