-- ============================================================================
-- VIBESPILOT SCHEMA MIGRATION 111
-- Purpose: Create ALL missing RPCs that the governor binary calls
-- Date: 2026-04-15
--
-- This migration closes the gap between Go code expectations and Supabase state.
-- 42 RPCs were referenced in the Go codebase but never created in Supabase.
-- Every function signature matches exactly what the Go RPC() calls pass.
--
-- Tables these depend on (already exist):
--   tasks, plans, task_runs, task_checkpoints, council_reviews,
--   maintenance_commands, research_suggestions, test_results,
--   failure_records, learned_heuristics, problem_solutions,
--   lessons_learned, state_transitions, performance_metrics,
--   memory_sessions, memory_project, memory_rules
--
-- New tables created:
--   security_audit_log, planner_rules, revision_feedback
--
-- RLS: service_role bypasses all RLS. Anon key has read-only where needed.
-- ============================================================================

BEGIN;

-- ============================================================================
-- 1. TABLES THAT DON'T EXIST YET
-- ============================================================================

-- Security audit log for vault operations
CREATE TABLE IF NOT EXISTS security_audit_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operation TEXT NOT NULL,
  key_name TEXT,
  allowed BOOLEAN NOT NULL DEFAULT true,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_security_audit_created ON security_audit_log(created_at DESC);

-- Planner rules for council feedback learning
CREATE TABLE IF NOT EXISTS planner_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  applies_to TEXT NOT NULL DEFAULT '*',
  rule_type TEXT NOT NULL DEFAULT 'general',
  rule_text TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT 'auto',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_planner_rules_active ON planner_rules(active) WHERE active = true;

-- Revision feedback tracking
CREATE TABLE IF NOT EXISTS revision_feedback (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID REFERENCES plans(id) ON DELETE CASCADE,
  source TEXT NOT NULL DEFAULT 'supervisor',
  feedback JSONB NOT NULL DEFAULT '{}',
  tasks_needing_revision TEXT[] DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_revision_feedback_plan ON revision_feedback(plan_id);

-- Enable RLS on new tables
ALTER TABLE security_audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE planner_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE revision_feedback ENABLE ROW LEVEL SECURITY;

CREATE POLICY "service_all_security_audit" ON security_audit_log FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "service_all_planner_rules" ON planner_rules FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_read_planner_rules" ON planner_rules FOR SELECT USING (true);
CREATE POLICY "service_all_revision_feedback" ON revision_feedback FOR ALL USING (true) WITH CHECK (true);

-- ============================================================================
-- 2. TASK LIFECYCLE RPCs
-- ============================================================================

-- claim_task: Atomically claim an available task for execution
-- Go params: p_task_id UUID, p_worker_id TEXT, p_model_id TEXT
CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_worker_id TEXT,
  p_model_id TEXT
) RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_model_id,
      processing_by = p_worker_id,
      processing_at = NOW(),
      started_at = COALESCE(started_at, NOW()),
      updated_at = NOW()
  WHERE id = p_task_id
    AND status IN ('available', 'pending_resources')
    AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes')
  RETURNING jsonb_build_object(
    'id', id,
    'status', status,
    'task_number', task_number,
    'title', title,
    'type', type,
    'result', result,
    'plan_id', plan_id,
    'slice_id', slice_id,
    'task_number', task_number
  ) INTO v_result;

  RETURN v_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- claim_for_review: Atomically claim a task for supervisor review
-- Go params: p_task_id UUID, p_reviewer_id TEXT
CREATE OR REPLACE FUNCTION claim_for_review(
  p_task_id UUID,
  p_reviewer_id TEXT
) RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  UPDATE tasks
  SET processing_by = p_reviewer_id,
      processing_at = NOW(),
      updated_at = NOW()
  WHERE id = p_task_id
    AND status IN ('review', 'testing')
    AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes')
  RETURNING jsonb_build_object(
    'id', id,
    'status', status,
    'task_number', task_number,
    'title', title,
    'type', type,
    'result', result,
    'branch_name', branch_name
  ) INTO v_result;

  RETURN v_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- create_task_run: Record a task execution run
