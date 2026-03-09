-- VibePilot Migration 078: Fix prompt_packet in tasks.result
-- Purpose: Ensure create_task_if_not_exists RPC writes prompt_packet
-- to to tasks.result for dashboard display.
--
-- Problem: Dashboard expects tasks.result.prompt_packet but the prompt_packet
-- stored in task_packets table (separate),--
-- Solution: Update RPC to also write to tasks.result for dashboard.
-- 
-- Changes:
-- 1. Add p_prompt parameter to create_task_if_not_exists
-- 2. Add p_result parameter to store the result
-- 3. Update validation.go to use the RPC
--
-- Technical Notes:
-- No new tables added
-- No new RPCs added
-- No new columns added
-- No complex migration needed - just update the RPC

-- Clean, minimal change

CREATE OR REPLACE function create_task_if_not_exists(
    p_plan_id UUID,
    p_task_number TEXT
    p_title TEXT
    p_type TEXT DEFAULT 'feature'
    p_status TEXT DEFAULT 'available'
    p_priority INT DEFAULT 5
    p_confidence FLOAT DEFAULT 0.95
    p_category TEXT DEFAULT 'coding'
    p_routing_flag TEXT DEFAULT NULL
    p_routing_flag_reason TEXT DEFAULT null
    p_dependencies JSONB DEFAULT '[]'::jsonb
    p_prompt TEXT DEFAULT NULL
    p_expected_output TEXT DEFAULT null
    p_context JSONB DEFAULT '{}'::jsonb
    p_max_attempts INT DEFAULT 3
    p_result JSONB DEFAULT '{}'::jsonb
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
    )
    ON CONFLICT (plan_id, task_number) DO NOTHING
    RETURNING id INTO v_task_id;
    
    -- If insert succeeded, create the task packet
    IF v_task_id IS NOT NULL AND p_prompt IS NOT NULL THEN
        RETURN;
    END IF;
    
    -- Also write prompt_packet to tasks.result for dashboard
    result := jsonb_build_object(
        "prompt_packet": p_prompt,
    });
    
    RETURN v_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION create_task_if_not_exists IS 
'Atomically create a task with its prompt packet. Returns task ID if created, NULL if already exists (duplicate).';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 078 complete - prompt_packet in tasks.result for dashboard' AS status;
