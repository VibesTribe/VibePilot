-- VibePilot Migration: Event Persistence & Recovery
-- Version: 032
-- Purpose: Durable event processing, orphan recovery, usage tracking
-- Design: Checkpoints survive restarts, sessions track active runners
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- 1. EVENT CHECKPOINTS (persist last processed timestamps)
-- ============================================================================

CREATE TABLE IF NOT EXISTS event_checkpoints (
  source TEXT PRIMARY KEY,
  last_seen_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE event_checkpoints IS 'Tracks last processed timestamp per event source to survive restarts';

-- Seed initial checkpoints
INSERT INTO event_checkpoints (source, last_seen_at) VALUES
  ('tasks', NOW()),
  ('plans', NOW()),
  ('maintenance_commands', NOW()),
  ('test_results', NOW())
ON CONFLICT (source) DO NOTHING;

-- ============================================================================
-- 2. RUNNER SESSIONS (track active executions for orphan detection)
-- ============================================================================

CREATE TABLE IF NOT EXISTS runner_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  destination_id TEXT NOT NULL,
  model_id TEXT,
  started_at TIMESTAMPTZ DEFAULT NOW(),
  last_heartbeat TIMESTAMPTZ DEFAULT NOW(),
  status TEXT DEFAULT 'running' CHECK (status IN ('running', 'completed', 'orphaned', 'failed')),
  failure_reason TEXT,
  completed_at TIMESTAMPTZ,
  
  -- Tokens for ROI
  tokens_in INT DEFAULT 0,
  tokens_out INT DEFAULT 0,
  
  -- Cost tracking
  theoretical_cost_usd DECIMAL(10,6) DEFAULT 0,
  actual_cost_usd DECIMAL(10,6) DEFAULT 0
);

COMMENT ON TABLE runner_sessions IS 'Active runner sessions for heartbeat tracking and orphan detection';

CREATE INDEX IF NOT EXISTS idx_runner_sessions_status ON runner_sessions(status) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_runner_sessions_task ON runner_sessions(task_id);
CREATE INDEX IF NOT EXISTS idx_runner_sessions_heartbeat ON runner_sessions(last_heartbeat) WHERE status = 'running';

-- ============================================================================
-- 3. EVENT QUEUE (for replay and audit)
-- ============================================================================

CREATE TABLE IF NOT EXISTS event_queue (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type TEXT NOT NULL,
  source_table TEXT NOT NULL,
  record_id TEXT NOT NULL,
  payload JSONB,
  status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
  attempts INT DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  processed_at TIMESTAMPTZ
);

COMMENT ON TABLE event_queue IS 'Durable event queue for replay and debugging';

CREATE INDEX IF NOT EXISTS idx_event_queue_pending ON event_queue(status, created_at) 
  WHERE status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_event_queue_type ON event_queue(event_type);

-- ============================================================================
-- 4. ENHANCE MODELS TABLE (usage windows and learned data)
-- ============================================================================

-- Add usage windows for multi-timeframe tracking
ALTER TABLE models ADD COLUMN IF NOT EXISTS usage_windows JSONB DEFAULT '{
  "minute": {"requests": 0, "tokens": 0, "window_start": null, "reset_at": null},
  "hour": {"requests": 0, "tokens": 0, "window_start": null, "reset_at": null},
  "day": {"requests": 0, "tokens": 0, "window_start": null, "reset_at": null},
  "week": {"requests": 0, "tokens": 0, "window_start": null, "reset_at": null}
}';

-- Add learned data column if not exists
ALTER TABLE models ADD COLUMN IF NOT EXISTS learned JSONB DEFAULT '{
  "avg_task_duration_seconds": null,
  "failure_rate_by_type": {},
  "optimal_cooldown_minutes": null,
  "best_for_task_types": [],
  "avoid_for_task_types": []
}';

-- Add consecutive failures tracking
ALTER TABLE models ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS last_failure_type TEXT;
ALTER TABLE models ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ;
ALTER TABLE models ADD COLUMN IF NOT EXISTS last_rate_limit_at TIMESTAMPTZ;
ALTER TABLE models ADD COLUMN IF NOT EXISTS rate_limit_count INT DEFAULT 0;

COMMENT ON COLUMN models.usage_windows IS 'Multi-timeframe usage tracking for rate limit enforcement';
COMMENT ON COLUMN models.learned IS 'System-learned optimal settings based on task performance';
COMMENT ON COLUMN models.consecutive_failures IS 'Count of consecutive failures for auto-benching';

-- ============================================================================
-- 5. SYSTEM CONFIG TABLE (fallback defaults)
-- ============================================================================

