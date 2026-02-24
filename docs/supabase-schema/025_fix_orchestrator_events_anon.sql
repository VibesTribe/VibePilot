-- VibePilot Migration: Fix orchestrator_events anon access
-- Version: 025
-- Purpose: Grant SELECT on orchestrator_events to anon role for dashboard
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- Grant SELECT on orchestrator_events to anon (dashboard uses anon key)
GRANT SELECT ON orchestrator_events TO anon;

-- Verify
DO $$
BEGIN
  RAISE NOTICE 'Migration 025 complete';
  RAISE NOTICE '  - orchestrator_events: SELECT granted to anon';
END $$;

COMMIT;
