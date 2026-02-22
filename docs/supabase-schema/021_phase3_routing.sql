-- Phase 3: Intelligent Routing Schema
-- Creates: runners table, RPCs, adds missing columns
-- NO SEED DATA - everything created dynamically

-- ============================================================================
-- 1. ADD RATE LIMITS CONFIG TO MODELS (flexible, easy to update)
-- ============================================================================

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  rate_limits JSONB DEFAULT '{}';
-- Example: {"per_minute": 5, "per_hour": 100, "per_day": 200, "per_week": 500}
-- Orchestrator reads this, enforces limits, updates as platform changes

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_remaining_usd DECIMAL(10,4) DEFAULT 0;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_alert_threshold DECIMAL(10,4) DEFAULT 1.00;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  rate_limit_requests_per_minute INT DEFAULT 60;

-- ============================================================================
-- 2. RUNNERS TABLE (links model + tool + capabilities + learning)
-- ============================================================================

CREATE TABLE IF NOT EXISTS runners (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Links to existing tables
  model_id TEXT REFERENCES models(id) ON DELETE CASCADE,
  tool_id TEXT REFERENCES tools(id) ON DELETE CASCADE,
  
  -- Routing
  routing_capability TEXT[] DEFAULT '{}',
  -- 'internal': CLI/API with codebase access
  -- 'web': courier/browser delivery  
  -- 'mcp': future MCP connectors
  
  -- Priority (lower = preferred)
  cost_priority INT DEFAULT 2 CHECK (cost_priority BETWEEN 0 AND 2),
  -- 0 = subscription (best), 1 = free API, 2 = paid API
  
  -- Status (mirrors model status but runner-specific)
  status TEXT DEFAULT 'active' CHECK (status IN (
    'active', 'cooldown', 'rate_limited', 'paused', 'benched'
  )),
  status_reason TEXT,
  
  -- Learning: task type performance
  task_ratings JSONB DEFAULT '{}',
  -- Format: {'coding': {'success': 10, 'fail': 2}, 'research': {...}}
  
  -- Usage tracking (runner-level)
  daily_used INT DEFAULT 0,
  daily_limit INT,
  daily_reset_at TIMESTAMPTZ,
  
  -- Cooldown/rate limit timers
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
CREATE INDEX IF NOT EXISTS idx_runners_model 
  ON runners(model_id);

-- ============================================================================
-- 3. RPC: get_best_runner
-- Intelligent model selection based on availability, cost, success rate
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
  daily_used INT,
  daily_limit INT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    r.id,
    r.model_id,
    r.tool_id,
    r.cost_priority,
    r.daily_used,
    r.daily_limit
  FROM runners r
  JOIN models m ON r.model_id = m.id
  WHERE r.status = 'active'
    AND m.status = 'active'
    AND p_routing = ANY(r.routing_capability)
    AND (r.cooldown_expires_at IS NULL OR r.cooldown_expires_at < NOW())
    AND (r.rate_limit_reset_at IS NULL OR r.rate_limit_reset_at < NOW())
    AND (m.credit_remaining_usd IS NULL OR m.access_type != 'paid_api' OR m.credit_remaining_usd > m.credit_alert_threshold)
    AND (r.daily_limit IS NULL OR r.daily_used < r.daily_limit)
  ORDER BY
    -- Prefer lower cost priority (0=subscription, 1=free, 2=paid)
    r.cost_priority ASC,
    -- Then by task type success rate if available
    CASE 
      WHEN p_task_type IS NOT NULL AND r.task_ratings ? p_task_type 
      THEN (r.task_ratings->p_task_type->>'success')::float / 
           NULLIF((r.task_ratings->p_task_type->>'success')::int + 
                  (r.task_ratings->p_task_type->>'fail')::int, 0)
      ELSE 0.5
    END DESC NULLS LAST,
    -- Then by model's overall success rate
    m.success_rate DESC NULLS LAST,
    -- Then by least daily usage
    r.daily_used ASC NULLS LAST
  LIMIT 1;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 4. RPC: record_runner_result
-- Update stats after task completes (learning)
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
  
  -- Update runner's task_ratings (learning)
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
  
  -- Update model's task counts
  UPDATE models m
  SET 
    tokens_used = COALESCE(m.tokens_used, 0) + COALESCE(p_tokens_used, 0),
    tasks_completed = COALESCE(m.tasks_completed, 0) + v_success_inc,
    tasks_failed = COALESCE(m.tasks_failed, 0) + v_fail_inc,
    updated_at = NOW()
  FROM runners r
  WHERE r.id = p_runner_id AND m.id = r.model_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 5. RPC: refresh_limits
-- Auto-refresh expired cooldowns, rate limits, daily limits
-- Called by Janitor every minute
-- ============================================================================

CREATE OR REPLACE FUNCTION refresh_limits()
RETURNS VOID AS $$
BEGIN
  -- Clear expired cooldowns
  UPDATE runners
  SET status = 'active',
      cooldown_expires_at = NULL,
      updated_at = NOW()
  WHERE status = 'cooldown' 
    AND cooldown_expires_at IS NOT NULL 
    AND cooldown_expires_at < NOW();

  -- Clear expired rate limits
  UPDATE runners
  SET status = 'active',
      rate_limit_reset_at = NULL,
      updated_at = NOW()
  WHERE status = 'rate_limited' 
    AND rate_limit_reset_at IS NOT NULL 
    AND rate_limit_reset_at < NOW();

  -- Reset daily limits at midnight (if daily_reset_at set)
  UPDATE runners
  SET daily_used = 0,
      daily_reset_at = date_trunc('day', NOW()) + INTERVAL '1 day',
      updated_at = NOW()
  WHERE daily_reset_at IS NOT NULL 
    AND daily_reset_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 6. RPC: set_runner_cooldown
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
-- 7. RPC: set_runner_rate_limited
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
-- 8. GRANT PERMISSIONS
-- ============================================================================

GRANT SELECT, INSERT, UPDATE ON runners TO authenticated;

GRANT EXECUTE ON FUNCTION get_best_runner(TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION record_runner_result(UUID, TEXT, BOOLEAN, INT) TO authenticated;
GRANT EXECUTE ON FUNCTION refresh_limits() TO authenticated;
GRANT EXECUTE ON FUNCTION set_runner_cooldown(UUID, TIMESTAMPTZ) TO authenticated;
GRANT EXECUTE ON FUNCTION set_runner_rate_limited(UUID, TIMESTAMPTZ) TO authenticated;

-- ============================================================================
-- DONE
-- No seed data. Runners created when models are set up.
-- Rate limits in models.rate_limits JSONB, easy to update anytime.
-- ============================================================================
