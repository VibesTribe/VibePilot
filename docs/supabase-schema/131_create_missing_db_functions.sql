-- Migration 131: Create missing DB functions referenced by Go code
-- get_change_approvals: used by maintenance.CheckApprovalChain
-- queue_maintenance_command: used by tools.db_tools
-- These are stubs that return safe defaults until the full tables are built.

-- 1. get_change_approvals: Returns change approvals for a given change ID
-- Called with p_change_id TEXT, returns rows with approver/approved fields
-- Since there's no change_approvals table yet, return empty set
CREATE OR REPLACE FUNCTION get_change_approvals(p_change_id TEXT)
RETURNS TABLE(approver TEXT, approved BOOLEAN, approved_at TIMESTAMPTZ) AS $$
BEGIN
  -- No change_approvals table exists yet.
  -- Return empty result set — maintenance.CheckApprovalChain will see
  -- no approvals and require them via its own requiredApprovals() logic.
  RETURN QUERY SELECT NULL::TEXT, NULL::BOOLEAN, NULL::TIMESTAMPTZ WHERE FALSE;
END;
$$ LANGUAGE plpgsql;

-- 2. queue_maintenance_command: Queues a command for the maintenance handler
-- Called with p_command TEXT, p_params JSONB
-- Returns the created command record
CREATE OR REPLACE FUNCTION queue_maintenance_command(
  p_command TEXT,
  p_params JSONB DEFAULT '{}'::jsonb
)
RETURNS JSONB AS $$
DECLARE
  v_id UUID := gen_random_uuid();
  v_key TEXT := 'mq_' || to_char(NOW(), 'YYYYMMDDHH24MISS') || '_' || replace(v_id::text, '-', '');
  v_result JSONB;
BEGIN
  INSERT INTO maintenance_commands (
    id, command_type, payload, status, idempotency_key, approved_by
  ) VALUES (
    v_id,
    p_command,
    COALESCE(p_params, '{}'::jsonb),
    'pending',
    v_key,
    'auto'
  )
  ON CONFLICT (idempotency_key) DO NOTHING
  RETURNING jsonb_build_object(
    'id', id,
    'command_type', command_type,
    'status', status,
    'idempotency_key', idempotency_key
  ) INTO v_result;

  -- If conflict (duplicate key), fetch existing
  IF v_result IS NULL THEN
    SELECT jsonb_build_object(
      'id', id,
      'command_type', command_type,
      'status', status,
      'idempotency_key', idempotency_key
    ) INTO v_result
    FROM maintenance_commands
    WHERE idempotency_key = v_key;
  END IF;

  RETURN COALESCE(v_result, '{}'::jsonb);
END;
$$ LANGUAGE plpgsql;
