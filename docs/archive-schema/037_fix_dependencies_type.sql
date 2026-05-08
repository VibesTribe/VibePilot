-- VibePilot Migration 037: Fix create_task_with_packet dependencies type
-- Purpose: Change p_dependencies from UUID[] to JSONB to match tasks table

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
    dependencies
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
    p_dependencies
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

SELECT 'Migration 037 complete - create_task_with_packet now uses JSONB for dependencies' AS status;
