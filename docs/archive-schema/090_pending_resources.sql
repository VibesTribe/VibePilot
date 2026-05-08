-- Migration: Add pending_resources status support
-- Tasks waiting for resources should be in pending_resources status
-- This RPC finds them so they can be moved back to available when resources free up

CREATE OR REPLACE FUNCTION find_pending_resource_tasks()
RETURNS TABLE (id uuid, task_number text)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT t.id, t.task_number::text
    FROM tasks t
    WHERE t.status = 'pending_resources'
    ORDER BY t.priority ASC, t.created_at ASC
    LIMIT 10;
END;
$$;

-- Add pending_resources to valid task statuses if not already there
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
    CHECK (status IN ('pending', 'available', 'pending_resources', 'in_progress', 'review', 'testing', 'approval', 'awaiting_human', 'merged', 'failed', 'escalated', 'council_review'));

GRANT EXECUTE ON FUNCTION find_pending_resource_tasks() TO service_role;
