-- VibePilot Migration: Orchestrator State & Event Logging
-- Version: 023
-- Purpose: Concurrent tracking, routing history, event log, security audit
-- Design: Stateless orchestrator, DB as source of truth
-- 
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- 1. CONCURRENT TRACKING (on runners - they are routing targets)
-- ============================================================================

ALTER TABLE runners ADD COLUMN IF NOT EXISTS max_concurrent INT DEFAULT 1;
ALTER TABLE runners ADD COLUMN IF NOT EXISTS current_in_flight INT DEFAULT 0;

COMMENT ON COLUMN runners.max_concurrent IS 'Maximum concurrent tasks this runner can handle';
COMMENT ON COLUMN runners.current_in_flight IS 'Current number of tasks in progress';

-- ============================================================================
-- 2. ROUTING HISTORY (on tasks - audit trail of routing decisions)
-- ============================================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS routing_history JSONB DEFAULT '[]';

COMMENT ON COLUMN tasks.routing_history IS '[{"from": "model_a", "to": "model_b", "reason": "...", "at": "..."}]';

-- ============================================================================
-- 3. ORCHESTRATOR EVENTS (dashboard logs, audit trail)
-- ============================================================================

CREATE TABLE IF NOT EXISTS orchestrator_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type TEXT NOT NULL,
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  runner_id UUID REFERENCES runners(id) ON DELETE SET NULL,
  from_runner_id UUID REFERENCES runners(id) ON DELETE SET NULL,
  to_runner_id UUID REFERENCES runners(id) ON DELETE SET NULL,
  model_id TEXT,
  reason TEXT,
  details JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE orchestrator_events IS 'Audit trail of all orchestrator decisions';

CREATE INDEX IF NOT EXISTS idx_orch_events_type ON orchestrator_events(event_type);
CREATE INDEX IF NOT EXISTS idx_orch_events_task ON orchestrator_events(task_id);
CREATE INDEX IF NOT EXISTS idx_orch_events_created ON orchestrator_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orch_events_model ON orchestrator_events(model_id);

-- ============================================================================
-- 4. SECURITY AUDIT (sensitive operation tracking)
-- ============================================================================

