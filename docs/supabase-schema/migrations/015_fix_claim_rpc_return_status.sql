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
    UPDATE maintenance_commands
    SET 
        status = 'in_progress',
        executed_by = p_agent_id,
        updated_at = NOW()
    WHERE id = (
        SELECT id 
        FROM maintenance_commands 
        WHERE status = 'pending'
        ORDER BY created_at ASC
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    )
    RETURNING 
        maintenance_commands.id,
        maintenance_commands.command_type,
        maintenance_commands.payload,
        maintenance_commands.status,
        maintenance_commands.idempotency_key,
        maintenance_commands.approved_by;
END;
$$ LANGUAGE plpgsql;
