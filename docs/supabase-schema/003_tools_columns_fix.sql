-- Add missing columns to tools table
-- Run this in Supabase SQL Editor

-- Add missing columns to existing tools table
ALTER TABLE tools ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE tools ADD COLUMN IF NOT EXISTS type TEXT;
ALTER TABLE tools ADD COLUMN IF NOT EXISTS supported_providers TEXT[];
ALTER TABLE tools ADD COLUMN IF NOT EXISTS has_codebase_access BOOLEAN DEFAULT FALSE;
ALTER TABLE tools ADD COLUMN IF NOT EXISTS has_browser_control BOOLEAN DEFAULT FALSE;
ALTER TABLE tools ADD COLUMN IF NOT EXISTS runner_class TEXT;

-- Verify columns
SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'tools';
