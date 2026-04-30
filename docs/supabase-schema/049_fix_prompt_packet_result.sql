-- VibePilot Migration 049: Fix prompt_packet in tasks.result
-- Purpose: Ensure create_task_with_packet populates tasks.result with prompt_packet
-- 
-- Problem: Dashboard expects task.result.prompt_packet but RPC doesn't populate it
-- Solution: Update create_task_with_packet to set result column

CREATE OR REPLACE FUNCTION create_task_with_packet(
  p_plan_id UUID,
  p_task_number TEXT,
  p_title TEXT,
  p_type TEXT,
  p_prompt TEXT,
  p_status TEXT DEFAULT 'pending',
  p_priority INT DEFAULT 5,
  p_confidence FLOAT DEFAULT NULL,
  p_category TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT 'web',
  p_routing_flag_reason TEXT DEFAULT NULL,
  p_dependencies JSONB DEFAULT '[]',
  p_expected_output TEXT DEFAULT NULL,
  p_context JSONB DEFAULT '{}'
)
RETURNS UUID AS $$
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
    routing_flag,
    routing_flag_reason,
    dependencies,
    result
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
    jsonb_build_object('prompt_packet', p_prompt)
  )
  RETURNING id INTO v_task_id;
  
  INSERT INTO task_packets (
    task_id,
    prompt,
    expected_output,
    context
  ) VALUES (
    v_task_id,
    p_prompt,
    p_expected_output,
    p_context
  );
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION create_task_with_packet IS 'Create task with prompt packet. Populates both tasks.result.prompt_packet (for dashboard) and task_packets table (for orchestrator).';

SELECT 'Migration 049 complete - prompt_packet now in tasks.result' AS status;
