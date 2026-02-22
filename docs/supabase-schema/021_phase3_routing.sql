-- Phase 3: Intelligent Routing Schema
-- Creates: tools, runners tables, RPCs for model selection
-- Run this in Supabase SQL Editor

-- ============================================================================
-- 1. TOOLS TABLE
-- CLI tools, browser automation, MCP connectors
-- ============================================================================

CREATE TABLE IF NOT EXISTS tools (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('cli', 'browser', 'mcp')),
  
  command TEXT,
  ram_requirement_mb INT DEFAULT 500,
  
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'removed')),
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Seed initial tools
INSERT INTO tools (id, name, type, command, ram_requirement_mb) VALUES
  ('opencode', 'OpenCode CLI', 'cli', 'opencode', 500),
  ('kimi-cli', 'Kimi CLI', 'cli', 'kimi', 500),
  ('browser-use', 'Browser Use', 'browser', NULL, 1000)
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 2. RUNNERS TABLE
-- Combines model + tool + capabilities + ratings
-- ============================================================================

CREATE TABLE IF NOT EXISTS runners (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  model_id TEXT REFERENCES models(id) ON DELETE CASCADE,
  tool_id TEXT REFERENCES tools(id) ON DELETE CASCADE,
  
  -- Capabilities
  routing_capability TEXT[] DEFAULT '{web}',
  -- 'internal': CLI/API with codebase access
  -- 'web': courier/browser delivery
  -- 'mcp': future MCP connectors
  
  -- Priority (lower = better)
  cost_priority INT DEFAULT 2 CHECK (cost_priority BETWEEN 0 AND 2),
  -- 0 = subscription (best), 1 = free API, 2 = paid API (worst)
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN (
    'active', 'cooldown', 'rate_limited', 'paused', 'benched'
  )),
  status_reason TEXT,
  
  -- Performance tracking
  strengths TEXT[] DEFAULT '{}',
  task_ratings JSONB DEFAULT '{}',
  -- Format: {'coding': {'success': 10, 'fail': 2}, 'research': {...}}
  
  -- Limit tracking
  daily_used INT DEFAULT 0,
  daily_limit INT,
  daily_reset_at TIMESTAMPTZ,
  
  cooldown_expires_at TIMESTAMPTZ,
  rate_limit_reset_at TIMESTAMPTZ,
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for routing queries
CREATE INDEX IF NOT EXISTS idx_runners_status_routing 
  ON runners(status, routing_capability);
CREATE INDEX IF NOT EXISTS idx_runners_priority 
  ON runners(cost_priority ASC) WHERE status = 'active';

-- Seed initial runners
INSERT INTO runners (model_id, tool_id, routing_capability, cost_priority, strengths, daily_limit)
SELECT 'glm-5', 'opencode', ARRAY['internal'], 0, ARRAY['coding', 'planning', 'execution'], 1000
WHERE NOT EXISTS (SELECT 1 FROM runners WHERE model_id = 'glm-5');

INSERT INTO runners (model_id, tool_id, routing_capability, cost_priority, strengths, daily_limit)
SELECT 'gemini-api', 'opencode', ARRAY['internal'], 1, ARRAY['research', 'coding'], 500
WHERE NOT EXISTS (SELECT 1 FROM runners WHERE model_id = 'gemini-api');

-- ============================================================================
-- 3. ADD COLUMNS TO MODELS TABLE
-- ============================================================================

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  access_type TEXT DEFAULT 'api' CHECK (access_type IN ('subscription', 'free_tier', 'paid_api'));

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_remaining_usd DECIMAL(10,4) DEFAULT 0;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_alert_threshold DECIMAL(10,4) DEFAULT 1.00;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  rate_limit_requests_per_minute INT DEFAULT 60;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  cost_per_1k_tokens_in DECIMAL(10,6) DEFAULT 0;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  cost_per_1k_tokens_out DECIMAL(10,6) DEFAULT 0;

-- Update existing models with access types
UPDATE models SET access_type = 'subscription' 
  WHERE id IN ('glm-5', 'opencode') AND access_type IS NULL;
UPDATE models SET access_type = 'free_tier' 
  WHERE id LIKE '%free%' OR id LIKE 'gemini-api' AND access_type IS NULL;
