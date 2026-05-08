-- VibePilot Migration 073: Fix create_task_with_packet to write to tasks.result
-- Purpose: Dashboard expects tasks.result.prompt_packet but RPC only wrote to task_packets table
--
-- Problem: Dashboard reads from tasks.result.prompt_packet, but RPC only wrote to task_packets table
-- Solution: Write to BOTH locations - task_packets for versioning, tasks.result for dashboard

DROP FUNCTION IF EXISTS public.create_task_with_packet(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);
DROP FUNCTION IF EXISTS public.create_task_with_packet(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB);
DROP FUNCTION IF EXISTS public.create_task_with_packet(UUID, TEXT, TEXT, TEXT, TEXT, INT, FLOAT, TEXT, TEXT, TEXT, JSONB, TEXT, TEXT, JSONB, INT);

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
    dependencies, max_attempts,
    result
  ) VALUES (
    p_plan_id, p_task_number, p_title, p_type, p_status, p_priority,
    p_confidence, p_category, p_routing_flag, p_routing_flag_reason,
    p_dependencies, p_max_attempts,
    jsonb_build_object(
      'prompt_packet', p_prompt,
      'expected_output', p_expected_output,
      'context', p_context
    )
  )
  RETURNING id INTO v_task_id;
  
  INSERT INTO task_packets (task_id, prompt, expected_output, context)
  VALUES (v_task_id, p_prompt, p_expected_output, p_context);
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION create_task_with_packet TO service_role;

SELECT 'Migration 073 complete: RPC now writes to tasks.result for dashboard' AS status;
