-- Fix RPC functions to handle double-quoted UUIDs
-- The data has embedded quotes that need stripping

BEGIN;

DROP FUNCTION IF EXISTS check_dependencies_complete(UUID);

CREATE OR REPLACE FUNCTION check_dependencies_complete(p_task_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_dependencies JSONB;
    v_dep_text TEXT;
    v_dep_id UUID;
    v_complete BOOLEAN := TRUE;
BEGIN
    SELECT dependencies INTO v_dependencies
    FROM tasks WHERE id = p_task_id;
    
    IF v_dependencies IS NULL OR jsonb_array_length(v_dependencies) = 0 THEN
        RETURN TRUE;
    END IF;
    
    FOR i IN 0..jsonb_array_length(v_dependencies) - 1 LOOP
        -- Get as text and strip any embedded quotes
        v_dep_text := v_dependencies->i#>>'{}';
        v_dep_text := trim(BOTH '"' FROM v_dep_text);
        
        BEGIN
            v_dep_id := v_dep_text::UUID;
        EXCEPTION WHEN OTHERS THEN
            -- Invalid UUID format, skip
            CONTINUE;
        END;
        
        IF NOT EXISTS (
            SELECT 1 FROM tasks 
            WHERE id = v_dep_id AND status = 'merged'
        ) THEN
            v_complete := FALSE;
            EXIT;
        END IF;
    END LOOP;
    
    RETURN v_complete;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION IF EXISTS get_available_tasks();
CREATE OR REPLACE FUNCTION get_available_tasks()
RETURNS TABLE(
    id UUID,
    title TEXT,
    type TEXT,
    priority INTEGER,
    dependencies JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT t.id, t.title, t.type, t.priority, t.dependencies
    FROM tasks t
    WHERE t.status = 'available'
    AND check_dependencies_complete(t.id)
    ORDER BY t.priority ASC, t.created_at ASC;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION IF EXISTS claim_next_task(TEXT, TEXT, TEXT);
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

DROP FUNCTION IF EXISTS get_available_for_routing(BOOLEAN, BOOLEAN, BOOLEAN);
CREATE OR REPLACE FUNCTION get_available_for_routing(
    p_can_web BOOLEAN DEFAULT TRUE,
    p_can_internal BOOLEAN DEFAULT TRUE,
    p_can_mcp BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(
    id UUID,
    title TEXT,
    routing_flag TEXT,
    priority INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT t.id, t.title, t.routing_flag, t.priority
    FROM tasks t
    WHERE t.status = 'available'
    AND check_dependencies_complete(t.id)
    AND (
        (p_can_web AND t.routing_flag = 'web')
        OR (p_can_internal AND t.routing_flag = 'internal')
        OR (p_can_mcp AND t.routing_flag = 'mcp')
        OR t.routing_flag IS NULL
    )
    ORDER BY t.priority ASC, t.created_at ASC;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION check_dependencies_complete(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION get_available_tasks() TO authenticated;
GRANT EXECUTE ON FUNCTION claim_next_task(TEXT, TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_available_for_routing(BOOLEAN, BOOLEAN, BOOLEAN) TO authenticated;

COMMIT;

-- Verify
SELECT check_dependencies_complete('e0a3d041-be6d-48a1-b8aa-42334269da6d'::uuid);
