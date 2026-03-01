-- VibePilot Migration 042: Processing State for Plans & Tasks
-- Purpose: Prevent duplicate event firing by tracking active processing
-- 
-- Problem: Events fire every poll cycle (1s) while agent works because status
-- doesn't change until work completes. Plans take minutes, causing infinite
-- duplicate event firing and capacity exhaustion.
--
-- Solution: Add processing_by column to track active processing. Event detection
-- checks processing_by IS NULL. Set atomically before spawning agent, clear when done.
--
-- Recovery: Periodic check for stale processing (timeout configurable) clears
-- processing_by so items become pickable again.

-- ============================================================================
-- PLANS TABLE
-- ============================================================================

ALTER TABLE plans ADD COLUMN IF NOT EXISTS processing_by TEXT;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_plans_processing ON plans(processing_by) 
  WHERE processing_by IS NOT NULL;

COMMENT ON COLUMN plans.processing_by IS 'Agent working on this plan: "agent_type:session_id" format. NULL when idle.';
COMMENT ON COLUMN plans.processing_at IS 'Timestamp when processing started. Used for timeout recovery.';

-- ============================================================================
-- TASKS TABLE
-- ============================================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS processing_by TEXT;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tasks_processing ON tasks(processing_by) 
  WHERE processing_by IS NOT NULL;

COMMENT ON COLUMN tasks.processing_by IS 'Agent working on this task: "agent_type:session_id" format. NULL when idle.';
COMMENT ON COLUMN tasks.processing_at IS 'Timestamp when processing started. Used for timeout recovery.';

-- ============================================================================
-- RPC: SET PROCESSING (Atomic Claim)
-- ============================================================================

