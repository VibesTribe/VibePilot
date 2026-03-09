-- VibePilot Migration 077: Prevent Duplicate Tasks
-- Purpose: Add unique constraint on (plan_id, task_number) and create atomic
-- task creation RPC to prevent duplicate tasks from race conditions.
--
-- Problem: Multiple realtime events can fire simultaneously for plan approval,
-- causing createTasksFromApprovedPlan to run multiple times and create
-- duplicate tasks with the same task_number.
--
-- Solution:
-- 1. Add unique constraint on (plan_id, task_number)
-- 2. Create atomic create_task_if_not_exists RPC using ON CONFLICT

-- ============================================================================
-- UNIQUE CONSTRAINT (skip if already exists)
-- ============================================================================

-- Clean up any existing duplicates (keep the oldest one)
DELETE FROM tasks t1
WHERE EXISTS (
    SELECT 1 FROM tasks t2
    WHERE t2.plan_id = t1.plan_id
      AND t2.task_number = t1.task_number
      AND t2.created_at < t1.created_at
);

-- Add unique constraint (will fail silently if exists via DO NOTHING pattern)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'tasks_plan_id_task_number_key'
    ) THEN
        ALTER TABLE tasks ADD CONSTRAINT tasks_plan_id_task_number_key UNIQUE (plan_id, task_number);
    END IF;
END $$;

-- ============================================================================
-- ATOMIC TASK CREATION RPC
-- ============================================================================

CREATE OR REPLACE FUNCTION create_task_if_not_exists(
    p_plan_id UUID,
    p_task_number TEXT,
    p_title TEXT,
    p_type TEXT DEFAULT 'feature',
    p_status TEXT DEFAULT 'available',
    p_priority INT DEFAULT 5,
    p_confidence FLOAT DEFAULT 0.95,
    p_category TEXT DEFAULT 'coding',
    p_routing_flag TEXT DEFAULT NULL,
    p_routing_flag_reason TEXT DEFAULT NULL,
    p_dependencies JSONB DEFAULT '[]'::jsonb,
    p_prompt TEXT DEFAULT NULL,
    p_expected_output TEXT DEFAULT NULL,
    p_context JSONB DEFAULT '{}'::jsonb,
    p_max_attempts INT DEFAULT 3
) RETURNS UUID AS $$
DECLARE
    v_task_id UUID;
BEGIN
    -- Atomic insert with ON CONFLICT - returns existing ID if duplicate
    INSERT INTO tasks (
        plan_id,
        task_number,
        title,
        type,
        status,
        priority,
        confidence,
        category,
        routing_flag,
        routing_flag_reason,
        dependencies,
        max_attempts,
        created_at,
        updated_at
    ) VALUES (
        p_plan_id,
        p_task_number,
        p_title,
        p_type,
        p_status,
        p_priority,
        p_confidence,
        p_category,
        p_routing_flag,
        p_routing_flag_reason,
        p_dependencies,
        p_max_attempts,
        NOW(),
        NOW()
    )
    ON CONFLICT (plan_id, task_number) DO NOTHING
    RETURNING id INTO v_task_id;
    
    -- If insert succeeded, create the task packet
    IF v_task_id IS NOT NULL AND p_prompt IS NOT NULL THEN
        INSERT INTO task_packets (
            task_id,
            prompt,
            expected_output,
            context,
            version,
            created_at
        ) VALUES (
            v_task_id,
            p_prompt,
            p_expected_output,
            p_context,
            1,
            NOW()
        );
    END IF;
    
    RETURN v_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION create_task_if_not_exists IS 
'Atomically create a task with its packet. Returns task ID if created, NULL if already exists (duplicate).';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 077 complete - unique constraint and atomic task creation' AS status;

-- Verify constraint exists
SELECT conname FROM pg_constraint 
WHERE conrelid = 'tasks'::regclass AND conname = 'tasks_plan_id_task_number_key';