CREATE TABLE IF NOT EXISTS system_config (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE system_config IS 'System-wide fallback configuration';

INSERT INTO system_config (key, value) VALUES
('defaults', '{
  "throttle_behavior": "slow_down",
  "buffer_pct": 80,
  "spacing_min_seconds": 1,
  "recovery": {
    "on_rate_limit": "cooldown",
    "cooldown_minutes": 30,
    "timeout_seconds": 300,
    "heartbeat_interval_seconds": 30,
    "orphan_threshold_seconds": 300,
    "max_task_attempts": 3,
    "model_failure_threshold": 3
  }
}')
ON CONFLICT (key) DO NOTHING;

-- ============================================================================
-- 6. RPC: Update checkpoint
-- ============================================================================

CREATE OR REPLACE FUNCTION update_event_checkpoint(
  p_source TEXT,
  p_last_seen_at TIMESTAMPTZ
) RETURNS VOID AS $$
BEGIN
  INSERT INTO event_checkpoints (source, last_seen_at, updated_at)
  VALUES (p_source, p_last_seen_at, NOW())
  ON CONFLICT (source) 
  DO UPDATE SET 
    last_seen_at = p_last_seen_at,
    updated_at = NOW();
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 7. RPC: Get checkpoint
-- ============================================================================

CREATE OR REPLACE FUNCTION get_event_checkpoint(p_source TEXT)
RETURNS TIMESTAMPTZ AS $$
DECLARE
  v_last_seen TIMESTAMPTZ;
BEGIN
  SELECT last_seen_at INTO v_last_seen
  FROM event_checkpoints
  WHERE source = p_source;
  
  RETURN COALESCE(v_last_seen, NOW() - INTERVAL '1 hour');
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 8. RPC: Find orphaned sessions
-- ============================================================================

CREATE OR REPLACE FUNCTION find_orphaned_sessions(
  p_orphan_threshold_seconds INT DEFAULT 300
) RETURNS TABLE (
  id UUID,
  task_id UUID,
  destination_id TEXT,
  model_id TEXT,
  started_at TIMESTAMPTZ,
  last_heartbeat TIMESTAMPTZ,
  seconds_since_heartbeat INT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    rs.id,
    rs.task_id,
    rs.destination_id,
    rs.model_id,
    rs.started_at,
    rs.last_heartbeat,
    EXTRACT(EPOCH FROM (NOW() - rs.last_heartbeat))::INT as seconds_since_heartbeat
  FROM runner_sessions rs
  WHERE rs.status = 'running'
    AND rs.last_heartbeat < NOW() - (p_orphan_threshold_seconds || ' seconds')::INTERVAL
  ORDER BY rs.last_heartbeat ASC;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 9. RPC: Recover orphaned session
-- ============================================================================

CREATE OR REPLACE FUNCTION recover_orphaned_session(
  p_session_id UUID,
  p_reason TEXT DEFAULT 'orphaned'
) RETURNS UUID AS $$
DECLARE
  v_task_id UUID;
BEGIN
  -- Get task_id before marking orphaned
  SELECT task_id INTO v_task_id
  FROM runner_sessions
  WHERE id = p_session_id;
  
  -- Mark session as orphaned
  UPDATE runner_sessions
  SET status = 'orphaned',
      failure_reason = p_reason,
      completed_at = NOW()
  WHERE id = p_session_id;
  
  -- If task is still in_progress, reset to available
  IF v_task_id IS NOT NULL THEN
    UPDATE tasks
    SET status = 'available',
        attempts = attempts + 1,
        updated_at = NOW()
    WHERE id = v_task_id
      AND status = 'in_progress';
  END IF;
  
  -- Log to event queue
  INSERT INTO event_queue (event_type, source_table, record_id, payload, status)
  VALUES (
    'session_orphaned',
    'runner_sessions',
    p_session_id::TEXT,
    jsonb_build_object(
      'session_id', p_session_id,
      'task_id', v_task_id,
      'reason', p_reason
    ),
    'completed'
  );
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 10. RPC: Record model failure
-- ============================================================================

CREATE OR REPLACE FUNCTION record_model_failure(
  p_model_id TEXT,
  p_failure_type TEXT,
  p_task_id UUID DEFAULT NULL
) RETURNS JSONB AS $$
DECLARE
  v_consecutive INT;
  v_should_bench BOOLEAN;
  v_cooldown_minutes INT;
  v_threshold INT;
BEGIN
  -- Get threshold from system config
  SELECT value->'recovery'->>'model_failure_threshold' INTO v_threshold
  FROM system_config WHERE key = 'defaults';
  v_threshold := COALESCE(v_threshold::INT, 3);
  
  -- Increment consecutive failures
  UPDATE models
  SET consecutive_failures = consecutive_failures + 1,
      last_failure_type = p_failure_type,
      last_failure_at = NOW()
  WHERE id = p_model_id
  RETURNING consecutive_failures INTO v_consecutive;
  
  -- Check if should bench
  v_should_bench := v_consecutive >= v_threshold;
  
  IF v_should_bench THEN
    -- Get cooldown from model or defaults
    SELECT 
      COALESCE(
        config->'recovery'->>'cooldown_minutes',
        (SELECT value->'recovery'->>'cooldown_minutes' FROM system_config WHERE key = 'defaults')
      ) INTO v_cooldown_minutes;
    v_cooldown_minutes := COALESCE(v_cooldown_minutes::INT, 30);
    
    UPDATE models
    SET status = 'paused',
        status_reason = 'auto_benched:consecutive_failures',
        cooldown_expires_at = NOW() + (v_cooldown_minutes || ' minutes')::INTERVAL
    WHERE id = p_model_id;
  END IF;
  
  RETURN jsonb_build_object(
    'model_id', p_model_id,
    'consecutive_failures', v_consecutive,
    'should_bench', v_should_bench,
    'failure_type', p_failure_type
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 11. RPC: Record model success (reset consecutive failures)
-- ============================================================================

CREATE OR REPLACE FUNCTION record_model_success(
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL,
  p_duration_seconds FLOAT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
  UPDATE models
  SET consecutive_failures = 0,
      tasks_completed = tasks_completed + 1,
      last_run_at = NOW()
  WHERE id = p_model_id;
  
  -- If task type provided, update learned data
  IF p_task_type IS NOT NULL THEN
    UPDATE models
    SET learned = jsonb_set(
      COALESCE(learned, '{}'::jsonb),
      '{best_for_task_types}',
      COALESCE(learned->'best_for_task_types', '[]'::jsonb) || jsonb_build_array(p_task_type)
    )
    WHERE id = p_model_id
      AND NOT (learned->'best_for_task_types' ? p_task_type);
  END IF;
  
  -- Update average duration
  IF p_duration_seconds IS NOT NULL THEN
    UPDATE models
    SET learned = jsonb_set(
      COALESCE(learned, '{}'::jsonb),
      '{avg_task_duration_seconds}',
      COALESCE(learned->'avg_task_duration_seconds', p_duration_seconds) * 0.9 + p_duration_seconds * 0.1
    )
    WHERE id = p_model_id;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 12. RPC: Check model availability (rate limits + cooldown)
-- ============================================================================

CREATE OR REPLACE FUNCTION check_model_availability(
  p_model_id TEXT,
  p_estimated_tokens INT DEFAULT 0
) RETURNS JSONB AS $$
DECLARE
  v_model RECORD;
  v_available BOOLEAN := true;
  v_reason TEXT := null;
  v_wait_seconds INT := 0;
  v_buffer_pct INT;
BEGIN
  SELECT * INTO v_model FROM models WHERE id = p_model_id;
  
  IF NOT FOUND THEN
    RETURN jsonb_build_object('available', false, 'reason', 'model_not_found');
  END IF;
  
  -- Check status
  IF v_model.status != 'active' THEN
    RETURN jsonb_build_object(
      'available', false, 
      'reason', 'model_not_active',
      'status', v_model.status,
      'status_reason', v_model.status_reason
    );
  END IF;
  
  -- Check cooldown
  IF v_model.cooldown_expires_at IS NOT NULL AND v_model.cooldown_expires_at > NOW() THEN
    v_wait_seconds := EXTRACT(EPOCH FROM (v_model.cooldown_expires_at - NOW()))::INT;
    RETURN jsonb_build_object(
      'available', false,
      'reason', 'cooldown_active',
      'wait_seconds', v_wait_seconds,
      'cooldown_expires_at', v_model.cooldown_expires_at
    );
  END IF;
  
  -- Get buffer percentage
  v_buffer_pct := COALESCE(
    (v_model.config->>'buffer_pct')::INT,
    (SELECT value->>'buffer_pct' FROM system_config WHERE key = 'defaults')::INT,
    80
  );
  
  -- Check usage windows (simplified - full logic in Go)
  -- This is a fallback for dashboard queries
  
  RETURN jsonb_build_object(
    'available', v_available,
    'reason', v_reason,
    'wait_seconds', v_wait_seconds,
    'buffer_pct', v_buffer_pct
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 13. GRANTS
-- ============================================================================

GRANT SELECT, INSERT, UPDATE ON event_checkpoints TO authenticated;
GRANT SELECT, INSERT, UPDATE ON runner_sessions TO authenticated;
GRANT SELECT, INSERT, UPDATE ON event_queue TO authenticated;
GRANT SELECT, UPDATE ON system_config TO authenticated;

GRANT EXECUTE ON FUNCTION update_event_checkpoint TO authenticated;
GRANT EXECUTE ON FUNCTION get_event_checkpoint TO authenticated;
GRANT EXECUTE ON FUNCTION find_orphaned_sessions TO authenticated;
GRANT EXECUTE ON FUNCTION recover_orphaned_session TO authenticated;
GRANT EXECUTE ON FUNCTION record_model_failure TO authenticated;
GRANT EXECUTE ON FUNCTION record_model_success TO authenticated;
GRANT EXECUTE ON FUNCTION check_model_availability TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE 'Migration 032 complete - Event Persistence & Recovery';
  RAISE NOTICE '  - event_checkpoints table';
  RAISE NOTICE '  - runner_sessions table';
  RAISE NOTICE '  - event_queue table';
  RAISE NOTICE '  - models: usage_windows, learned, consecutive_failures';
  RAISE NOTICE '  - system_config table';
  RAISE NOTICE '  - 8 new RPCs';
END $$;

COMMIT;
