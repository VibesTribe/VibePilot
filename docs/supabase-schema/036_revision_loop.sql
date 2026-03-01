-- VibePilot Migration 036: Revision Loop Support
-- Purpose: Enable multi-round revision loop with tracking
-- 
-- Config: Max rounds controlled by config/plan_lifecycle.json revision_rules.max_rounds
-- Default: 6 rounds (configurable)

-- Add revision tracking
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_round INT DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_history JSONB DEFAULT '[]';

-- Add council execution tracking (HOW it ran, not how many times)
ALTER TABLE plans ADD COLUMN IF NOT EXISTS council_mode TEXT;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS council_models JSONB DEFAULT '[]';

-- Index for finding plans needing attention
CREATE INDEX IF NOT EXISTS idx_plans_revision_round ON plans(revision_round) 
  WHERE revision_round > 0;

-- RPC: Record revision feedback
-- Called by supervisor OR council when revision needed
-- Stores feedback for planner learning and audit trail
CREATE OR REPLACE FUNCTION record_revision_feedback(
  p_plan_id UUID,
  p_source TEXT,
  p_feedback JSONB,
  p_tasks_needing_revision TEXT[] DEFAULT '{}'
)
RETURNS VOID AS $$
DECLARE
  v_current_round INT;
  v_history_entry JSONB;
BEGIN
  SELECT revision_round INTO v_current_round FROM plans WHERE id = p_plan_id;
  
  v_history_entry := jsonb_build_object(
    'round', v_current_round,
    'source', p_source,
    'feedback', p_feedback,
    'tasks_affected', p_tasks_needing_revision,
    'timestamp', NOW()
  );
  
  UPDATE plans SET
    revision_history = revision_history || jsonb_build_array(v_history_entry),
    updated_at = NOW()
  WHERE id = p_plan_id;
  
  -- If source is council, also create learned rules for planner
  IF p_source = 'council' THEN
    INSERT INTO planner_learned_rules (
      applies_to, rule_type, rule_text, source, details
    )
    SELECT 
      'task_creation',
      'council_feedback',
      value::text,
      'council',
      jsonb_build_object('plan_id', p_plan_id, 'round', v_current_round)
    FROM jsonb_array_elements_text(p_feedback->'concerns')
    ON CONFLICT DO NOTHING;
  END IF;
END;
$$ LANGUAGE plpgsql;

-- RPC: Increment revision round
-- Called when plan enters revision_needed state
CREATE OR REPLACE FUNCTION increment_revision_round(p_plan_id UUID)
RETURNS INT AS $$
DECLARE v_new_round INT;
BEGIN
  UPDATE plans SET
    revision_round = COALESCE(revision_round, 0) + 1,
    updated_at = NOW()
  WHERE id = p_plan_id
  RETURNING revision_round INTO v_new_round;
  RETURN v_new_round;
END;
$$ LANGUAGE plpgsql;

-- RPC: Check revision limit
-- p_max_rounds comes from config/plan_lifecycle.json
-- This RPC just checks, doesn't enforce (governor enforces)
CREATE OR REPLACE FUNCTION check_revision_limit(
  p_plan_id UUID,
  p_max_rounds INT DEFAULT 6
)
RETURNS BOOLEAN AS $$
DECLARE v_round INT;
BEGIN
  SELECT revision_round INTO v_round FROM plans WHERE id = p_plan_id;
  RETURN COALESCE(v_round, 0) >= p_max_rounds;
END;
$$ LANGUAGE plpgsql;

-- RPC: Store council reviews
CREATE OR REPLACE FUNCTION store_council_reviews(
  p_plan_id UUID,
  p_reviews JSONB,
  p_mode TEXT,
  p_models JSONB
)
RETURNS VOID AS $$
BEGIN
  UPDATE plans SET
    council_reviews = p_reviews,
    council_mode = p_mode,
    council_models = p_models,
    updated_at = NOW()
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 036 complete - revision loop enabled (max_rounds configurable in plan_lifecycle.json)' AS status;
