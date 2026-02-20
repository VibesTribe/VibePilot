-- Fix: Add status to claim_next_command return
-- Run this to update the function signature

DROP FUNCTION IF EXISTS claim_next_command(text);

CREATE OR REPLACE FUNCTION claim_next_command(p_agent_id TEXT)
RETURNS TABLE (
    command_id UUID,
    command_type TEXT,
    payload JSONB,
    status TEXT,
    idempotency_key TEXT,
    approved_by TEXT
) AS $$
BEGIN
    RETURN QUERY
    UPDATE maintenance_commands mc
    SET 
        mc.status = 'in_progress',
        mc.executed_by = p_agent_id,
        mc.updated_at = NOW()
    WHERE mc.id = (
        SELECT sub.id 
        FROM maintenance_commands sub
        WHERE sub.status = 'pending'
        ORDER BY sub.created_at ASC
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    )
    RETURNING 
        mc.id,
        mc.command_type,
        mc.payload,
        mc.status,
        mc.idempotency_key,
        mc.approved_by;
END;
$$ LANGUAGE plpgsql;
