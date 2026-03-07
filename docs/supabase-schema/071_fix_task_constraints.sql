-- VibePilot Migration 071: Clean up duplicate functions and constraints
-- Purpose: Fix the mess of duplicate RPCs and conflicting constraints
--
-- PROBLEM:
--   1. Multiple versions of create_task_with_packet exist (pre-068, 068, etc)
--   2. Multiple conflicting CHECK constraints on tasks.type and tasks.status
--   3. Self-dependency check uses ANY() which doesn't work with JSONB
--
-- SOLUTION:
--   1. Drop ALL create_task_with_packet functions, recreate ONE correct version
--   2. Drop ALL CHECK constraints on tasks, recreate only what's needed
--   3. Use trigger for self-dependency check (JSONB compatible)

-- ============================================================================
-- PART 1: Fix create_task_with_packet RPC
-- ============================================================================

-- Drop ALL versions of this function
DROP FUNCTION IF EXISTS public.create_task_with_packet(
  p_plan_id uuid, p_task_number text, p_title text, p_type text, p_prompt text,
  p_status text, p_priority integer, p_confidence double precision, p_category text,
  p_routing_flag text, p_routing_flag_reason text, p_dependencies uuid[],
  p_expected_output text, p_context jsonb
);

DROP FUNCTION IF EXISTS public.create_task_with_packet(
  p_plan_id uuid, p_task_number text, p_title text, p_type text, p_status text,
  p_priority integer, p_confidence double precision, p_category text,
  p_routing_flag text, p_routing_flag_reason text, p_dependencies jsonb,
  p_prompt text, p_expected_output text, p_context jsonb, p_max_attempts integer
);

DROP FUNCTION IF EXISTS public.create_task_with_packet(
  p_plan_id uuid, p_task_number text, p_title text, p_type text, p_status text,
  p_priority integer, p_confidence double precision, p_category text,
  p_routing_flag text, p_routing_flag_reason text, p_dependencies jsonb,
  p_prompt text, p_expected_output text, p_context jsonb
);

-- Create ONE correct version matching Go code in validation.go:151-167
CREATE OR REPLACE FUNCTION create_task_with_packet(
  p_plan_id UUID,
  p_task_number TEXT,
  p_title TEXT,
  p_type TEXT,
  p_status TEXT DEFAULT 'pending',
  p_priority INT DEFAULT 5,
  p_confidence FLOAT DEFAULT NULL,
  p_category TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT NULL,
  p_routing_flag_reason TEXT DEFAULT NULL,
  p_dependencies JSONB DEFAULT '[]'::jsonb,
  p_prompt TEXT DEFAULT NULL,
  p_expected_output TEXT DEFAULT NULL,
  p_context JSONB DEFAULT '{}'::jsonb,
  p_max_attempts INT DEFAULT 3
)
RETURNS UUID AS $$
DECLARE
  v_task_id UUID;
BEGIN
  INSERT INTO tasks (
    plan_id, task_number, title, type, status, priority,
    confidence, category, routing_flag, routing_flag_reason,
    dependencies, max_attempts
  ) VALUES (
    p_plan_id, p_task_number, p_title, p_type, p_status, p_priority,
    p_confidence, p_category, p_routing_flag, p_routing_flag_reason,
    p_dependencies, p_max_attempts
  )
  RETURNING id INTO v_task_id;
  
  INSERT INTO task_packets (task_id, prompt, expected_output, context)
  VALUES (v_task_id, p_prompt, p_expected_output, p_context);
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION create_task_with_packet TO service_role;

-- ============================================================================
-- PART 2: Fix tasks table constraints
-- ============================================================================

-- Drop ALL check constraints on tasks table
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname FROM pg_constraint 
              WHERE conrelid = 'public.tasks'::regclass AND contype = 'c')
    LOOP
        EXECUTE 'ALTER TABLE public.tasks DROP CONSTRAINT IF EXISTS ' || r.conname;
    END LOOP;
END $$;

-- Recreate only the constraints we actually need

ALTER TABLE tasks ADD CONSTRAINT tasks_priority_check 
  CHECK (priority BETWEEN 1 AND 10);

ALTER TABLE tasks ADD CONSTRAINT tasks_type_check 
  CHECK (type IN ('feature','bug','fix','test','refactor','lint','typecheck','visual','accessibility','docs','setup'));

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN ('pending','available','in_progress','review','testing','approval','merged','escalated','blocked'));

ALTER TABLE tasks ADD CONSTRAINT tasks_confidence_check 
  CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1));

ALTER TABLE tasks ADD CONSTRAINT tasks_max_attempts_check 
  CHECK (max_attempts > 0 AND max_attempts <= 10);

-- ============================================================================
-- PART 3: Self-dependency check via trigger (JSONB compatible)
-- ============================================================================

CREATE OR REPLACE FUNCTION check_no_self_dependency()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.dependencies::jsonb ? NEW.id::text THEN
    RAISE EXCEPTION 'Task cannot depend on itself';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_no_self_dependency ON tasks;
CREATE TRIGGER trg_no_self_dependency
  BEFORE INSERT OR UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION check_no_self_dependency();

SELECT 'Migration 071 complete: cleaned up duplicates and constraints' AS status;
