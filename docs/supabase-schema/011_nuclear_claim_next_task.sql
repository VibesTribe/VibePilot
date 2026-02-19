-- Nuclear option: Drop ALL claim_next_task functions by OID
-- Run in Supabase SQL Editor

DO $$
DECLARE
    func_oid OID;
BEGIN
    FOR func_oid IN 
        SELECT p.oid 
        FROM pg_proc p
        JOIN pg_namespace n ON p.pronamespace = n.oid
        WHERE n.nspname = 'public'
        AND p.proname = 'claim_next_task'
    LOOP
        EXECUTE 'DROP FUNCTION IF EXISTS public.claim_next_task CASCADE';
        RAISE NOTICE 'Dropped function OID %', func_oid;
    END LOOP;
END $$;

-- Now recreate single version
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
SELECT count(*) AS function_count FROM pg_proc p
JOIN pg_namespace n ON p.pronamespace = n.oid
WHERE n.nspname = 'public' AND p.proname = 'claim_next_task';
