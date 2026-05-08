-- Migration 114: All remaining RPCs + missing tables
-- Every RPC the Go governor calls, with param names matching the Go code exactly.
-- DROP IF EXISTS first to avoid "cannot change return type" errors.

-- ========================================
-- MISSING TABLES
-- ========================================

CREATE TABLE IF NOT EXISTS checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id),
  step TEXT NOT NULL,
  progress NUMERIC DEFAULT 0,
  output TEXT,
  files JSONB,
  timestamp TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(task_id, step)
);

CREATE TABLE IF NOT EXISTS state_transitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  from_state TEXT,
  to_state TEXT,
  transition_reason TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ========================================
-- FIX: set_processing and clear_processing param names
-- Go code uses p_table/p_id, not p_table_name/p_record_id
-- ========================================

DROP FUNCTION IF EXISTS set_processing(TEXT, UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS set_processing(TEXT, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION set_processing(
  p_table TEXT,
  p_id TEXT,
  p_processing_by TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  EXECUTE format('UPDATE %I SET processing_by = %L, processing_at = NOW() WHERE id = %L AND processing_by IS NULL',
    p_table, p_processing_by, p_id);
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP FUNCTION IF EXISTS clear_processing(TEXT, UUID) CASCADE;
DROP FUNCTION IF EXISTS clear_processing(TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION clear_processing(
  p_table TEXT,
  p_id TEXT
)
RETURNS VOID AS $$
BEGIN
  EXECUTE format('UPDATE %I SET processing_by = NULL, processing_at = NULL WHERE id = %L',
    p_table, p_id);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ========================================
-- 24 REMAINING RPCs (all actively called by Go code)
-- ========================================

-- 1. claim_for_review
DROP FUNCTION IF EXISTS claim_review(UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS claim_for_review(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE tasks
  SET status = 'review', processing_by = p_reviewer_id, processing_at = NOW(), updated_at = NOW()
  WHERE id = p_task_id AND status IN ('approved', 'merge_pending') AND processing_by IS NULL;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. get_planner_rules
DROP FUNCTION IF EXISTS get_planner_rules(TEXT, INT) CASCADE;
CREATE OR REPLACE FUNCTION get_planner_rules(
  p_applies_to TEXT DEFAULT NULL,
  p_limit INT DEFAULT 20
)
RETURNS TABLE(id UUID, rule_name TEXT, rule_text TEXT, applies_to TEXT, priority INT) AS $$
BEGIN
  RETURN QUERY
  SELECT pr.id, pr.rule_name, pr.rule_text, pr.applies_to, pr.priority
  FROM planner_rules pr
  WHERE p_applies_to IS NULL OR pr.applies_to = p_applies_to OR pr.applies_to = 'all'
  ORDER BY pr.priority DESC
  LIMIT p_limit;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. save_checkpoint
DROP FUNCTION IF EXISTS save_checkpoint(UUID, TEXT, NUMERIC, TEXT, JSONB, TIMESTAMPTZ) CASCADE;
CREATE OR REPLACE FUNCTION save_checkpoint(
  p_task_id UUID,
  p_step TEXT,
  p_progress NUMERIC DEFAULT 0,
  p_output TEXT DEFAULT NULL,
  p_files JSONB DEFAULT NULL,
  p_timestamp TIMESTAMPTZ DEFAULT NOW()
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO checkpoints (task_id, step, progress, output, files, timestamp)
  VALUES (p_task_id, p_step, p_progress, p_output, p_files, p_timestamp)
  ON CONFLICT (task_id, step) DO UPDATE SET
    progress = p_progress,
    output = p_output,
    files = p_files,
    timestamp = p_timestamp;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. load_checkpoint
DROP FUNCTION IF EXISTS load_checkpoint(UUID) CASCADE;
CREATE OR REPLACE FUNCTION load_checkpoint(
  p_task_id UUID
)
RETURNS TABLE(step TEXT, progress NUMERIC, output TEXT, files JSONB, checkpoint_ts TIMESTAMPTZ) AS $$
BEGIN
  RETURN QUERY
  SELECT c.step, c.progress, c.output, c.files, c.timestamp AS checkpoint_ts
  FROM checkpoints c
  WHERE c.task_id = p_task_id
  ORDER BY c.timestamp DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. delete_checkpoint
DROP FUNCTION IF EXISTS delete_checkpoint(UUID) CASCADE;
CREATE OR REPLACE FUNCTION delete_checkpoint(
  p_task_id UUID
)
RETURNS VOID AS $$
BEGIN
  DELETE FROM checkpoints WHERE task_id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6. find_stale_processing
DROP FUNCTION IF EXISTS find_stale_processing(TEXT, INT) CASCADE;
CREATE OR REPLACE FUNCTION find_stale_processing(
  p_table TEXT,
  p_timeout_seconds INT DEFAULT 360
)
RETURNS TABLE(id UUID, processing_by TEXT, processing_at TIMESTAMPTZ, seconds_stale NUMERIC) AS $$
BEGIN
  RETURN QUERY EXECUTE format(
    'SELECT id, processing_by, processing_at, EXTRACT(EPOCH FROM (NOW() - processing_at))::NUMERIC
     FROM %I
     WHERE processing_by IS NOT NULL
       AND processing_at < NOW() - INTERVAL ''1 second'' * %s',
    p_table, p_timeout_seconds
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 7. recover_stale_processing
DROP FUNCTION IF EXISTS recover_stale_processing(TEXT, UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION recover_stale_processing(
  p_table TEXT,
  p_id UUID,
  p_reason TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  EXECUTE format(
    'UPDATE %I SET processing_by = NULL, processing_at = NULL, status = CASE
       WHEN status = ''in_progress'' THEN ''available''
       WHEN status = ''processing'' THEN ''pending''
       ELSE status
     END
     WHERE id = %L',
    p_table, p_id
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 8. update_task_branch
DROP FUNCTION IF EXISTS update_task_branch(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION update_task_branch(
  p_task_id UUID,
  p_branch_name TEXT
)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks SET branch_name = p_branch_name, updated_at = NOW() WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 9. unlock_dependent_tasks
DROP FUNCTION IF EXISTS unlock_dependent_tasks(UUID) CASCADE;
CREATE OR REPLACE FUNCTION unlock_dependent_tasks(
  p_completed_task_id UUID
)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET status = 'available', updated_at = NOW()
  WHERE id IN (
    SELECT t.id FROM tasks t
    WHERE t.depends_on @> to_jsonb(p_completed_task_id)
      AND t.status = 'blocked'
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 10. get_next_task_number_for_slice
DROP FUNCTION IF EXISTS get_next_task_number_for_slice(UUID) CASCADE;
CREATE OR REPLACE FUNCTION get_next_task_number_for_slice(
  p_slice_id UUID
)
RETURNS INT AS $$
DECLARE
  v_next INT;
BEGIN
  SELECT COALESCE(MAX(task_number), 0) + 1 INTO v_next
  FROM tasks WHERE slice_id = p_slice_id;
  RETURN v_next;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 11. log_orchestrator_event
DROP FUNCTION IF EXISTS log_orchestrator_event(TEXT, UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION log_orchestrator_event(
  p_event_type TEXT,
  p_task_id UUID DEFAULT NULL,
  p_message TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO orchestrator_events (event_type, task_id, payload)
  VALUES (p_event_type, p_task_id, jsonb_build_object('message', p_message, 'timestamp', NOW()));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 12. store_council_reviews
DROP FUNCTION IF EXISTS store_council_reviews(UUID, JSONB, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION store_council_reviews(
  p_plan_id UUID,
  p_reviews JSONB DEFAULT NULL,
  p_mode TEXT DEFAULT NULL,
  p_models JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO council_reviews (plan_id, reviews, mode, models, created_at)
  VALUES (p_plan_id, p_reviews, p_mode, p_models, NOW())
  ON CONFLICT (plan_id) DO UPDATE SET
    reviews = p_reviews, mode = p_mode, models = p_models, created_at = NOW();
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 13. set_council_consensus
DROP FUNCTION IF EXISTS set_council_consensus(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION set_council_consensus(
  p_plan_id UUID,
  p_consensus TEXT
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO council_reviews (plan_id, consensus, created_at)
  VALUES (p_plan_id, p_consensus, NOW())
  ON CONFLICT (plan_id) DO UPDATE SET consensus = p_consensus, created_at = NOW();
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 14. record_planner_revision
DROP FUNCTION IF EXISTS record_planner_revision(UUID, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns TEXT DEFAULT NULL,
  p_tasks_needing_revision TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO orchestrator_events (event_type, task_id, payload)
  VALUES ('planner_revision', p_plan_id, jsonb_build_object(
    'concerns', p_concerns,
    'tasks_needing_revision', p_tasks_needing_revision,
    'timestamp', NOW()
  ));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 15. record_revision_feedback
DROP FUNCTION IF EXISTS record_revision_feedback(UUID, TEXT, JSONB, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION record_revision_feedback(
  p_plan_id UUID,
  p_source TEXT DEFAULT NULL,
  p_feedback JSONB DEFAULT NULL,
  p_tasks_needing_revision TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO orchestrator_events (event_type, task_id, payload)
  VALUES ('revision_feedback', p_plan_id, jsonb_build_object(
    'source', p_source,
    'feedback', p_feedback,
    'tasks_needing_revision', p_tasks_needing_revision,
    'timestamp', NOW()
  ));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 16. create_maintenance_command
DROP FUNCTION IF EXISTS create_maintenance_command(TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION create_maintenance_command(
  p_command_type TEXT,
  p_payload JSONB DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO maintenance_commands (command_type, payload, status, created_at)
  VALUES (p_command_type, p_payload, 'pending', NOW())
  RETURNING id INTO v_id;
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 17. check_platform_availability
DROP FUNCTION IF EXISTS check_platform_availability(TEXT) CASCADE;
CREATE OR REPLACE FUNCTION check_platform_availability(
  p_platform_id TEXT
)
RETURNS BOOLEAN AS $$
DECLARE
  v_available BOOLEAN;
BEGIN
  SELECT is_available INTO v_available
  FROM platforms
  WHERE id::TEXT = p_platform_id OR name = p_platform_id
  LIMIT 1;

  RETURN COALESCE(v_available, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 18. get_model_score_for_task
DROP FUNCTION IF EXISTS get_model_score_for_task(TEXT, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION get_model_score_for_task(
  p_model_id TEXT,
  p_task_type TEXT DEFAULT NULL,
  p_task_category TEXT DEFAULT NULL
)
RETURNS NUMERIC AS $$
DECLARE
  v_score NUMERIC;
BEGIN
  SELECT CASE
    WHEN total_count = 0 THEN 0.5
    ELSE success_count::NUMERIC / total_count::NUMERIC
  END INTO v_score
  FROM model_scores
  WHERE model_id = p_model_id;

  RETURN COALESCE(v_score, 0.5);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 19. get_problem_solution
DROP FUNCTION IF EXISTS get_problem_solution(TEXT, TEXT, TEXT[]) CASCADE;
CREATE OR REPLACE FUNCTION get_problem_solution(
  p_failure_type TEXT,
  p_task_type TEXT DEFAULT NULL,
  p_keywords TEXT[] DEFAULT NULL
)
RETURNS TABLE(id UUID, problem_type TEXT, solution TEXT, effectiveness NUMERIC) AS $$
BEGIN
  RETURN QUERY
  SELECT ps.id, ps.problem_type, ps.solution, ps.effectiveness
  FROM problem_solutions ps
  WHERE ps.problem_type = p_failure_type
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 20. recover_orphaned_session
DROP FUNCTION IF EXISTS recover_orphaned_session(TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION recover_orphaned_session(
  p_session_id TEXT,
  p_reason TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE task_runs
  SET status = 'failed', result = jsonb_build_object('error', p_reason, 'recovered_at', NOW())
  WHERE session_id = p_session_id AND status = 'in_progress';
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 21. get_latest_state
DROP FUNCTION IF EXISTS get_latest_state(TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION get_latest_state(
  p_entity_type TEXT,
  p_entity_id TEXT
)
RETURNS TABLE(to_state TEXT, transition_reason TEXT, created_at TIMESTAMPTZ) AS $$
BEGIN
  RETURN QUERY
  SELECT st.to_state, st.transition_reason, st.created_at
  FROM state_transitions st
  WHERE st.entity_type = p_entity_type AND st.entity_id = p_entity_id
  ORDER BY st.created_at DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 22. record_state_transition
DROP FUNCTION IF EXISTS record_state_transition(TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION record_state_transition(
  p_entity_type TEXT,
  p_entity_id TEXT,
  p_from_state TEXT DEFAULT NULL,
  p_to_state TEXT DEFAULT NULL,
  p_reason TEXT DEFAULT NULL,
  p_metadata JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO state_transitions (entity_type, entity_id, from_state, to_state, transition_reason, metadata)
  VALUES (p_entity_type, p_entity_id, p_from_state, p_to_state, p_reason, p_metadata);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 23. log_security_audit
DROP FUNCTION IF EXISTS log_security_audit(TEXT, TEXT, BOOLEAN, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION log_security_audit(
  p_operation TEXT,
  p_key_name TEXT DEFAULT NULL,
  p_allowed BOOLEAN DEFAULT NULL,
  p_reason TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO orchestrator_events (event_type, payload)
  VALUES ('security_audit', jsonb_build_object(
    'operation', p_operation,
    'key_name', p_key_name,
    'allowed', p_allowed,
    'reason', p_reason,
    'timestamp', NOW()
  ));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 24. update_research_suggestion_status
DROP FUNCTION IF EXISTS update_research_suggestion_status(UUID, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION update_research_suggestion_status(
  p_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE research_suggestions
  SET status = p_status,
      review_notes = COALESCE(p_review_notes, review_notes),
      updated_at = NOW()
  WHERE id = p_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ========================================
-- GRANT permissions
-- ========================================
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;

SELECT 'Migration 114 complete - all 24 RPCs + 2 tables created' AS status;
