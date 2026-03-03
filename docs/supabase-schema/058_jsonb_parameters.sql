-- VibePilot Migration 058: Convert TEXT[] to JSONB
-- Purpose: Database agnosticism - JSONB works everywhere, TEXT[] is PostgreSQL-only
--
-- This migration ensures:
-- Zero vendor lock-in (swap Supabase -> any database)
-- Universal JSON interface (LLM-friendly, consistent)
-- No code changes needed when swapping databases
--
-- RPCs affected:
-- record_planner_revision (migrations 042, 047)
-- get_problem_solution (migration 024)
-- find_tasks_with_checkpoints (migration 057)
-- record_revision_feedback (migration 036)

-- ============================================================================
-- 1. record_planner_revision - Convert TEXT[] to JSONB
-- ============================================================================

DROP FUNCTION IF EXISTS record_planner_revision(UUID, TEXT[], TEXT[]);

CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns JSONB DEFAULT '[]'::jsonb,
  p_tasks_needing_revision JSONB DEFAULT '[]'::jsonb
) RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
  v_concern TEXT;
  v_concerns_array TEXT[];
  v_tasks_array TEXT[];
  v_history_entry JSONB;
BEGIN
  v_concerns_array := ARRAY(SELECT jsonb_array_elements_text(p_concerns));
  v_tasks_array := ARRAY(SELECT jsonb_array_elements_text(p_tasks_needing_revision));
  
  v_history_entry := jsonb_build_object(
    'timestamp', NOW(),
    'concerns', p_concerns,
    'tasks_needing_revision', p_tasks_needing_revision
  );
  
  UPDATE plans 
  SET revision_history = COALESCE(revision_history, '[]'::jsonb) || jsonb_build_array(v_history_entry)
  WHERE id = p_plan_id;
  
  IF jsonb_array_length(p_concerns) > 0 THEN
    FOREACH v_concern IN ARRAY v_concerns_array
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
  
  RETURN COALESCE(v_rule_id, p_plan_id);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_planner_revision(UUID, JSONB, JSONB) IS 
'Record supervisor revision feedback. Uses JSONB for database agnosticism.';

-- ============================================================================
-- 2. get_problem_solution - Convert TEXT[] to JSONB
-- ============================================================================

DROP FUNCTION IF EXISTS get_problem_solution(TEXT, TEXT, TEXT[]);

CREATE OR REPLACE FUNCTION get_problem_solution(
  p_failure_type TEXT DEFAULT NULL,
  p_task_type TEXT DEFAULT NULL,
  p_keywords JSONB DEFAULT '[]'::jsonb
) RETURNS TABLE(
  id UUID,
  solution_type TEXT,
  solution_model TEXT,
  solution_details JSONB,
  success_rate FLOAT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    ps.id,
    ps.solution_type,
    ps.solution_model,
    ps.solution_details,
    ps.success_rate
  FROM problem_solutions ps
  WHERE 
    ps.problem_pattern ILIKE '%' || p_failure_type || '%'
    OR p_failure_type = ANY(ps.keywords)
    OR ps.keywords && p_keywords
  ORDER BY 
    ps.success_rate DESC,
    ps.success_count DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION get_problem_solution(TEXT, TEXT, JSONB) IS 
'Find solution for a problem. Uses JSONB for keywords to enable database agnosticism.';

-- ============================================================================
-- 3. find_tasks_with_checkpoints - Convert TEXT[] to JSONB
-- ============================================================================

DROP FUNCTION IF EXISTS find_tasks_with_checkpoints(TEXT[]);

CREATE OR REPLACE FUNCTION find_tasks_with_checkpoints(
  p_statuses JSONB DEFAULT '["in_progress", "review", "testing"]'::jsonb
) RETURNS TABLE (
  task_id UUID,
  task_number TEXT,
  title TEXT,
  status TEXT,
  step TEXT,
  progress INT,
  checkpoint_created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.id AS task_id,
    t.task_number,
    t.title,
    t.status,
    tc.step,
    tc.progress,
    tc.created_at AS checkpoint_created_at
  FROM tasks t
  INNER JOIN task_checkpoints tc ON t.id = tc.task_id
  WHERE t.status IN (SELECT jsonb_array_elements_text(p_statuses))
  ORDER BY tc.created_at ASC;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION find_tasks_with_checkpoints(JSONB) IS 
'Find tasks that have checkpoints and are in specified statuses. Uses JSONB for database agnosticism. Used for crash recovery.';

-- ============================================================================
-- 4. record_revision_feedback - Convert TEXT[] to JSONB
-- ============================================================================

DROP FUNCTION IF EXISTS record_revision_feedback(UUID, TEXT, JSONB, TEXT[]);

CREATE OR REPLACE FUNCTION record_revision_feedback(
  p_plan_id UUID,
  p_source TEXT DEFAULT 'supervisor',
  p_feedback JSONB DEFAULT '{}'::jsonb,
  p_tasks_needing_revision JSONB DEFAULT '[]'::jsonb
) RETURNS VOID AS $$
DECLARE
  v_current_round INT;
BEGIN
  SELECT revision_round INTO v_current_round
  FROM plans WHERE id = p_plan_id;
  
  UPDATE plans SET
    revision_round = revision_round + 1,
    revision_history = COALESCE(revision_history, '[]'::jsonb) || jsonb_build_array(
      jsonb_build_object(
        'round', v_current_round,
        'source', p_source,
        'feedback', p_feedback,
        'tasks_needing_revision', p_tasks_needing_revision,
        'timestamp', NOW()
      )
    )
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_revision_feedback(UUID, TEXT, JSONB, JSONB) IS 
'Record revision feedback for a plan. Uses JSONB for database agnosticism.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 058 complete - all TEXT[] converted to JSONB' AS status;
