-- VibePilot Migration: Fix RLS on access table
-- Version: 027
-- Purpose: Enable RLS on public.access table (security fix)
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- Enable RLS on access table
ALTER TABLE access ENABLE ROW LEVEL SECURITY;

-- Allow authenticated users to read/write access data
CREATE POLICY "Allow authenticated to manage access" ON access
  FOR ALL TO authenticated
  USING (true)
  WITH CHECK (true);

-- Verify
DO $$
BEGIN
  RAISE NOTICE 'Migration 027 complete';
  RAISE NOTICE '  - RLS enabled on access table';
  RAISE NOTICE '  - Policy created for authenticated role';
END $$;

COMMIT;
