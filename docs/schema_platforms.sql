-- VIBESPILOT PLATFORM REGISTRY
-- Web AI platforms for courier dispatch

-- PLATFORMS TABLE (Web destinations for couriers)
CREATE TABLE platforms (
  id TEXT PRIMARY KEY,
  type TEXT DEFAULT 'web_courier',
  url TEXT NOT NULL,
  gmail_account TEXT DEFAULT 'vibes.agents@gmail.com',
  
  -- Capabilities
  capabilities TEXT[] DEFAULT '{}', -- ['reasoning', 'code', 'research', 'analysis', 'creative']
  
  -- Usage & Limits
  daily_limit INT DEFAULT 50,
  daily_used INT DEFAULT 0,
  usage_reset_at TIMESTAMPTZ,
  
  -- Performance Tracking
  success_rate FLOAT DEFAULT 0,
  total_tasks INT DEFAULT 0,
  successful_tasks INT DEFAULT 0,
  avg_response_time_ms INT,
  
  -- Health
  last_success TIMESTAMPTZ,
  last_failure TIMESTAMPTZ,
  consecutive_failures INT DEFAULT 0,
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'benched', 'paused', 'offline')),
  status_reason TEXT,
  
  -- ROI Data
  theoretical_api_cost_per_1k_tokens FLOAT, -- What it would cost via API
  actual_courier_cost_per_task FLOAT DEFAULT 0, -- What we actually pay (usually 0 for free tier)
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- SEED INITIAL PLATFORMS
INSERT INTO platforms (id, url, capabilities, daily_limit, theoretical_api_cost_per_1k_tokens) VALUES
('chatgpt-free', 'https://chat.openai.com', ARRAY['reasoning', 'code', 'creative', 'analysis'], 50, 0.002),
('claude-free', 'https://claude.ai', ARRAY['reasoning', 'code', 'analysis', 'long-context'], 45, 0.003),
('gemini-free', 'https://gemini.google.com', ARRAY['reasoning', 'code', 'research', 'vision'], 60, 0.001),
('perplexity-free', 'https://perplexity.ai', ARRAY['research', 'reasoning', 'web-search'], 40, 0.002),
('deepseek-web', 'https://chat.deepseek.com', ARRAY['reasoning', 'code', 'math'], 50, 0.001),
('grok-free', 'https://x.com/i/grok', ARRAY['reasoning', 'creative', 'real-time'], 30, 0.002);

-- INDEXES
CREATE INDEX idx_platforms_status ON platforms(status);
CREATE INDEX idx_platforms_success ON platforms(success_rate DESC);

-- FUNCTION: Select best platform for task
CREATE OR REPLACE FUNCTION select_platform_for_task(
  p_task_type TEXT,
  p_capability TEXT
)
RETURNS TEXT AS $$
DECLARE
  v_platform_id TEXT;
BEGIN
  SELECT id INTO v_platform_id
  FROM platforms
  WHERE status = 'active'
    AND (p_capability = ANY(capabilities) OR p_capability IS NULL)
    AND daily_used < daily_limit * 0.8
    AND consecutive_failures < 3
  ORDER BY 
    success_rate DESC,
    (daily_limit - daily_used) DESC,
    avg_response_time_ms ASC NULLS LAST
  LIMIT 1;
  
  RETURN v_platform_id;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Record platform usage
CREATE OR REPLACE FUNCTION record_platform_usage(
  p_platform_id TEXT,
  p_success BOOLEAN,
  p_response_time_ms INT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  IF p_success THEN
    UPDATE platforms
    SET daily_used = daily_used + 1,
        total_tasks = total_tasks + 1,
        successful_tasks = successful_tasks + 1,
        success_rate = successful_tasks::FLOAT / total_tasks,
        last_success = NOW(),
        consecutive_failures = 0,
        avg_response_time_ms = COALESCE(
          (avg_response_time_ms + p_response_time_ms) / 2,
          p_response_time_ms
        ),
        updated_at = NOW()
    WHERE id = p_platform_id;
  ELSE
    UPDATE platforms
    SET daily_used = daily_used + 1,
        total_tasks = total_tasks + 1,
        success_rate = successful_tasks::FLOAT / NULLIF(total_tasks, 0),
        last_failure = NOW(),
        consecutive_failures = consecutive_failures + 1,
        updated_at = NOW()
    WHERE id = p_platform_id;
    
    -- Auto-bench if too many failures
    UPDATE platforms
    SET status = 'benched',
        status_reason = 'Auto-benched: 3+ consecutive failures'
    WHERE id = p_platform_id
      AND consecutive_failures >= 3;
  END IF;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Reset daily usage (call via cron at midnight)
CREATE OR REPLACE FUNCTION reset_platform_daily_usage()
RETURNS VOID AS $$
BEGIN
  UPDATE platforms
  SET daily_used = 0,
      usage_reset_at = NOW(),
      updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

-- VIEW: Platform health dashboard
CREATE VIEW platform_health AS
SELECT 
  id,
  status,
  daily_used,
  daily_limit,
  ROUND((daily_used::FLOAT / daily_limit) * 100, 1) as daily_pct,
  ROUND(success_rate * 100, 1) as success_pct,
  consecutive_failures,
  last_success,
  last_failure,
  ARRAY_TO_STRING(capabilities, ', ') as caps
FROM platforms
ORDER BY 
  CASE WHEN status = 'active' THEN 0 ELSE 1 END,
  success_rate DESC;

SELECT 'Platform registry initialized. Platforms registered: ' || COUNT(*) FROM platforms;
