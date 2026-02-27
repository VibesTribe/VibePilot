-- Destinations Table
-- WHERE tasks execute: CLI tools, API endpoints, Web platforms
-- Config-driven: status change = instant routing change, zero code changes
-- 
-- Flow: destinations.json → sync script → this table → Go queries
-- If destination dies: set status = 'inactive' → sync → system excludes automatically

CREATE TABLE IF NOT EXISTS destinations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,                     -- 'cli', 'api', 'web'
    status TEXT DEFAULT 'active',           -- 'active', 'inactive', 'paused'
    
    -- CLI-specific
    command TEXT,                           -- e.g., "opencode", "kimi"
    
    -- API-specific
    endpoint TEXT,                          -- e.g., "https://generativelanguage.googleapis.com/v1beta"
    api_key_ref TEXT,                       -- Vault reference, e.g., "GEMINI_API_KEY"
    
    -- Web-specific (courier)
    url TEXT,                               -- e.g., "https://chat.openai.com"
    new_chat_url TEXT,                      -- e.g., "https://chat.openai.com/?model=auto"
    
    -- Common config
    cost_category TEXT,                     -- 'free', 'subscription_sunk', 'credit_burn'
    rate_limits JSONB DEFAULT '{}',         -- {"rpm": 15, "rpd": 1500}
    throttle JSONB DEFAULT '{}',            -- {"strategy": "pace", "at_percent": 80}
    models_available TEXT[] DEFAULT '{}',   -- Which models this destination can run
    
    -- Preserve full config for flexibility
    config JSONB DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_destinations_status ON destinations(status);
CREATE INDEX IF NOT EXISTS idx_destinations_type ON destinations(type);

-- RPC: Get single active destination
CREATE OR REPLACE FUNCTION get_destination(p_id TEXT)
RETURNS destinations
LANGUAGE plpgsql
AS $$
DECLARE
    result destinations%ROWTYPE;
BEGIN
    SELECT * INTO result FROM destinations WHERE id = p_id AND status = 'active';
    RETURN result;
END;
$$;

-- RPC: Get all active destinations by type
CREATE OR REPLACE FUNCTION get_destinations_by_type(p_type TEXT)
RETURNS SETOF destinations
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY SELECT * FROM destinations WHERE type = p_type AND status = 'active';
END;
$$;

-- RPC: Get all active destinations
CREATE OR REPLACE FUNCTION get_active_destinations()
RETURNS SETOF destinations
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY SELECT * FROM destinations WHERE status = 'active' ORDER BY id;
END;
$$;

-- Update get_best_runner to also check destination status
-- This ensures runners with inactive destinations are excluded
DROP FUNCTION IF EXISTS get_best_runner(TEXT, TEXT);

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
  LEFT JOIN destinations d ON r.tool_id = d.id
  WHERE r.status = 'active'
    AND m.status = 'active'
    AND (d.status = 'active' OR d.status IS NULL)
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

-- Trigger: Update updated_at on change
CREATE OR REPLACE FUNCTION update_destinations_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trigger_destinations_updated_at ON destinations;
CREATE TRIGGER trigger_destinations_updated_at
    BEFORE UPDATE ON destinations
    FOR EACH ROW
    EXECUTE FUNCTION update_destinations_updated_at();

-- Grant permissions
GRANT SELECT, INSERT, UPDATE ON destinations TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;

COMMENT ON TABLE destinations IS 'WHERE tasks execute. CLI tools, API endpoints, Web platforms. Config-driven routing.';
COMMENT ON FUNCTION get_destination(TEXT) IS 'Get active destination by ID. Returns null if inactive.';
COMMENT ON FUNCTION get_best_runner(TEXT, TEXT) IS 'Best available runner with active model AND active destination.';
