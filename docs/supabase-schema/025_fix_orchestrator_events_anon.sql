-- VibePilot Migration: Fix orchestrator_events access for dashboard
-- Version: 025
-- Purpose: Grant full read access on orchestrator_events to anon role
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- Grant SELECT on orchestrator_events to anon (dashboard uses anon key)
GRANT SELECT ON orchestrator_events TO anon;

-- Create RLS policy to allow anon to read orchestrator_events
CREATE POLICY "anon_read_orchestrator_events" ON orchestrator_events
  FOR SELECT
  TO anon
  USING (true);

-- Also ensure authenticated can read
CREATE POLICY IF NOT EXISTS "authenticated_read_orchestrator_events" ON orchestrator_events
  FOR SELECT
  TO authenticated
  USING (true);

-- Verify
DO $$
BEGIN
  RAISE NOTICE 'Migration 025 complete';
  RAISE NOTICE '  - orchestrator_events: SELECT granted to anon';
  RAISE NOTICE '  - RLS policy created for anon read access';
END $$;

COMMIT;
