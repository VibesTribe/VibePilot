-- VIBESPILOT SCHEMA MIGRATION v1.2
-- Purpose: Add platforms table and model metadata for dashboard
-- Date: 2026-02-16
-- 
-- Run this AFTER schema_v1.1_routing.sql
-- 
-- Changes:
--   - Add platforms table (web courier targets)
--   - Add display columns to models (name, vendor, tier, logo, access_type)

-- 1. Create platforms table
CREATE TABLE IF NOT EXISTS platforms (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  vendor TEXT,
  type TEXT DEFAULT 'web' CHECK (type IN ('web', 'mcp', 'internal')),
  
  -- Limits (for dashboard display)
  context_limit INT,
  request_limit INT,
  request_used INT DEFAULT 0,
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'paused', 'offline')),
  status_reason TEXT,
  
  -- Display
  logo_url TEXT,
  
  updated_at TIMESTAMPT DEFAULT NOW()
);

-- 2. Add display columns to models if they don't exist
ALTER TABLE models 
  ADD COLUMN IF NOT EXISTS name TEXT,
  ADD COLUMN IF NOT EXISTS vendor TEXT,
  ADD COLUMN IF NOT EXISTS access_type TEXT DEFAULT 'api' CHECK (access_type IN ('api', 'cli', 'cli_subscription', 'web')),
  ADD COLUMN IF NOT EXISTS logo_url TEXT,
  ADD COLUMN IF NOT EXISTS subscription_status TEXT,
  ADD COLUMN IF NOT EXISTS subscription_ends TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS subscription_cost DECIMAL(10,2);

-- 3. Create index
CREATE INDEX IF NOT EXISTS idx_platforms_status ON platforms(status);
CREATE INDEX IF NOT EXISTS idx_models_access_type ON models(access_type);

-- 4. Function to get all agents (models + platforms) for dashboard
CREATE OR REPLACE FUNCTION get_dashboard_agents()
RETURNS TABLE (
  id TEXT,
  name TEXT,
  type TEXT,
  vendor TEXT,
  tier TEXT,
  context_limit INT,
  status TEXT,
  logo_url TEXT,
  access_type TEXT
) AS $$
BEGIN
  -- Return models as internal agents (Q tier)
  RETURN QUERY
  SELECT 
    m.id,
    COALESCE(m.name, m.id) as name,
    'model' as type,
    m.vendor,
    CASE 
      WHEN m.access_type IN ('api', 'cli', 'cli_subscription') THEN 'Q'
      ELSE 'W'
    END as tier,
    m.context_limit,
    m.status,
    m.logo_url,
    m.access_type
  FROM models m
  WHERE m.status = 'active'
  
  UNION ALL
  
  -- Return platforms as web agents (W tier)
  SELECT 
    p.id,
    p.name,
    'platform' as type,
    p.vendor,
    'W' as tier,
    p.context_limit,
    p.status,
    p.logo_url,
    'web' as access_type
  FROM platforms p
  WHERE p.status = 'active'
  ORDER BY tier, name;
END;
$$ LANGUAGE plpgsql;

-- Verify
SELECT 'Migration v1.2 complete' AS status;
SELECT COUNT(*) as models_count FROM models;
SELECT COUNT(*) as platforms_count FROM platforms;
