-- Add missing display columns to existing platforms table
-- Run this in Supabase SQL Editor

ALTER TABLE platforms 
  ADD COLUMN IF NOT EXISTS name TEXT,
  ADD COLUMN IF NOT EXISTS vendor TEXT,
  ADD COLUMN IF NOT EXISTS context_limit INT,
  ADD COLUMN IF NOT EXISTS request_limit INT,
  ADD COLUMN IF NOT EXISTS request_used INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS logo_url TEXT;

-- Verify
SELECT column_name FROM information_schema.columns WHERE table_name = 'platforms';