UPDATE models SET access_type = 'paid_api' 
  WHERE id LIKE '%deepseek%' AND access_type IS NULL;

-- ============================================================================
-- 4. ADD COLUMNS TO PLATFORMS TABLE (if not exists)
-- ============================================================================

ALTER TABLE platforms ADD COLUMN IF NOT EXISTS
  theoretical_cost_input_per_1k_usd DECIMAL(10,6);

ALTER TABLE platforms ADD COLUMN IF NOT EXISTS
  theoretical_cost_output_per_1k_usd DECIMAL(10,6);

-- Update existing platforms with theoretical costs
UPDATE platforms SET 
  theoretical_cost_input_per_1k_usd = 0.005,
  theoretical_cost_output_per_1k_usd = 0.015
WHERE id = 'chatgpt-free' AND theoretical_cost_input_per_1k_usd IS NULL;

UPDATE platforms SET 
  theoretical_cost_input_per_1k_usd = 0.003,
  theoretical_cost_output_per_1k_usd = 0.015
WHERE id = 'claude-free' AND theoretical_cost_input_per_1k_usd IS NULL;

UPDATE platforms SET 
  theoretical_cost_input_per_1k_usd = 0,
  theoretical_cost_output_per_1k_usd = 0
WHERE id = 'gemini-free' AND theoretical_cost_input_per_1k_usd IS NULL;

-- ============================================================================
-- 5. RPC: get_best_runner
-- Intelligent model selection based on routing, task type, availability
-- ============================================================================