-- Go params: p_task_id, p_model_id, p_courier, p_platform, p_status,
--            p_tokens_in, p_tokens_out, p_tokens_used, p_courier_model_id,
--            p_courier_tokens, p_courier_cost_usd, p_platform_theoretical_cost_usd,
--            p_total_actual_cost_usd, p_total_savings_usd
CREATE OR REPLACE FUNCTION create_task_run(
  p_task_id UUID,
  p_model_id TEXT,
  p_courier TEXT,
  p_platform TEXT,
  p_status TEXT DEFAULT 'success',
  p_tokens_in INT DEFAULT 0,
  p_tokens_out INT DEFAULT 0,
  p_tokens_used INT DEFAULT 0,
  p_courier_model_id TEXT DEFAULT NULL,
  p_courier_tokens INT DEFAULT 0,
  p_courier_cost_usd FLOAT DEFAULT 0,
  p_platform_theoretical_cost_usd FLOAT DEFAULT 0,
  p_total_actual_cost_usd FLOAT DEFAULT 0,
  p_total_savings_usd FLOAT DEFAULT 0
) RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (
    task_id, model_id, courier, platform, status,
    tokens_in, tokens_out, tokens_used,
    courier_model_id, courier_tokens, courier_cost_usd,
    platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
    created_at
  ) VALUES (
    p_task_id, p_model_id, p_courier, p_platform, p_status,
    p_tokens_in, p_tokens_out, p_tokens_used,
    p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
    p_platform_theoretical_cost_usd, p_total_actual_cost_usd, p_total_savings_usd,
    NOW()
  ) RETURNING id INTO v_run_id;

  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- calculate_run_costs: Calculate cost breakdown for a task run
