-- VibePilot Migration 050: Add State Tracking and Revision History
-- Purpose: Enable proper state-based recovery and revision tracking
-- 
-- Problems this solves:
-- 1. Processing claims timeout-based (now state-based)
-- 2. Revision history not tracked (now tracked)
-- 3. Error states permanent (now recoverable)
-- 4. No performance metrics (now tracked)

-- ============================================================================
-- PART 1: Add revision tracking to plans
-- ============================================================================

ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_round INT DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_history JSONB DEFAULT '[]'::jsonb;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS latest_feedback JSONB;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS tasks_needing_revision TEXT[] DEFAULT '{}';

COMMENT ON COLUMN plans.revision_round IS 'Current revision round (0 = initial, increments on each revision)';
COMMENT ON COLUMN plans.revision_history IS 'Array of revision records with round, concerns, suggestions, timestamp';
COMMENT ON COLUMN plans.latest_feedback IS 'Most recent feedback from supervisor/council';
COMMENT ON COLUMN plans.tasks_needing_revision IS 'Task numbers that need revision in current round';

-- ============================================================================
-- PART 2: Add retry tracking to tasks
-- ============================================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

COMMENT ON COLUMN tasks.retry_count IS 'Number of times this task has been retried';
COMMENT ON COLUMN tasks.last_error IS 'Most recent error message';
COMMENT ON COLUMN tasks.last_error_at IS 'Timestamp of most recent error';

-- ============================================================================
-- PART 3: Add state transitions table for tracking
-- ============================================================================

CREATE TABLE IF NOT EXISTS state_transitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,  -- 'plan' or 'task'
  entity_id UUID NOT NULL,
  from_state TEXT,
  to_state TEXT NOT NULL,
  transition_reason TEXT,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_state_transitions_entity ON state_transitions(entity_type, entity_id);
CREATE INDEX idx_state_transitions_created ON state_transitions(created_at DESC);

COMMENT ON TABLE state_transitions IS 'Tracks all state transitions for recovery and debugging';

-- ============================================================================
-- PART 4: Add performance metrics table
-- ============================================================================

CREATE TABLE IF NOT EXISTS performance_metrics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  metric_type TEXT NOT NULL,  -- 'prd_to_plan', 'plan_to_tasks', 'task_execution', etc.
  entity_id UUID,
  duration_seconds FLOAT NOT NULL,
  success BOOLEAN NOT NULL,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_performance_metrics_type ON performance_metrics(metric_type);
CREATE INDEX idx_performance_metrics_created ON performance_metrics(created_at DESC);

COMMENT ON TABLE performance_metrics IS 'Tracks performance metrics for optimization';

-- ============================================================================
-- PART 5: Update create_task_with_packet to validate prompt_packet
-- ============================================================================

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
  -- Validate prompt is not empty
  IF p_prompt IS NULL OR trim(p_prompt) = '' THEN
    RAISE EXCEPTION 'prompt_packet cannot be empty for task %', p_task_number;
  END IF;
  
  -- Validate confidence
  IF p_confidence IS NOT NULL AND p_confidence < 0.95 THEN
    RAISE EXCEPTION 'confidence must be >= 0.95 for task %, got %', p_task_number, p_confidence;
  END IF;
  
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
    result,
    retry_count
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
    jsonb_build_object(
      'prompt_packet', p_prompt,
      'expected_output', p_expected_output,
      'context', p_context
    ),
    0
  )
  RETURNING id INTO v_task_id;
  
  -- Also insert into task_packets for versioning
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
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION create_task_with_packet IS 
'Creates a task with validation: prompt_packet required, confidence >= 0.95, stores in both tasks.result and task_packets';

-- ============================================================================
-- PART 6: Add function to record state transitions
-- ============================================================================

CREATE OR REPLACE FUNCTION record_state_transition(
  p_entity_type TEXT,
  p_entity_id UUID,
  p_from_state TEXT,
  p_to_state TEXT,
  p_reason TEXT DEFAULT NULL,
  p_metadata JSONB DEFAULT '{}'
)
RETURNS UUID AS $$
DECLARE
  v_transition_id UUID;
BEGIN
  INSERT INTO state_transitions (
    entity_type,
    entity_id,
    from_state,
    to_state,
    transition_reason,
    metadata
  ) VALUES (
    p_entity_type,
    p_entity_id,
    p_from_state,
    p_to_state,
    p_reason,
    p_metadata
  )
  RETURNING id INTO v_transition_id;
  
  RETURN v_transition_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION record_state_transition IS 
'Records state transitions for recovery and debugging';

-- ============================================================================
-- PART 7: Add function to record performance metrics
-- ============================================================================

CREATE OR REPLACE FUNCTION record_performance_metric(
  p_metric_type TEXT,
  p_entity_id UUID DEFAULT NULL,
  p_duration_seconds FLOAT,
  p_success BOOLEAN,
  p_metadata JSONB DEFAULT '{}'
)
RETURNS UUID AS $$
DECLARE
  v_metric_id UUID;
BEGIN
  INSERT INTO performance_metrics (
    metric_type,
    entity_id,
    duration_seconds,
    success,
    metadata
  ) VALUES (
    p_metric_type,
    p_entity_id,
    p_duration_seconds,
    p_success,
    p_metadata
  )
  RETURNING id INTO v_metric_id;
  
  RETURN v_metric_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION record_performance_metric IS 
'Records performance metrics for optimization';

-- ============================================================================
-- PART 8: Add function to get latest state for recovery
-- ============================================================================

CREATE OR REPLACE FUNCTION get_latest_state(
  p_entity_type TEXT,
  p_entity_id UUID
)
RETURNS TABLE (
  to_state TEXT,
  transition_reason TEXT,
  created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    st.to_state,
    st.transition_reason,
    st.created_at
  FROM state_transitions st
  WHERE st.entity_type = p_entity_type
    AND st.entity_id = p_entity_id
  ORDER BY st.created_at DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_latest_state IS 
'Gets the latest state for an entity for recovery purposes';

SELECT 'Migration 050 complete - state tracking and revision history added' AS status;
