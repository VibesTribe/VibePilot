-- VibePilot Migration: UUID[] → JSONB for dependencies
-- 
-- WHY: RPC functions expect JSONB, but column was UUID[]
--       JSONB is more extensible, self-documenting, future-proof
--
-- SAFE: Converts existing data in place
--       ["uuid1", "uuid2"] → ["uuid1", "uuid2"] (same content, new type)
--
-- Run this ONCE in Supabase SQL Editor

BEGIN;

-- 1. Migrate dependencies column to JSONB
ALTER TABLE tasks 
ALTER COLUMN dependencies TYPE JSONB 
USING CASE 
  WHEN dependencies IS NULL THEN '[]'::jsonb
  ELSE to_jsonb(dependencies)
END;

-- 2. Update the GIN index for JSONB
DROP INDEX IF EXISTS idx_tasks_deps;
CREATE INDEX idx_tasks_deps ON tasks USING GIN(dependencies);

-- 3. Fix check_dependencies_complete to work with JSONB array of UUIDs
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
        -- Handle both string UUIDs and objects with task_id
        IF jsonb_typeof(v_dependencies->i) = 'object' THEN
            v_dep_id := (v_dependencies->i->>'task_id')::UUID;
        ELSE
            v_dep_id := (v_dependencies->i)::TEXT::UUID;
        END IF;
        
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

-- 4. Fix unlock_dependent_tasks to work with JSONB
CREATE OR REPLACE FUNCTION unlock_dependent_tasks(p_completed_task_id UUID)
RETURNS TABLE(unlocked_id UUID, unlocked_title TEXT) AS $$
DECLARE
    v_task RECORD;
    v_deps JSONB;
    v_dep_id UUID;
    v_has_dep BOOLEAN;
BEGIN
    FOR v_task IN 
        SELECT id, title, dependencies 
        FROM tasks 
        WHERE status = 'locked'
    LOOP
        v_deps := v_task.dependencies;
        v_has_dep := FALSE;
        
        IF v_deps IS NOT NULL AND jsonb_array_length(v_deps) > 0 THEN
            FOR i IN 0..jsonb_array_length(v_deps) - 1 LOOP
                -- Handle both string UUIDs and objects with task_id
                IF jsonb_typeof(v_deps->i) = 'object' THEN
                    v_dep_id := (v_deps->i->>'task_id')::UUID;
                ELSE
                    v_dep_id := (v_deps->i)::TEXT::UUID;
                END IF;
                
                IF v_dep_id = p_completed_task_id THEN
                    v_has_dep := TRUE;
                    EXIT;
                END IF;
            END LOOP;
        END IF;
        
        -- If this task depends on completed task AND all deps are satisfied
        IF v_has_dep AND check_dependencies_complete(v_task.id) THEN
            UPDATE tasks
            SET status = 'available',
                updated_at = NOW()
            WHERE id = v_task.id;
            
            unlocked_id := v_task.id;
            unlocked_title := v_task.title;
            RETURN NEXT;
        END IF;
    END LOOP;
    
    RETURN;
END;
$$ LANGUAGE plpgsql;

-- 5. Fix get_available_tasks return type
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

-- 6. Fix claim_next_task
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

-- 7. Add routing-aware task fetch (was missing)
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

-- 8. Grant permissions
GRANT EXECUTE ON FUNCTION check_dependencies_complete(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION unlock_dependent_tasks(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION get_available_tasks() TO authenticated;
GRANT EXECUTE ON FUNCTION claim_next_task(TEXT, TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_available_for_routing(BOOLEAN, BOOLEAN, BOOLEAN) TO authenticated;

-- 9. Add locked status to check constraint if not exists
-- (Note: May need to drop/recreate constraint if locked not included)

COMMIT;

-- Verify migration
SELECT 
    column_name, 
    data_type, 
    udt_name 
FROM information_schema.columns 
WHERE table_name = 'tasks' 
AND column_name = 'dependencies';
