-- VibePilot Migration 055: Add State Tracking (Safe Version)
-- Purpose: Add new columns and tables WITHOUT modifying existing functions
-- Run this after: 054_simple_reset.sql

-- ============================================================================
-- PART 1: Add revision tracking to plans
-- ============================================================================

ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_round INT DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_history JSONB DEFAULT '[]'::jsonb;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS latest_feedback JSONB;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS tasks_needing_revision TEXT[] DEFAULT '{}';

-- ============================================================================
-- PART 2: Add retry tracking to tasks
-- ============================================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

-- ============================================================================
-- PART 3: Add state transitions table
-- ============================================================================

CREATE TABLE IF NOT EXISTS state_transitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  from_state TEXT,
  to_state TEXT NOT NULL,
  transition_reason TEXT,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_state_transitions_entity ON state_transitions(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_state_transitions_created ON state_transitions(created_at DESC);

-- ============================================================================
-- PART 4: Add performance metrics table
-- ============================================================================

CREATE TABLE IF NOT EXISTS performance_metrics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  metric_type TEXT NOT NULL,
  entity_id UUID,
  duration_seconds FLOAT NOT NULL,
  success BOOLEAN NOT NULL,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_performance_metrics_type ON performance_metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_performance_metrics_created ON performance_metrics(created_at DESC);

-- ============================================================================
-- PART 5: Add helper functions
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
    entity_type, entity_id, from_state, to_state, transition_reason, metadata
  ) VALUES (
    p_entity_type, p_entity_id, p_from_state, p_to_state, p_reason, p_metadata
  )
  RETURNING id INTO v_transition_id;
  
  RETURN v_transition_id;
END;
$$ LANGUAGE plpgsql;

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
    metric_type, entity_id, duration_seconds, success, metadata
  ) VALUES (
    p_metric_type, p_entity_id, p_duration_seconds, p_success, p_metadata
  )
  RETURNING id INTO v_metric_id;
  
  RETURN v_metric_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 055 complete - state tracking added (no function changes)' AS status;
