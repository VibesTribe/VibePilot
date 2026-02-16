-- VIBESPILOT SCHEMA MIGRATION v1.3
-- Purpose: Store full config in JSONB for live updates
-- Date: 2026-02-16
-- 
-- Run this AFTER schema_v1.2.1_platforms_fix.sql
-- 
-- Changes:
--   - Add config JSONB to models (stores full model config)
--   - Add config JSONB to platforms (stores full platform config)
--   - Add live stats columns for routing decisions

-- Add config columns
ALTER TABLE models ADD COLUMN IF NOT EXISTS config JSONB;
ALTER TABLE platforms ADD COLUMN IF NOT EXISTS config JSONB;

-- Add live stats to models
ALTER TABLE models 
  ADD COLUMN IF NOT EXISTS tokens_used INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tasks_completed INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tasks_failed INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS success_rate DECIMAL(5,2) DEFAULT 0.00,
  ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;

-- Add live stats to platforms
ALTER TABLE platforms
  ADD COLUMN IF NOT EXISTS tokens_used INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tasks_completed INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tasks_failed INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS success_rate DECIMAL(5,2) DEFAULT 0.00,
  ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;

-- Create skills table
CREATE TABLE IF NOT EXISTS skills (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  config JSONB,
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create prompts table
CREATE TABLE IF NOT EXISTS prompts (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  content TEXT NOT NULL,
  version INT DEFAULT 1,
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create tools table
CREATE TABLE IF NOT EXISTS tools (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  config JSONB,
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Function to update model stats after task run
CREATE OR REPLACE FUNCTION update_model_stats(
  p_model_id TEXT,
  p_success BOOLEAN,
  p_tokens INT
) RETURNS void AS $$
BEGIN
  UPDATE models SET
    tokens_used = tokens_used + COALESCE(p_tokens, 0),
    tasks_completed = tasks_completed + CASE WHEN p_success THEN 1 ELSE 0 END,
    tasks_failed = tasks_failed + CASE WHEN NOT p_success THEN 1 ELSE 0 END,
    success_rate = CASE 
      WHEN (tasks_completed + tasks_failed + CASE WHEN NOT p_success THEN 1 ELSE 0 END) > 0 
      THEN ROUND(
        (tasks_completed + CASE WHEN p_success THEN 1 ELSE 0 END)::DECIMAL / 
        (tasks_completed + tasks_failed + 1) * 100, 2
      )
      ELSE 0
    END,
    last_run_at = NOW(),
    updated_at = NOW()
  WHERE id = p_model_id;
END;
$$ LANGUAGE plpgsql;

-- Function to update platform stats after task run
CREATE OR REPLACE FUNCTION update_platform_stats(
  p_platform_id TEXT,
  p_success BOOLEAN,
  p_tokens INT
) RETURNS void AS $$
BEGIN
  UPDATE platforms SET
    tokens_used = tokens_used + COALESCE(p_tokens, 0),
    tasks_completed = tasks_completed + CASE WHEN p_success THEN 1 ELSE 0 END,
    tasks_failed = tasks_failed + CASE WHEN NOT p_success THEN 1 ELSE 0 END,
    success_rate = CASE 
      WHEN (tasks_completed + tasks_failed + CASE WHEN NOT p_success THEN 1 ELSE 0 END) > 0 
      THEN ROUND(
        (tasks_completed + CASE WHEN p_success THEN 1 ELSE 0 END)::DECIMAL / 
        (tasks_completed + tasks_failed + 1) * 100, 2
      )
      ELSE 0
    END,
    last_run_at = NOW(),
    updated_at = NOW()
  WHERE id = p_platform_id;
END;
$$ LANGUAGE plpgsql;

-- Verify
SELECT 'Migration v1.3 complete' AS status;
