-- Drop both claim_next_task versions (3 and 4 args) and recreate single version
-- Run in Supabase SQL Editor

-- Drop the 3-arg version
DROP FUNCTION IF EXISTS public.claim_next_task(text, text, text);

-- Drop the 4-arg version
DROP FUNCTION IF EXISTS public.claim_next_task(text, text, text, text);

-- Also try with character varying
DROP FUNCTION IF EXISTS public.claim_next_task(character varying, character varying, character varying);
DROP FUNCTION IF EXISTS public.claim_next_task(character varying, character varying, character varying, character varying);

-- Recreate single version with 3 args
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

-- Verify only one exists
SELECT p.proname AS name, pg_get_function_identity_arguments(p.oid) AS args
FROM pg_proc p
JOIN pg_namespace n ON p.pronamespace = n.oid
WHERE n.nspname = 'public' AND p.proname = 'claim_next_task';
