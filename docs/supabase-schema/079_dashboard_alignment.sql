-- VibePilot Migration 079: Dashboard Alignment
-- Purpose: Fix create_task_if_not_exists RPC to match dashboard expectations
--
-- Dashboard Expects (from vibepilotAdapter.ts):
--   - tasks.result.prompt_packet (jsonb field) - for displaying prompts
--   - tasks.slice_id (text) - for grouping tasks into slices
--   - tasks.status values: pending, available, in_progress, review, testing, approval, merged
--
-- This migration:
-- 1. Adds p_slice_id parameter to create_task_if_not_exists RPC
-- 2. Writes prompt_packet to tasks.result for dashboard display
-- 3. Ensures tasks table has required columns
--
-- Replaces broken migration 078

-- ============================================================================

-- DROP OLD/BROKEN FUNCTIONS
-- ============================================================================

DROP FUNCTION IF EXISTS public.create_task_if_not_exists(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);
DROP FUNCTION IF EXISTS public.create_task_with_packet(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);

-- ============================================================================
-- ENSURE tasks.result COLUMN EXISTS (jsonb)
-- ============================================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tasks' AND column_name = 'result'
    ) THEN
        ALTER TABLE tasks ADD COLUMN result JSONB DEFAULT '{}'::jsonb;
    END IF;
END $$;

-- ============================================================================
-- ENSURE tasks.slice_id COLUMN EXISTS (text)
-- ============================================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tasks' AND column_name = 'slice_id'
    ) THEN
        ALTER TABLE tasks ADD COLUMN slice_id TEXT DEFAULT 'general';
    END IF;
END $$;

-- ============================================================================
-- FIXED RPC: create_task_if_not_exists
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
    p_slice_id TEXT DEFAULT 'general',
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
        jsonb_build_object(
            'prompt_packet', p_prompt,
            'expected_output', p_expected_output,
            'context', p_context
        ),
        NOW(),
        NOW()
    )
    ON CONFLICT (plan_id, task_number) DO NOTHING
    RETURNING id INTO v_task_id;
    
    -- If insert succeeded, also create the task packet (for versioning)
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
        )
        ON CONFLICT (task_id) DO NOTHING;
    END IF;
    
    RETURN v_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION create_task_if_not_exists IS 
'Atomically create a task with prompt packet in tasks.result for dashboard. Returns task ID if created, NULL if already exists.';

-- ============================================================================
-- GRANT PERMISSIONS
-- ============================================================================

GRANT EXECUTE ON FUNCTION create_task_if_not_exists TO service_role;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 079 complete - dashboard alignment fixed' AS status;

-- Verify columns exist
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'tasks' 
  AND column_name IN ('result', 'slice_id')
ORDER BY column_name;
