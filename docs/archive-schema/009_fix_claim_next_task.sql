-- Fix: Drop ALL versions of claim_next_task and recreate single version
-- Run in Supabase SQL Editor

-- Drop all versions
DROP FUNCTION IF EXISTS claim_next_task();
DROP FUNCTION IF EXISTS claim_next_task(TEXT);
DROP FUNCTION IF EXISTS claim_next_task(TEXT, TEXT);
DROP FUNCTION IF EXISTS claim_next_task(TEXT, TEXT, TEXT);

-- Recreate single version
CREATE OR REPLACE FUNCTION claim_next_task(
    p_courier TEXT,
    p_platform TEXT,
    p_model_id TEXT
)
RETURNS UUID AS $$
DECLARE
    v_task_id UUID;
BEGIN
    SELECT id INTO v_task_id
    FROM tasks
    WHERE status = 'available'
    AND check_dependencies_complete(id)
    ORDER BY priority ASC, created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED;
    
    IF v_task_id IS NOT NULL THEN
        UPDATE tasks
        SET status = 'in_progress',
            assigned_to = p_model_id,
            updated_at = NOW()
        WHERE id = v_task_id;
        
        RETURN v_task_id;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION claim_next_task(TEXT, TEXT, TEXT) TO authenticated;

-- Verify
SELECT proname, pronargs FROM pg_proc WHERE proname = 'claim_next_task';