CREATE OR REPLACE FUNCTION get_best_runner(
  p_routing TEXT,
  p_task_type TEXT DEFAULT NULL
)
RETURNS TABLE(
  id UUID,
  model_id TEXT,
  tool_id TEXT,
  cost_priority INT,
  strengths TEXT[]
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    r.id,
    r.model_id,
    r.tool_id,
    r.cost_priority,
    r.strengths
  FROM runners r
  JOIN models m ON r.model_id = m.id
  WHERE r.status = 'active'
    AND p_routing = ANY(r.routing_capability)
    AND (r.cooldown_expires_at IS NULL OR r.cooldown_expires_at < NOW())
    AND (r.rate_limit_reset_at IS NULL OR r.rate_limit_reset_at < NOW())
    AND (m.credit_remaining_usd IS NULL OR m.access_type != 'paid_api' OR m.credit_remaining_usd > m.credit_alert_threshold)
    AND (r.daily_limit IS NULL OR r.daily_used < r.daily_limit)
  ORDER BY
    r.cost_priority ASC,
    CASE 
      WHEN p_task_type IS NOT NULL AND r.task_ratings ? p_task_type 
      THEN (r.task_ratings->p_task_type->>'success')::float / 
           NULLIF((r.task_ratings->p_task_type->>'success')::int + 
                  (r.task_ratings->p_task_type->>'fail')::int, 0)
      ELSE 0.5
    END DESC NULLS LAST,
    m.success_rate DESC NULLS LAST,
    r.daily_used ASC NULLS LAST
  LIMIT 1;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 6. RPC: record_runner_result
-- Update stats after task completes
-- ============================================================================

CREATE OR REPLACE FUNCTION record_runner_result(
  p_runner_id UUID,
  p_task_type TEXT,
  p_success BOOLEAN,
  p_tokens_used INT
)
RETURNS VOID AS $$
DECLARE
  v_success_inc INT;
  v_fail_inc INT;
BEGIN
  v_success_inc := CASE WHEN p_success THEN 1 ELSE 0 END;
  v_fail_inc := CASE WHEN p_success THEN 0 ELSE 1 END;
  
  -- Update runner's task_ratings and daily_used
  UPDATE runners r
  SET 
    task_ratings = jsonb_set(
      COALESCE(r.task_ratings, '{}'::jsonb),
      ARRAY[COALESCE(p_task_type, 'general')],
      jsonb_build_object(
        'success', COALESCE((r.task_ratings->COALESCE(p_task_type, 'general')->>'success')::int, 0) + v_success_inc,
        'fail', COALESCE((r.task_ratings->COALESCE(p_task_type, 'general')->>'fail')::int, 0) + v_fail_inc
      )
    ),
    daily_used = r.daily_used + 1,
    updated_at = NOW()
  WHERE id = p_runner_id;
  
  -- Update model's tokens and task counts
  UPDATE models m
  SET 
    token_used = COALESCE(m.token_used, 0) + COALESCE(p_tokens_used, 0),
    tasks_completed = COALESCE(m.tasks_completed, 0) + v_success_inc,
    tasks_failed = COALESCE(m.tasks_failed, 0) + v_fail_inc,
    updated_at = NOW()
  FROM runners r
  WHERE r.id = p_runner_id AND m.id = r.model_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 7. RPC: refresh_limits
-- Auto-refresh expired cooldowns, rate limits, daily limits
-- ============================================================================

CREATE OR REPLACE FUNCTION refresh_limits()
RETURNS TABLE(runner_id UUID, action TEXT) AS $$
BEGIN
  -- Reset daily limits at midnight
  RETURN QUERY
  UPDATE runners
  SET 
    daily_used = 0,
    daily_reset_at = date_trunc('day', NOW()) + INTERVAL '1 day',
    status = CASE 
      WHEN status = 'cooldown' AND (cooldown_expires_at IS NULL OR cooldown_expires_at < NOW()) 
      THEN 'active'::text
      WHEN status = 'rate_limited' AND (rate_limit_reset_at IS NULL OR rate_limit_reset_at < NOW())
      THEN 'active'::text
      ELSE status
    END
  WHERE daily_reset_at IS NOT NULL 
    AND daily_reset_at < NOW()
    AND status IN ('active', 'cooldown', 'rate_limited')
  RETURNING id, 'daily_reset'::TEXT;

  -- Clear expired cooldowns
  RETURN QUERY
  UPDATE runners
  SET 
    status = 'active',
    cooldown_expires_at = NULL,
    updated_at = NOW()
  WHERE status = 'cooldown' 
    AND cooldown_expires_at IS NOT NULL 
    AND cooldown_expires_at < NOW()
  RETURNING id, 'cooldown_cleared'::TEXT;

  -- Clear expired rate limits
  RETURN QUERY
  UPDATE runners
  SET 
    status = 'active',
    rate_limit_reset_at = NULL,
    updated_at = NOW()
  WHERE status = 'rate_limited' 
    AND rate_limit_reset_at IS NOT NULL 
    AND rate_limit_reset_at < NOW()
  RETURNING id, 'rate_limit_cleared'::TEXT;

  RETURN;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 8. RPC: set_runner_cooldown
-- Put runner in cooldown (80% limit reached)
-- ============================================================================

CREATE OR REPLACE FUNCTION set_runner_cooldown(
  p_runner_id UUID,
  p_expires_at TIMESTAMPTZ
)
RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET 
    status = 'cooldown',
    cooldown_expires_at = p_expires_at,
    updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 9. RPC: set_runner_rate_limited
-- Mark runner as rate limited
-- ============================================================================

CREATE OR REPLACE FUNCTION set_runner_rate_limited(
  p_runner_id UUID,
  p_reset_at TIMESTAMPTZ
)
RETURNS VOID AS $$
BEGIN
  UPDATE runners
  SET 
    status = 'rate_limited',
    rate_limit_reset_at = p_reset_at,
    updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 10. GRANT PERMISSIONS
-- ============================================================================

GRANT SELECT, INSERT, UPDATE ON tools TO authenticated;
GRANT SELECT, INSERT, UPDATE ON runners TO authenticated;

GRANT EXECUTE ON FUNCTION get_best_runner(TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION record_runner_result(UUID, TEXT, BOOLEAN, INT) TO authenticated;
GRANT EXECUTE ON FUNCTION refresh_limits() TO authenticated;
GRANT EXECUTE ON FUNCTION set_runner_cooldown(UUID, TIMESTAMPTZ) TO authenticated;
GRANT EXECUTE ON FUNCTION set_runner_rate_limited(UUID, TIMESTAMPTZ) TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Phase 3 schema applied successfully' AS status;

SELECT COUNT(*) AS tools_count FROM tools;
SELECT COUNT(*) AS runners_count FROM runners;
