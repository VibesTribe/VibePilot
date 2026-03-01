-- VibePilot Migration 034: Task Improvements
-- Purpose: Add confidence and category fields, plus atomic task creation RPC
-- 
-- Changes:
--   - Add confidence FLOAT to tasks (planner's confidence score)
--   - Add category TEXT to tasks (coding, research, image, testing, etc.)
--   - Add create_task_with_packet RPC for atomic task creation
--
-- Note: categories are freeform, suggestions in config/categories.json

-- Add confidence and category columns
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS confidence FLOAT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS category TEXT;

-- Create index for category queries
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);

-- Create index for confidence-based queries
CREATE INDEX IF NOT EXISTS idx_tasks_confidence ON tasks(confidence);

-- RPC: Create task with packet in single atomic transaction
-- This ensures task and its prompt packet are created together
-- Prevents orphaned tasks or missing packets
CREATE OR REPLACE FUNCTION create_task_with_packet(
  p_plan_id UUID,
  p_task_number TEXT,
  p_title TEXT,
  p_type TEXT,
  p_status TEXT DEFAULT 'pending',
  p_priority INT DEFAULT 5,
  p_confidence FLOAT DEFAULT NULL,
  p_category TEXT DEFAULT NULL,
  p_routing_flag TEXT DEFAULT 'web',
  p_routing_flag_reason TEXT DEFAULT NULL,
  p_dependencies UUID[] DEFAULT '{}',
  p_prompt TEXT,
  p_expected_output TEXT DEFAULT NULL,
  p_context JSONB DEFAULT '{}'
)
RETURNS UUID AS $$
DECLARE
  v_task_id UUID;
BEGIN
  -- Insert into tasks
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
  
  -- Insert into task_packets
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

-- RPC: Record supervisor revision feedback for planner learning
-- Called when supervisor returns needs_revision
CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns JSONB,
  p_tasks_needing_revision TEXT[] DEFAULT '{}'
)
RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
  v_concern TEXT;
BEGIN
  -- Create a learned rule for each concern
  FOR v_concern IN SELECT value::text FROM jsonb_array_elements_text(p_concerns)
  LOOP
    INSERT INTO planner_learned_rules (
      applies_to,
      rule_type,
      rule_text,
      source,
      source_task_id,
      details
    ) VALUES (
      'task_creation',
      'revision_feedback',
      v_concern,
      'supervisor',
      p_plan_id,
      jsonb_build_object(
        'tasks_affected', p_tasks_needing_revision,
        'plan_id', p_plan_id
      )
    )
    RETURNING id INTO v_rule_id;
  END LOOP;
  
  RETURN v_rule_id;
END;
$$ LANGUAGE plpgsql;

-- Verify migration
SELECT 'Migration 034 complete. Tasks table new columns:' AS status;
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'tasks' 
  AND column_name IN ('confidence', 'category');
