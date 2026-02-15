-- VibePilot Dependency Management RPC Functions
-- Run this in Supabase SQL Editor

-- Function to check if all dependencies are complete
CREATE OR REPLACE FUNCTION check_dependencies_complete(p_task_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_dependencies JSONB;
    v_dep_id UUID;
    v_complete BOOLEAN := TRUE;
BEGIN
    SELECT dependencies INTO v_dependencies
    FROM tasks WHERE id = p_task_id;
    
    IF v_dependencies IS NULL OR jsonb_array_length(v_dependencies) = 0 THEN
        RETURN TRUE;
    END IF;
    
    FOR i IN 0..jsonb_array_length(v_dependencies) - 1 LOOP
        v_dep_id := (v_dependencies->i->>'task_id')::UUID;
        
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

-- Function to unlock tasks when their dependencies are complete
CREATE OR REPLACE FUNCTION unlock_dependent_tasks(p_completed_task_id UUID)
RETURNS TABLE(unlocked_id UUID) AS $$
BEGIN
    -- Find tasks that depend on the completed task and have all deps satisfied
    FOR unlocked_id IN
        SELECT t.id
        FROM tasks t
        WHERE t.status = 'locked'
        AND t.dependencies ?| ARRAY[p_completed_task_id::TEXT]
        AND check_dependencies_complete(t.id)
    LOOP
        UPDATE tasks
        SET status = 'available',
            updated_at = NOW()
        WHERE id = unlocked_id;
        
        RETURN NEXT;
    END LOOP;
    
    RETURN;
END;
$$ LANGUAGE plpgsql;

-- Function to get available tasks (dependencies satisfied)
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

-- Function to claim a task atomically
CREATE OR REPLACE FUNCTION claim_next_task(
    p_courier TEXT,
    p_platform TEXT,
    p_model_id TEXT
)
RETURNS UUID AS $$
DECLARE
    v_task_id UUID;
BEGIN
    -- Find and lock an available task
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

-- Function to make task available when dependency completes (legacy support)
CREATE OR REPLACE FUNCTION make_task_available(p_task_id UUID)
RETURNS VOID AS $$
BEGIN
    -- Check all tasks that might depend on this one
    PERFORM unlock_dependent_tasks(p_task_id);
END;
$$ LANGUAGE plpgsql;

-- Grant execute permissions
GRANT EXECUTE ON FUNCTION check_dependencies_complete(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION unlock_dependent_tasks(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION get_available_tasks() TO authenticated;
GRANT EXECUTE ON FUNCTION claim_next_task(TEXT, TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION make_task_available(UUID) TO authenticated;
