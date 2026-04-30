-- VibePilot Migration 068: Fix create_task_with_packet RPC
-- Purpose: Fix parameter order and types to match Go code expectations
--
-- Changes:
--   - Reorder parameters to match Go code call order
--   - Change p_dependencies from UUID[] to JSONB (JSONB everywhere principle)
--   - Add p_max_attempts parameter
--   - Keep backward compatibility by using defaults

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
  p_dependencies JSONB DEFAULT '[]',
  p_prompt TEXT DEFAULT NULL,
  p_expected_output TEXT DEFAULT NULL,
  p_context JSONB DEFAULT '{}',
  p_max_attempts INT DEFAULT 3
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
    max_attempts
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
    p_max_attempts
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

SELECT 'Migration 068 complete: create_task_with_packet RPC fixed' AS status;
