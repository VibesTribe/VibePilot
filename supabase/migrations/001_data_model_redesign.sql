-- VibePilot Data Model Migration
-- Run this in Supabase SQL Editor
-- Creates: models_new, access, task_history (tools already exists)

-- 1. models_new (AI capabilities only - no access info)
CREATE TABLE IF NOT EXISTS models_new (
  id TEXT PRIMARY KEY,
  name TEXT,
  provider TEXT,
  capabilities TEXT[],
  context_limit INT,
  cost_input_per_1k_usd FLOAT,
  cost_output_per_1k_usd FLOAT,
  notes TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE models_new IS 'AI models with their capabilities. Does NOT include access methods or limits.';

-- 2. tools (interfaces to use models) - may already exist
CREATE TABLE IF NOT EXISTS tools (
  id TEXT PRIMARY KEY,
  name TEXT,
  type TEXT,  -- 'cli', 'api', 'courier'
  supported_providers TEXT[],
  has_codebase_access BOOLEAN DEFAULT FALSE,
  has_browser_control BOOLEAN DEFAULT FALSE,
  runner_class TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE tools IS 'Interfaces for accessing models: opencode, kimi-cli, direct-api, courier';

-- 3. access (HOW we reach each model + limits + usage)
CREATE TABLE IF NOT EXISTS access (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id TEXT REFERENCES models_new(id),
  tool_id TEXT REFERENCES tools(id),
  platform_id TEXT,  -- For courier access, which platform
  
  method TEXT,  -- 'api', 'subscription', 'web_free_tier'
  priority INT DEFAULT 1,  -- 0=subscription(best), 1=web, 2=api(paid)
  
  status TEXT DEFAULT 'active',  -- 'active', 'paused', 'benched', 'cooldown'
  status_reason TEXT,
  cooldown_until TIMESTAMPTZ,
  
  -- Rate Limits (what we're allowed)
  requests_per_minute INT,
  requests_per_hour INT,
  requests_per_day INT,
  tokens_per_minute INT,
  tokens_per_day INT,
  
  -- Current Usage (rolling windows - updated by orchestrator)
  requests_last_minute INT DEFAULT 0,
  requests_last_hour INT DEFAULT 0,
  requests_today INT DEFAULT 0,
  tokens_last_minute INT DEFAULT 0,
  tokens_today INT DEFAULT 0,
  
  -- Reset tracking
  minute_window_start TIMESTAMPTZ,
  hour_window_start TIMESTAMPTZ,
  daily_reset_at TIMESTAMPTZ,
  
  -- Subscription info (if applicable)
  subscription_cost_usd FLOAT,
  subscription_started_at TIMESTAMPTZ,
  subscription_ends_at TIMESTAMPTZ,
  
  -- API credentials (if applicable)
  api_key_ref TEXT,  -- Vault key reference
  
  -- Learning stats (updated after each task)
  total_tasks INT DEFAULT 0,
  successful_tasks INT DEFAULT 0,
  failed_tasks INT DEFAULT 0,
  avg_tokens_per_task FLOAT DEFAULT 0,
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE access IS 'How we access each model: method, limits, current usage, learning stats';

-- 4. task_history (for learning)
CREATE TABLE IF NOT EXISTS task_history (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID,
  access_id UUID REFERENCES access(id),
  
  task_type TEXT,
  estimated_tokens INT,
  actual_tokens_in INT,
  actual_tokens_out INT,
  actual_requests INT DEFAULT 1,
  
  success BOOLEAN,
  failure_reason TEXT,
  failure_code TEXT,
  
  duration_ms INT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE task_history IS 'History of task executions for learning better estimates';

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_access_model_id ON access(model_id);
CREATE INDEX IF NOT EXISTS idx_access_tool_id ON access(tool_id);
CREATE INDEX IF NOT EXISTS idx_access_status ON access(status);
CREATE INDEX IF NOT EXISTS idx_task_history_access_id ON task_history(access_id);
CREATE INDEX IF NOT EXISTS idx_task_history_task_type ON task_history(task_type);
