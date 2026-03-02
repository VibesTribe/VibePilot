-- VibePilot Migration 047: Fix revision_history not being recorded
-- Purpose: record_planner_revision should update plans.revision_history
-- 
-- Problem: Migration 042 removed the revision_history update, only inserting
-- into planner_learned_rules. Planner has no memory of what was tried.
--
-- Solution: Add revision_history update back to record_planner_revision

CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns TEXT[],
  p_tasks_needing_revision TEXT[] DEFAULT '{}'
) RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
  v_concern TEXT;
  v_history_entry JSONB;
BEGIN
  -- Build history entry
  v_history_entry := jsonb_build_object(
    'timestamp', NOW(),
    'concerns', COALESCE(to_jsonb(p_concerns), '[]'::jsonb),
    'tasks_needing_revision', COALESCE(to_jsonb(p_tasks_needing_revision), '[]'::jsonb)
  );
  
  -- Append to plan's revision history
  UPDATE plans 
  SET revision_history = COALESCE(revision_history, '[]'::jsonb) || jsonb_build_array(v_history_entry)
  WHERE id = p_plan_id;
  
  -- Also create learned rules for each concern
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
'Record supervisor revision feedback. Updates plan.revision_history and inserts into planner_learned_rules.';

SELECT 'Migration 047 complete - revision_history now recorded' AS status;