CREATE OR REPLACE FUNCTION set_processing(
  p_table TEXT,
  p_id UUID,
  p_processing_by TEXT
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans 
    SET processing_by = p_processing_by, 
        processing_at = NOW(),
        updated_at = NOW()
    WHERE id = p_id AND processing_by IS NULL;
    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RETURN v_updated > 0;
  ELSIF p_table = 'tasks' THEN
    UPDATE tasks 
    SET processing_by = p_processing_by, 
        processing_at = NOW(),
        updated_at = NOW()
    WHERE id = p_id AND processing_by IS NULL;
    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RETURN v_updated > 0;
  END IF;
  RETURN FALSE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION set_processing(TEXT, UUID, TEXT) IS 
'Atomically claim processing of a plan or task. Returns TRUE if claim succeeded (was NULL), FALSE if already claimed.';

-- ============================================================================
-- RPC: CLEAR PROCESSING
-- ============================================================================

CREATE OR REPLACE FUNCTION clear_processing(
  p_table TEXT,
  p_id UUID
) RETURNS VOID AS $$
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans 
    SET processing_by = NULL, 
        processing_at = NULL,
        updated_at = NOW()
    WHERE id = p_id;
  ELSIF p_table = 'tasks' THEN
    UPDATE tasks 
    SET processing_by = NULL, 
        processing_at = NULL,
        updated_at = NOW()
    WHERE id = p_id;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION clear_processing(TEXT, UUID) IS 
'Release processing claim on a plan or task. Always succeeds even if not claimed.';

-- ============================================================================
-- RPC: FIND STALE PROCESSING (Recovery)
-- ============================================================================

CREATE OR REPLACE FUNCTION find_stale_processing(
  p_table TEXT,
  p_timeout_seconds INT DEFAULT 300
) RETURNS TABLE(
  id UUID,
  processing_by TEXT,
  processing_at TIMESTAMPTZ,
  seconds_stale INT
) AS $$
BEGIN
  IF p_table = 'plans' THEN
    RETURN QUERY 
    SELECT 
      plans.id,
      plans.processing_by,
      plans.processing_at,
      EXTRACT(EPOCH FROM (NOW() - plans.processing_at))::INT AS seconds_stale
    FROM plans 
    WHERE plans.processing_by IS NOT NULL 
      AND plans.processing_at < NOW() - (p_timeout_seconds || ' seconds')::interval
    ORDER BY plans.processing_at;
  ELSIF p_table = 'tasks' THEN
    RETURN QUERY 
    SELECT 
      tasks.id,
      tasks.processing_by,
      tasks.processing_at,
      EXTRACT(EPOCH FROM (NOW() - tasks.processing_at))::INT AS seconds_stale
    FROM tasks 
    WHERE tasks.processing_by IS NOT NULL
      AND tasks.processing_at < NOW() - (p_timeout_seconds || ' seconds')::interval
    ORDER BY tasks.processing_at;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION find_stale_processing(TEXT, INT) IS 
'Find plans or tasks that have been processing longer than timeout. Used for recovery.';

-- ============================================================================
-- RPC: RECOVER STALE PROCESSING
-- ============================================================================

CREATE OR REPLACE FUNCTION recover_stale_processing(
  p_table TEXT,
  p_id UUID,
  p_reason TEXT DEFAULT 'timeout_recovery'
) RETURNS VOID AS $$
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans 
    SET processing_by = NULL, 
        processing_at = NULL,
        updated_at = NOW(),
        review_notes = COALESCE(review_notes, '{}'::jsonb) || 
          jsonb_build_object('recovery_reason', p_reason, 'recovered_at', NOW())
    WHERE id = p_id;
  ELSIF p_table = 'tasks' THEN
    UPDATE tasks 
    SET processing_by = NULL, 
        processing_at = NULL,
        updated_at = NOW(),
        failure_notes = COALESCE(failure_notes || E'\n', '') || 
          p_reason || ' at ' || NOW()::text
    WHERE id = p_id;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION recover_stale_processing(TEXT, UUID, TEXT) IS 
'Recover a stale plan or task by clearing processing state. Preserves status for re-pickup.';

-- ============================================================================
-- RPC: RECORD PLANNER REVISION (Fixed Parameter Handling)
-- ============================================================================

-- Drop and recreate with proper parameter types
DROP FUNCTION IF EXISTS record_planner_revision(UUID, JSONB, TEXT[]);

CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns TEXT[],
  p_tasks_needing_revision TEXT[] DEFAULT '{}'
) RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
  v_concern TEXT;
BEGIN
  -- Create a learned rule for each concern
  IF p_concerns IS NOT NULL THEN
    FOREACH v_concern IN ARRAY p_concerns
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
  END IF;
  
  RETURN v_rule_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_planner_revision(UUID, TEXT[], TEXT[]) IS 
'Record supervisor revision feedback for planner learning. Accepts TEXT[] not JSONB.';

-- ============================================================================
-- RPC: RECORD SUPERVISOR RULE (Simplified for validation feedback)
-- ============================================================================

CREATE OR REPLACE FUNCTION record_supervisor_rule(
  p_rule_text TEXT,
  p_applies_to TEXT DEFAULT 'plan_review',
  p_source TEXT DEFAULT 'supervisor',
  p_details JSONB DEFAULT '{}'::jsonb
) RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
BEGIN
  INSERT INTO supervisor_rules (
    rule_text,
    applies_to,
    source,
    details,
    active
  ) VALUES (
    p_rule_text,
    p_applies_to,
    p_source,
    p_details,
    true
  )
  RETURNING id INTO v_rule_id;
  
  RETURN v_rule_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_supervisor_rule(TEXT, TEXT, TEXT, JSONB) IS 
'Record a learned supervisor rule for future reviews.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 042 complete - processing state enabled' AS status;

-- Verify columns exist
SELECT 'plans.processing_by' AS column_check FROM information_schema.columns 
  WHERE table_name = 'plans' AND column_name = 'processing_by';
SELECT 'tasks.processing_by' AS column_check FROM information_schema.columns 
  WHERE table_name = 'tasks' AND column_name = 'processing_by';