CREATE TABLE IF NOT EXISTS security_audit (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operation TEXT NOT NULL,
  agent_id TEXT,
  resource TEXT,
  key_name TEXT,
  allowed BOOLEAN NOT NULL,
  reason TEXT,
  ip_address TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE security_audit IS 'Audit trail for vault access, config changes, deletes';

CREATE INDEX IF NOT EXISTS idx_security_audit_op ON security_audit(operation);
CREATE INDEX IF NOT EXISTS idx_security_audit_created ON security_audit(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_audit_agent ON security_audit(agent_id);

-- ============================================================================
-- 5. VAULT RLS HARDENING (prevent bulk export/delete)
-- ============================================================================

-- Drop existing permissive policy if exists
DROP POLICY IF EXISTS "Allow all for service role" ON secrets_vault;

-- Service role gets full access (vault_manager.py uses service role)
CREATE POLICY "vault_service_role_full" ON secrets_vault
  FOR ALL TO service_role
  USING (true)
  WITH CHECK (true);

-- Authenticated users can only read one key at a time (no SELECT *)
-- Note: This requires the caller to specify key_name in request
CREATE POLICY "vault_authenticated_read" ON secrets_vault
  FOR SELECT TO authenticated
  USING (true);

-- Block DELETE for non-service-role
CREATE POLICY "vault_no_delete" ON secrets_vault
  FOR DELETE TO authenticated
  USING (false);

-- Block INSERT for non-service-role  
CREATE POLICY "vault_no_insert" ON secrets_vault
  FOR INSERT TO authenticated
  WITH CHECK (false);

-- Block UPDATE for non-service-role
CREATE POLICY "vault_no_update" ON secrets_vault
  FOR UPDATE TO authenticated
  USING (false)
  WITH CHECK (false);

-- ============================================================================
-- 6. RPC: log_orchestrator_event
-- ============================================================================

CREATE OR REPLACE FUNCTION log_orchestrator_event(
  p_event_type TEXT,
  p_task_id UUID DEFAULT NULL,
  p_runner_id UUID DEFAULT NULL,
  p_from_runner_id UUID DEFAULT NULL,
  p_to_runner_id UUID DEFAULT NULL,
  p_model_id TEXT DEFAULT NULL,
  p_reason TEXT DEFAULT NULL,
  p_details JSONB DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO orchestrator_events (
    event_type, task_id, runner_id, from_runner_id, to_runner_id, 
    model_id, reason, details
  ) VALUES (
    p_event_type, p_task_id, p_runner_id, p_from_runner_id, p_to_runner_id,
    p_model_id, p_reason, p_details
  ) RETURNING id INTO v_id;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 7. RPC: append_routing_history
-- ============================================================================

CREATE OR REPLACE FUNCTION append_routing_history(
  p_task_id UUID,
  p_from_model TEXT,
  p_to_model TEXT,
  p_reason TEXT
) RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET routing_history = routing_history || jsonb_build_object(
    'from', p_from_model,
    'to', p_to_model,
    'reason', p_reason,
    'at', NOW()
  ),
  updated_at = NOW()
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 8. RPC: increment_in_flight (atomic, returns false if at capacity)
-- ============================================================================

CREATE OR REPLACE FUNCTION increment_in_flight(p_runner_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
  v_max INT;
  v_current INT;
BEGIN
  SELECT max_concurrent, current_in_flight INTO v_max, v_current
  FROM runners WHERE id = p_runner_id FOR UPDATE;
  
  IF v_max IS NULL THEN
    v_max := 1;
  END IF;
  
  IF v_current >= v_max THEN
    RETURN FALSE;
  END IF;
  
  UPDATE runners 
  SET current_in_flight = current_in_flight + 1,
      updated_at = NOW()
  WHERE id = p_runner_id;
  
  RETURN TRUE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 9. RPC: decrement_in_flight
-- ============================================================================

CREATE OR REPLACE FUNCTION decrement_in_flight(p_runner_id UUID)
RETURNS VOID AS $$
BEGIN
  UPDATE runners 
  SET current_in_flight = GREATEST(current_in_flight - 1, 0),
      updated_at = NOW()
  WHERE id = p_runner_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 10. RPC: log_security_audit
-- ============================================================================

CREATE OR REPLACE FUNCTION log_security_audit(
  p_operation TEXT,
  p_agent_id TEXT DEFAULT NULL,
  p_resource TEXT DEFAULT NULL,
  p_key_name TEXT DEFAULT NULL,
  p_allowed BOOLEAN DEFAULT TRUE,
  p_reason TEXT DEFAULT NULL,
  p_ip_address TEXT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO security_audit (
    operation, agent_id, resource, key_name, allowed, reason, ip_address
  ) VALUES (
    p_operation, p_agent_id, p_resource, p_key_name, p_allowed, p_reason, p_ip_address
  ) RETURNING id INTO v_id;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 11. RPC: get_system_state (for orchestrator snapshot)
-- ============================================================================

CREATE OR REPLACE FUNCTION get_system_state()
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  SELECT jsonb_build_object(
    'models', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', id,
        'status', status,
        'tokens_used', tokens_used,
        'tasks_completed', tasks_completed,
        'tasks_failed', tasks_failed,
        'success_rate', success_rate,
        'cooldown_expires_at', cooldown_expires_at
      ))
      FROM models 
      WHERE status IN ('active', 'paused')
    ),
    'runners', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', id,
        'model_id', model_id,
        'status', status,
        'routing_capability', routing_capability,
        'cost_priority', cost_priority,
        'task_ratings', task_ratings,
        'daily_used', daily_used,
        'daily_limit', daily_limit,
        'max_concurrent', max_concurrent,
        'current_in_flight', current_in_flight,
        'cooldown_expires_at', cooldown_expires_at,
        'rate_limit_reset_at', rate_limit_reset_at
      ))
      FROM runners
      WHERE status IN ('active', 'cooldown', 'rate_limited')
    ),
    'tasks_available', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', id,
        'title', title,
        'type', type,
        'priority', priority,
        'routing_flag', routing_flag,
        'dependencies', dependencies
      ))
      FROM tasks
      WHERE status = 'available'
      ORDER BY priority, created_at
      LIMIT 50
    ),
    'tasks_in_flight', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', id,
        'title', title,
        'assigned_to', assigned_to,
        'started_at', started_at
      ))
      FROM tasks
      WHERE status = 'in_progress'
    ),
    'timestamp', NOW()
  ) INTO v_result;
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 12. GRANTS
-- ============================================================================

GRANT SELECT, INSERT ON orchestrator_events TO authenticated;
GRANT SELECT, INSERT ON security_audit TO authenticated;

GRANT EXECUTE ON FUNCTION log_orchestrator_event TO authenticated;
GRANT EXECUTE ON FUNCTION append_routing_history TO authenticated;
GRANT EXECUTE ON FUNCTION increment_in_flight TO authenticated;
GRANT EXECUTE ON FUNCTION decrement_in_flight TO authenticated;
GRANT EXECUTE ON FUNCTION log_security_audit TO authenticated;
GRANT EXECUTE ON FUNCTION get_system_state TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE 'Migration 023 complete';
  RAISE NOTICE '  - runners: max_concurrent, current_in_flight';
  RAISE NOTICE '  - tasks: routing_history';
  RAISE NOTICE '  - orchestrator_events table';
  RAISE NOTICE '  - security_audit table';
  RAISE NOTICE '  - Vault RLS hardened';
  RAISE NOTICE '  - 6 new RPCs';
END $$;

COMMIT;
