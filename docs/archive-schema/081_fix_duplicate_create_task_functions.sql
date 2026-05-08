-- ============================================================================
-- 081_fix_duplicate_create_task_functions.sql
-- Purpose: Drop ALL duplicate create_task_if_not_exists functions and create ONE correct version
-- ============================================================================

-- Drop ALL existing versions of create_task_if_not_exists
DROP FUNCTION IF EXISTS public.create_task_if_not_exists(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);
DROP FUNCTION IF EXISTS public.create_task_if_not_exists(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB);
DROP FUNCTION IF EXISTS public.create_task_if_not_exists(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);
DROP FUNCTION IF EXISTS public.create_task_if_not_exists(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB);

-- Create single correct version matching Go code
CREATE OR REPLACE FUNCTION create_task_if_not_exists(
    p_plan_id UUID,
    p_task_number TEXT,
    p_title TEXT,
    p_type TEXT DEFAULT 'feature',
    p_status TEXT DEFAULT 'available',
    p_priority INT DEFAULT 5,
    p_confidence FLOAT DEFAULT 0.0,
    p_category TEXT DEFAULT 'coding',
    p_slice_id TEXT DEFAULT NULL,
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
    INSERT INTO tasks (
        plan_id,
        task_number,
        title,
        type,
        status,
        priority,
        confidence,
        category,
        slice_id,
        routing_flag,
        routing_flag_reason,
        dependencies,
        max_attempts,
        result,
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
        p_slice_id,
        p_routing_flag,
        p_routing_flag_reason,
        p_dependencies,
        p_max_attempts,
        CASE WHEN p_prompt IS NOT NULL 
             THEN jsonb_build_object('prompt_packet', p_prompt)
             ELSE NULL END,
        NOW(),
        NOW()
    )
    ON CONFLICT (plan_id, task_number) DO NOTHING
    RETURNING id INTO v_task_id;
    
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
'Atomically create a task with prompt packet in tasks.result for dashboard. Returns task ID if created, NULL if already exists.';

GRANT EXECUTE ON FUNCTION create_task_if_not_exists TO service_role;
GRANT EXECUTE ON FUNCTION create_task_if_not_exists TO authenticated;

SELECT 'Migration 081 complete: single create_task_if_not_exists function created' AS status;
