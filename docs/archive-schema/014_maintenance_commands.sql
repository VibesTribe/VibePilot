-- VibePilot Maintenance Commands Table
-- Command queue for Supervisor → Maintenance communication
-- Provides audit trail and enables recovery from crashes

-- Drop if exists (for idempotent migration)
DROP TABLE IF EXISTS maintenance_commands CASCADE;

-- Create command queue table
CREATE TABLE maintenance_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Command specification
    command_type TEXT NOT NULL CHECK (command_type IN (
        'create_branch',      -- Create a new git branch
        'commit_code',        -- Commit code to a branch
        'merge_branch',       -- Merge one branch into another
        'delete_branch',      -- Delete a git branch
        'tag_release'         -- Create a git tag
    )),
    
    -- Command payload (JSONB for flexibility)
    payload JSONB NOT NULL DEFAULT '{}',
    
    -- Example payloads by type:
    -- create_branch: {"branch_name": "task/T001-desc", "base_branch": "main"}
    -- commit_code: {"branch": "task/T001", "files": [{"path": "...", "content": "..."}], "message": "..."}
    -- merge_branch: {"source": "task/T001", "target": "module/user-auth", "delete_source": true}
    -- delete_branch: {"branch_name": "task/T001"}
    -- tag_release: {"tag": "module-user-auth-v1", "target": "main", "message": "..."}
    
    -- Execution status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending',      -- Waiting to be executed
        'in_progress',  -- Currently being executed
        'completed',    -- Successfully executed
        'failed'        -- Execution failed
    )),
    
    -- Idempotency - prevents duplicate execution on retry
    idempotency_key TEXT UNIQUE NOT NULL,
    -- Format: "{command_type}-{task_id}-{timestamp}" or "{command_type}-{uuid}"
    
    -- Approval chain
    approved_by TEXT NOT NULL,  -- Who authorized this command (supervisor session ID)
    approved_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Execution tracking
    executed_by TEXT,           -- Which Maintenance agent executed it
    executed_at TIMESTAMPTZ,    -- When execution completed
    
    -- Result tracking
    result JSONB,               -- Execution result (success details, commit hash, etc.)
    error_message TEXT,         -- Error details if failed
    retry_count INT DEFAULT 0,  -- Number of retry attempts
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_maintenance_commands_status ON maintenance_commands(status);
CREATE INDEX idx_maintenance_commands_type ON maintenance_commands(command_type);
CREATE INDEX idx_maintenance_commands_created ON maintenance_commands(created_at);
CREATE INDEX idx_maintenance_commands_pending ON maintenance_commands(status, created_at) 
    WHERE status = 'pending';

-- Trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_maintenance_commands_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_maintenance_commands_updated_at ON maintenance_commands;
CREATE TRIGGER trigger_maintenance_commands_updated_at
    BEFORE UPDATE ON maintenance_commands
    FOR EACH ROW
    EXECUTE FUNCTION update_maintenance_commands_updated_at();

-- Function to claim next pending command (atomic)
-- Note: If changing return type, must DROP first:
-- DROP FUNCTION IF EXISTS claim_next_command(text);
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

-- Function to complete a command
CREATE OR REPLACE FUNCTION complete_command(
    p_command_id UUID,
    p_success BOOLEAN,
    p_result JSONB DEFAULT NULL,
    p_error_message TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
    UPDATE maintenance_commands
    SET 
        status = CASE WHEN p_success THEN 'completed' ELSE 'failed' END,
        result = p_result,
        error_message = p_error_message,
        executed_at = NOW(),
        updated_at = NOW()
    WHERE id = p_command_id;
END;
$$ LANGUAGE plpgsql;

-- Function to retry a failed command
CREATE OR REPLACE FUNCTION retry_command(p_command_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_retry_count INT;
    v_max_retries INT := 3;
BEGIN
    SELECT retry_count INTO v_retry_count
    FROM maintenance_commands
    WHERE id = p_command_id;
    
    IF v_retry_count >= v_max_retries THEN
        RETURN FALSE;  -- Max retries exceeded
    END IF;
    
    UPDATE maintenance_commands
    SET 
        status = 'pending',
        retry_count = retry_count + 1,
        error_message = NULL,
        updated_at = NOW()
    WHERE id = p_command_id;
    
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your setup)
GRANT SELECT, INSERT, UPDATE ON maintenance_commands TO authenticated;
GRANT EXECUTE ON FUNCTION claim_next_command TO authenticated;
GRANT EXECUTE ON FUNCTION complete_command TO authenticated;
GRANT EXECUTE ON FUNCTION retry_command TO authenticated;

-- Add comment for documentation
COMMENT ON TABLE maintenance_commands IS 
'Command queue for Supervisor → Maintenance communication. Provides audit trail for all git operations.';