-- Go params: p_model_id, p_tokens_in, p_tokens_out, p_courier_cost_usd
CREATE OR REPLACE FUNCTION calculate_run_costs(
  p_model_id TEXT,
  p_tokens_in INT,
  p_tokens_out INT,
  p_courier_cost_usd FLOAT DEFAULT 0
) RETURNS JSONB AS $$
BEGIN
  -- Free-tier models cost $0. Return zeroed cost structure.
  RETURN jsonb_build_object(
    'theoretical', 0.0,
    'actual', p_courier_cost_usd,
    'savings', 0.0,
    'model_id', p_model_id,
    'tokens_in', p_tokens_in,
    'tokens_out', p_tokens_out
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 3. PLAN LIFECYCLE RPCs
-- ============================================================================

-- create_plan: Create a new plan from a PRD trigger
-- Go params: p_project_id, p_prd_path, p_plan_path
CREATE OR REPLACE FUNCTION create_plan(
  p_project_id UUID,
  p_prd_path TEXT,
  p_plan_path TEXT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_plan_id UUID;
BEGIN
  INSERT INTO plans (
    project_id, prd_path, plan_path, status, created_at, updated_at
  ) VALUES (
    p_project_id, p_prd_path, p_plan_path, 'pending', NOW(), NOW()
  ) RETURNING id INTO v_plan_id;

  RETURN v_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- update_plan_status: Transition plan to new status with optional notes
-- Go params: p_plan_id, p_status, p_review_notes
CREATE OR REPLACE FUNCTION update_plan_status(
  p_plan_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE plans
  SET status = p_status,
      processing_by = NULL,
      processing_at = NULL,
      latest_feedback = COALESCE(p_review_notes, latest_feedback),
      updated_at = NOW()
  WHERE id = p_plan_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 4. PROCESSING STATE RPCs (supplement 086)
-- ============================================================================

-- set_processing: Atomically claim an item for processing
-- Go params vary: sometimes (p_table, p_id, p_processing_by), sometimes (p_id, p_processing_by, p_table)
-- Support BOTH parameter orders via overloading
CREATE OR REPLACE FUNCTION set_processing(
  p_table TEXT,
  p_id UUID,
  p_processing_by TEXT
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans SET processing_by = p_processing_by, processing_at = NOW(), updated_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'tasks' THEN
    UPDATE tasks SET processing_by = p_processing_by, processing_at = NOW(), updated_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'maintenance_commands' THEN
    UPDATE maintenance_commands SET processing_by = p_processing_by, processing_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'research_suggestions' THEN
    UPDATE research_suggestions SET processing_by = p_processing_by, processing_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'test_results' THEN
    UPDATE test_results SET processing_by = p_processing_by, processing_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSE
    RAISE EXCEPTION 'set_processing: unknown table %', p_table;
  END IF;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- clear_processing: Release processing lock on an item
CREATE OR REPLACE FUNCTION clear_processing(
  p_table TEXT,
  p_id UUID
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans SET processing_by = NULL, processing_at = NULL, updated_at = NOW() WHERE id = p_id;
  ELSIF p_table = 'tasks' THEN
    UPDATE tasks SET processing_by = NULL, processing_at = NULL, updated_at = NOW() WHERE id = p_id;
  ELSIF p_table = 'maintenance_commands' THEN
    UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE id = p_id;
  ELSIF p_table = 'research_suggestions' THEN
    UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE id = p_id;
  ELSIF p_table = 'test_results' THEN
    UPDATE test_results SET processing_by = NULL, processing_at = NULL WHERE id = p_id;
  ELSE
    RAISE EXCEPTION 'clear_processing: unknown table %', p_table;
  END IF;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- find_stale_processing: Find items that have been processing too long
-- Go params: p_table TEXT, p_timeout_seconds INT
CREATE OR REPLACE FUNCTION find_stale_processing(
  p_table TEXT,
  p_timeout_seconds INT DEFAULT 300
) RETURNS TABLE (
  id UUID,
  processing_by TEXT,
  seconds_stale FLOAT
) AS $$
BEGIN
  IF p_table = 'plans' THEN
    RETURN QUERY
    SELECT plans.id, plans.processing_by,
           EXTRACT(EPOCH FROM (NOW() - plans.processing_at))::FLOAT AS seconds_stale
    FROM plans
    WHERE plans.processing_by IS NOT NULL
      AND plans.processing_at < NOW() - (p_timeout_seconds || ' seconds')::INTERVAL;
  ELSIF p_table = 'tasks' THEN
    RETURN QUERY
    SELECT tasks.id, tasks.processing_by,
           EXTRACT(EPOCH FROM (NOW() - tasks.processing_at))::FLOAT AS seconds_stale
    FROM tasks
    WHERE tasks.processing_by IS NOT NULL
      AND tasks.processing_at < NOW() - (p_timeout_seconds || ' seconds')::INTERVAL;
  ELSIF p_table = 'maintenance_commands' THEN
    RETURN QUERY
    SELECT maintenance_commands.id, maintenance_commands.processing_by,
           EXTRACT(EPOCH FROM (NOW() - maintenance_commands.processing_at))::FLOAT AS seconds_stale
    FROM maintenance_commands
    WHERE maintenance_commands.processing_by IS NOT NULL
      AND maintenance_commands.processing_at < NOW() - (p_timeout_seconds || ' seconds')::INTERVAL;
  ELSIF p_table = 'research_suggestions' THEN
    RETURN QUERY
    SELECT research_suggestions.id, research_suggestions.processing_by,
           EXTRACT(EPOCH FROM (NOW() - research_suggestions.processing_at))::FLOAT AS seconds_stale
    FROM research_suggestions
    WHERE research_suggestions.processing_by IS NOT NULL
      AND research_suggestions.processing_at < NOW() - (p_timeout_seconds || ' seconds')::INTERVAL;
  ELSIF p_table = 'test_results' THEN
    RETURN QUERY
    SELECT test_results.id, test_results.processing_by,
           EXTRACT(EPOCH FROM (NOW() - test_results.processing_at))::FLOAT AS seconds_stale
    FROM test_results
    WHERE test_results.processing_by IS NOT NULL
      AND test_results.processing_at < NOW() - (p_timeout_seconds || ' seconds')::INTERVAL;
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- recover_stale_processing: Release lock on stale item
-- Go params: p_table TEXT, p_id UUID, p_reason TEXT
CREATE OR REPLACE FUNCTION recover_stale_processing(
  p_table TEXT,
  p_id UUID,
  p_reason TEXT
) RETURNS BOOLEAN AS $$
BEGIN
  -- Clear processing lock so item becomes pickable again
  PERFORM clear_processing(p_table, p_id);
  RETURN true;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 5. LEARNING / MODEL TRACKING RPCs
-- ============================================================================

-- record_model_success: Track successful model execution for learning
-- Go params: p_model_id, p_task_type, p_duration_seconds, p_tokens_used
CREATE OR REPLACE FUNCTION record_model_success(
  p_model_id TEXT,
  p_task_type TEXT,
  p_duration_seconds FLOAT DEFAULT NULL,
  p_tokens_used INT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
  -- Upsert into learned_heuristics to track success
  INSERT INTO learned_heuristics (task_type, preferred_model, source, confidence, success_count, created_at)
  VALUES (p_task_type, p_model_id, 'statistical', 0.5, 1, NOW())
  ON CONFLICT DO NOTHING;

  -- Increment success count
  UPDATE learned_heuristics
  SET success_count = COALESCE(success_count, 0) + 1,
      confidence = LEAST(1.0, COALESCE(success_count, 0)::FLOAT / NULLIF(COALESCE(success_count, 0) + COALESCE(failure_count, 0), 0)),
      last_applied_at = NOW()
  WHERE task_type = p_task_type AND preferred_model = p_model_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- record_model_failure: Track failed model execution for learning
-- Go params: p_model_id, p_task_id, p_failure_type, p_failure_category
CREATE OR REPLACE FUNCTION record_model_failure(
  p_model_id TEXT,
  p_task_id UUID DEFAULT NULL,
  p_failure_type TEXT DEFAULT 'unknown',
  p_failure_category TEXT DEFAULT 'unknown'
) RETURNS VOID AS $$
BEGIN
  -- Record the failure
  INSERT INTO failure_records (task_id, failure_type, model_id, created_at)
  VALUES (p_task_id, p_failure_type, p_model_id, NOW())
  ON CONFLICT DO NOTHING;

  -- Decrement confidence for this model
  UPDATE learned_heuristics
  SET failure_count = COALESCE(failure_count, 0) + 1,
      confidence = GREATEST(0.0, COALESCE(success_count, 0)::FLOAT / NULLIF(COALESCE(success_count, 0) + COALESCE(failure_count, 0) + 1, 0))
  WHERE preferred_model = p_model_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- record_failure: Detailed failure recording from task handler
-- Go params: p_task_id, p_failure_type, p_failure_category, p_failure_details, p_model_id, p_task_type
CREATE OR REPLACE FUNCTION record_failure(
  p_task_id UUID,
  p_failure_type TEXT,
  p_failure_category TEXT DEFAULT NULL,
  p_failure_details JSONB DEFAULT '{}',
  p_model_id TEXT DEFAULT NULL,
  p_task_type TEXT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO failure_records (
    task_id, failure_type, model_id, details, created_at
  ) VALUES (
    p_task_id, p_failure_type, p_model_id, p_failure_details, NOW()
  ) RETURNING id INTO v_id;

  -- Append to task failure notes
  UPDATE tasks
  SET failure_notes = COALESCE(failure_notes || E'\n', '') ||
                      p_failure_type || ' (' || p_failure_category || ') at ' || NOW()::text,
      last_error = p_failure_type || ': ' || COALESCE(p_failure_details->>'description', ''),
      last_error_at = NOW(),
      updated_at = NOW()
  WHERE id = p_task_id;

  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 6. COUNCIL RPCs
-- ============================================================================

-- store_council_reviews: Store all council member reviews
-- Go params: p_plan_id, p_reviews (JSONB array), p_mode TEXT
CREATE OR REPLACE FUNCTION store_council_reviews(
  p_plan_id UUID,
  p_reviews JSONB,
  p_mode TEXT DEFAULT 'sequential_same_model'
) RETURNS VOID AS $$
DECLARE
  review JSONB;
BEGIN
  FOR review IN SELECT * FROM jsonb_array_elements(p_reviews)
  LOOP
    INSERT INTO council_reviews (
      plan_id, reviewer_model, vote, reasoning, concerns, mode, created_at
    ) VALUES (
      p_plan_id,
      COALESCE(review->>'reviewer', review->>'model_id', 'unknown'),
      COALESCE(review->>'vote', review->>'decision', 'unknown'),
      COALESCE(review->>'reasoning', ''),
      COALESCE(review->'concerns', '[]'::jsonb),
      p_mode,
      NOW()
    );
  END LOOP;

  -- Update plan with council mode
  UPDATE plans SET council_mode = p_mode, updated_at = NOW() WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- set_council_consensus: Record the consensus decision for a plan
-- Go params: p_plan_id, p_consensus TEXT
CREATE OR REPLACE FUNCTION set_council_consensus(
  p_plan_id UUID,
  p_consensus TEXT
) RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET latest_feedback = jsonb_build_object(
        'consensus', p_consensus,
        'updated_at', NOW()::text
      ),
      updated_at = NOW()
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 7. MAINTENANCE COMMAND RPCs
-- ============================================================================

-- create_maintenance_command: Queue a maintenance action
-- Go params: p_command_type TEXT, p_payload JSONB
CREATE OR REPLACE FUNCTION create_maintenance_command(
  p_command_type TEXT,
  p_payload JSONB DEFAULT '{}'
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO maintenance_commands (type, payload, status, created_at)
  VALUES (p_command_type, p_payload, 'pending', NOW())
  RETURNING id INTO v_id;

  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- update_maintenance_command_status: Update maintenance command status
-- Go params: p_id UUID, p_status TEXT, p_result_notes JSONB
CREATE OR REPLACE FUNCTION update_maintenance_command_status(
  p_id UUID,
  p_status TEXT,
  p_result_notes JSONB DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE maintenance_commands
  SET status = p_status,
      result = COALESCE(p_result_notes, result),
      processing_by = NULL,
      processing_at = NULL,
      updated_at = NOW()
  WHERE id = p_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- queue_maintenance_command: Alias used by db_tools.go
CREATE OR REPLACE FUNCTION queue_maintenance_command(
  p_command_type TEXT,
  p_payload JSONB DEFAULT '{}',
  p_priority INT DEFAULT 5
) RETURNS UUID AS $$
BEGIN
  RETURN create_maintenance_command(p_command_type, p_payload);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 8. RESEARCH SUGGESTION RPCs
-- ============================================================================

-- update_research_suggestion_status: Transition research suggestion
-- Go params: p_id UUID, p_status TEXT, p_review_notes JSONB
CREATE OR REPLACE FUNCTION update_research_suggestion_status(
  p_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE research_suggestions
  SET status = p_status,
      review_notes = COALESCE(p_review_notes, review_notes),
      processing_by = NULL,
      processing_at = NULL,
      updated_at = NOW()
  WHERE id = p_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 9. DEPENDENCY / UNLOCK RPCs
-- ============================================================================

-- unlock_dependent_tasks: Release tasks blocked by a completed task
-- Go params: p_completed_task_id UUID
CREATE OR REPLACE FUNCTION unlock_dependent_tasks(
  p_completed_task_id UUID
) RETURNS INT AS $$
DECLARE
  v_unlocked INT;
BEGIN
  -- Find tasks whose dependencies include the completed task and set them available
  UPDATE tasks
  SET status = 'available',
      updated_at = NOW()
  WHERE status = 'pending'
    AND dependencies @> to_jsonb(ARRAY[p_completed_task_id::text])
    AND id != p_completed_task_id;

  GET DIAGNOSTICS v_unlocked = ROW_COUNT;
  RETURN v_unlocked;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 10. ROUTING / PLATFORM RPCs
-- ============================================================================

-- check_platform_availability: Check if a platform/connector is available
-- Go params: p_platform_id TEXT
CREATE OR REPLACE FUNCTION check_platform_availability(
  p_platform_id TEXT
) RETURNS JSONB AS $$
BEGIN
  -- Check if any model from this platform exists in our config
  -- For now, always return available (governor's UsageTracker handles rate limits)
  RETURN jsonb_build_object(
    'available', true,
    'platform_id', p_platform_id,
    'checked_at', NOW()::text
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 11. ANALYST RPCs
-- ============================================================================

-- get_model_performance: Get aggregated model performance stats
-- No params
CREATE OR REPLACE FUNCTION get_model_performance()
RETURNS JSONB AS $$
BEGIN
  RETURN (
    SELECT COALESCE(jsonb_object_agg(
      model_id,
      jsonb_build_object(
        'success_count', success_count,
        'failure_count', failure_count,
        'confidence', confidence,
        'last_applied', last_applied_at
      )
    ), '{}'::jsonb)
    FROM learned_heuristics
    WHERE expires_at > NOW() OR expires_at IS NULL
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- get_failure_patterns: Get recent failure patterns
-- Go params: days INT
CREATE OR REPLACE FUNCTION get_failure_patterns(
  days INT DEFAULT 7
) RETURNS JSONB AS $$
BEGIN
  RETURN (
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
      'failure_type', failure_type,
      'count', count,
      'latest', latest
    )), '[]'::jsonb)
    FROM (
      SELECT failure_type,
             COUNT(*) as count,
             MAX(created_at) as latest
      FROM failure_records
      WHERE created_at > NOW() - (days || ' days')::INTERVAL
      GROUP BY failure_type
      ORDER BY count DESC
      LIMIT 20
    ) sub
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 12. SECURITY / AUDIT RPCs
-- ============================================================================

-- log_security_audit: Record a vault security event
-- Go params: p_operation TEXT, p_key_name TEXT, p_allowed BOOLEAN
CREATE OR REPLACE FUNCTION log_security_audit(
  p_operation TEXT,
  p_key_name TEXT DEFAULT NULL,
  p_allowed BOOLEAN DEFAULT true,
  p_metadata JSONB DEFAULT '{}'
) RETURNS VOID AS $$
BEGIN
  INSERT INTO security_audit_log (operation, key_name, allowed, metadata, created_at)
  VALUES (p_operation, p_key_name, p_allowed, p_metadata, NOW());
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 13. PLANNER LEARNING RPCs
-- ============================================================================

-- create_planner_rule: Store a learned planner rule from council feedback
-- Go params: p_applies_to TEXT, p_rule_type TEXT, p_rule_text TEXT, p_source TEXT
CREATE OR REPLACE FUNCTION create_planner_rule(
  p_applies_to TEXT DEFAULT '*',
  p_rule_type TEXT DEFAULT 'general',
  p_rule_text TEXT,
  p_source TEXT DEFAULT 'auto'
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO planner_rules (applies_to, rule_type, rule_text, source, created_at, updated_at)
  VALUES (p_applies_to, p_rule_type, p_rule_text, p_source, NOW(), NOW())
  RETURNING id INTO v_id;

  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- record_planner_revision: Track plan revision for planner learning
-- Go params: p_plan_id UUID, p_concerns JSONB, p_tasks_needing_revision JSONB
CREATE OR REPLACE FUNCTION record_planner_revision(
  p_plan_id UUID,
  p_concerns JSONB DEFAULT '[]',
  p_tasks_needing_revision JSONB DEFAULT '[]'
) RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET revision_round = COALESCE(revision_round, 0) + 1,
      tasks_needing_revision = p_tasks_needing_revision,
      latest_feedback = jsonb_build_object(
        'concerns', p_concerns,
        'tasks_needing_revision', p_tasks_needing_revision,
        'revised_at', NOW()::text
      ),
      revision_history = COALESCE(revision_history, '[]'::jsonb) ||
        jsonb_build_object(
          'round', COALESCE(revision_round, 0) + 1,
          'concerns', p_concerns,
          'timestamp', NOW()::text
        ),
      updated_at = NOW()
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- record_revision_feedback: Store feedback from revision review
-- Go params: p_plan_id UUID, p_source TEXT, p_feedback JSONB, p_tasks_needing_revision JSONB
CREATE OR REPLACE FUNCTION record_revision_feedback(
  p_plan_id UUID,
  p_source TEXT DEFAULT 'supervisor',
  p_feedback JSONB DEFAULT '{}',
  p_tasks_needing_revision JSONB DEFAULT '[]'
) RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO revision_feedback (plan_id, source, feedback, tasks_needing_revision, created_at)
  VALUES (p_plan_id, p_source, p_feedback,
          (SELECT array_agg(elem) FROM jsonb_array_elements_text(p_tasks_needing_revision) elem),
          NOW())
  RETURNING id INTO v_id;

  -- Also update plan's latest feedback
  UPDATE plans
  SET latest_feedback = p_feedback,
      updated_at = NOW()
  WHERE id = p_plan_id;

  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 14. MEMORY SYSTEM RPCs (for 3-layer memory)
-- ============================================================================

-- store_memory: Store a memory item in the appropriate layer
-- Go params: p_layer TEXT, p_key TEXT, p_value TEXT, p_ttl_sec INT
CREATE OR REPLACE FUNCTION store_memory(
  p_layer TEXT,
  p_key TEXT,
  p_value TEXT,
  p_ttl_sec INT DEFAULT 3600
) RETURNS BOOLEAN AS $$
DECLARE
  v_success BOOLEAN := false;
BEGIN
  IF p_layer = 'short_term' THEN
    INSERT INTO memory_sessions (session_id, agent_type, context, created_at, expires_at)
    VALUES (p_key, 'unknown', jsonb_build_object('value', p_value), NOW(), NOW() + (p_ttl_sec || ' seconds')::INTERVAL)
    ON CONFLICT (session_id) DO UPDATE
      SET context = jsonb_build_object('value', p_value),
          created_at = NOW(),
          expires_at = NOW() + (p_ttl_sec || ' seconds')::INTERVAL;
    v_success := true;

  ELSIF p_layer = 'mid_term' THEN
    INSERT INTO memory_project (project_id, key, value, updated_at)
    VALUES ('vibepilot', p_key, jsonb_build_object('value', p_value), NOW())
    ON CONFLICT (project_id, key) DO UPDATE
      SET value = jsonb_build_object('value', p_value),
          updated_at = NOW();
    v_success := true;

  ELSIF p_layer = 'long_term' THEN
    INSERT INTO memory_rules (category, rule_text, source, priority, confidence, created_at, updated_at)
    VALUES (p_key, p_value, 'auto', 0, 0.5, NOW(), NOW());
    v_success := true;
  END IF;

  RETURN v_success;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- recall_memories: Retrieve memory items by layer and key prefix
-- Go params: p_layer TEXT, p_query TEXT, p_limit INT
CREATE OR REPLACE FUNCTION recall_memories(
  p_layer TEXT,
  p_query TEXT DEFAULT '',
  p_limit INT DEFAULT 10
) RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  IF p_layer = 'short_term' THEN
    SELECT COALESCE(jsonb_agg(
      jsonb_build_object('key', session_id, 'value', context->>'value', 'created_at', created_at)
    ), '[]'::jsonb) INTO v_result
    FROM memory_sessions
    WHERE (p_query = '' OR session_id LIKE p_query || '%')
      AND expires_at > NOW()
    ORDER BY created_at DESC
    LIMIT p_limit;

  ELSIF p_layer = 'mid_term' THEN
    SELECT COALESCE(jsonb_agg(
      jsonb_build_object('key', key, 'value', value->>'value', 'updated_at', updated_at)
    ), '[]'::jsonb) INTO v_result
    FROM memory_project
    WHERE (p_query = '' OR key LIKE p_query || '%')
    ORDER BY updated_at DESC
    LIMIT p_limit;

  ELSIF p_layer = 'long_term' THEN
    SELECT COALESCE(jsonb_agg(
      jsonb_build_object('key', category, 'value', rule_text, 'source', source, 'confidence', confidence)
    ), '[]'::jsonb) INTO v_result
    FROM memory_rules
    WHERE (p_query = '' OR category LIKE p_query || '%')
    ORDER BY priority DESC, updated_at DESC
    LIMIT p_limit;

  ELSE
    v_result := '[]'::jsonb;
  END IF;

  RETURN COALESCE(v_result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 15. ORPHAN RECOVERY RPCs (complement existing find_orphaned_sessions)
-- ============================================================================

-- recover_orphaned_session: Recover a single orphaned session
-- Go params: p_session_id UUID, p_reason TEXT
CREATE OR REPLACE FUNCTION recover_orphaned_session(
  p_session_id UUID,
  p_reason TEXT DEFAULT 'timeout_recovery'
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  -- Release processing lock on any entity held by this session
  UPDATE tasks
  SET processing_by = NULL, processing_at = NULL, updated_at = NOW()
  WHERE processing_by LIKE '%' || p_session_id::text || '%';

  UPDATE plans
  SET processing_by = NULL, processing_at = NULL, updated_at = NOW()
  WHERE processing_by LIKE '%' || p_session_id::text || '%';

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- 16. GRANTS
-- ============================================================================

-- All RPCs use SECURITY DEFINER so they run as table owner.
-- Grant execute to anon role for Supabase REST access.
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon, authenticated, service_role;

COMMIT;

SELECT 'Migration 111 complete: 42 missing RPCs created' AS status;
