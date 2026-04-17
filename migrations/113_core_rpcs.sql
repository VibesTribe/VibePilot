-- Migration 113: Create ALL missing RPCs the governor needs
-- These are the functions the Go governor calls. Tables already exist.
-- Each function is DROP IF EXISTS first to avoid the "cannot change return type" error.

-- ========================================
-- REQUIRED: Core pipeline (plan → task → review)
-- ========================================

-- 1. update_plan_status
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT, TEXT) CASCADE;
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT, JSONB) CASCADE;
DROP FUNCTION IF EXISTS update_plan_status(UUID, TEXT, JSONB, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION update_plan_status(
  p_plan_id UUID,
  p_status TEXT,
  p_plan_path TEXT DEFAULT NULL,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  UPDATE plans
  SET status = p_status,
      plan_path = COALESCE(p_plan_path, plan_path),
      processing_by = NULL,
      processing_at = NULL,
      review_notes = COALESCE(p_review_notes, review_notes),
      updated_at = NOW(),
      approved_at = CASE WHEN p_status = 'approved' THEN NOW() ELSE approved_at END
  WHERE id = p_plan_id;

  IF p_status = 'approved' THEN
    UPDATE tasks SET status = 'available', updated_at = NOW()
    WHERE plan_id = p_plan_id AND status = 'pending';
  END IF;

  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. claim_task
DROP FUNCTION IF EXISTS claim_task(UUID, TEXT, TEXT, TEXT, TEXT, TEXT) CASCADE;
DROP FUNCTION IF EXISTS claim_task(UUID, TEXT, TEXT, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION claim_task(
  p_task_id UUID,
  p_agent_id TEXT,
  p_model_id TEXT,
  p_connector_id TEXT,
  p_session_id TEXT DEFAULT NULL,
  p_branch_name TEXT DEFAULT NULL
)
RETURNS TABLE(id UUID, title TEXT, task_number INT, status TEXT, slice_id UUID, description TEXT, acceptance_criteria TEXT, prompt_packet TEXT, plan_id UUID) AS $$
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_agent_id,
      model_id = p_model_id,
      connector_id = p_connector_id,
      session_id = COALESCE(p_session_id, session_id),
      branch_name = COALESCE(p_branch_name, branch_name),
      started_at = NOW(),
      updated_at = NOW()
  WHERE id = p_task_id AND status = 'available'
  RETURNING tasks.id, tasks.title, tasks.task_number, tasks.status, tasks.slice_id,
            tasks.description, tasks.acceptance_criteria, tasks.prompt_packet, tasks.plan_id
  INTO id, title, task_number, status, slice_id, description, acceptance_criteria, prompt_packet, plan_id;

  IF id IS NULL THEN
    RAISE EXCEPTION 'Task % not available for claiming', p_task_id;
  END IF;

  RETURN NEXT;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. transition_task
DROP FUNCTION IF EXISTS transition_task(UUID, TEXT, TEXT) CASCADE;
DROP FUNCTION IF EXISTS transition_task(UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION transition_task(
  p_task_id UUID,
  p_new_status TEXT,
  p_reason TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_old_status TEXT;
  v_updated INT;
BEGIN
  SELECT status INTO v_old_status FROM tasks WHERE id = p_task_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'Task % not found', p_task_id;
  END IF;

  UPDATE tasks
  SET status = p_new_status,
      updated_at = NOW(),
      completed_at = CASE WHEN p_new_status IN ('completed', 'failed', 'cancelled') THEN NOW() ELSE completed_at END
  WHERE id = p_task_id;

  GET DIAGNOSTICS v_updated = ROW_COUNT;

  -- Log the transition
  INSERT INTO orchestrator_events (event_type, payload)
  VALUES ('task_transition', jsonb_build_object(
    'task_id', p_task_id,
    'old_status', v_old_status,
    'new_status', p_new_status,
    'reason', p_reason,
    'timestamp', NOW()
  ));

  -- Unlock dependents if completed
  IF p_new_status = 'completed' THEN
    PERFORM unlock_dependent_tasks(p_task_id);
  END IF;

  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. create_task_run
DROP FUNCTION IF EXISTS create_task_run(UUID, TEXT, TEXT, TEXT, TEXT, JSONB, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION create_task_run(
  p_task_id UUID,
  p_status TEXT,
  p_model_id TEXT,
  p_connector_id TEXT,
  p_session_id TEXT DEFAULT NULL,
  p_result JSONB DEFAULT NULL,
  p_branch_name TEXT DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_run_id UUID;
BEGIN
  INSERT INTO task_runs (task_id, status, model_id, connector_id, session_id, result, branch_name, started_at)
  VALUES (p_task_id, p_status, p_model_id, p_connector_id, p_session_id, p_result, p_branch_name, NOW())
  RETURNING id INTO v_run_id;
  RETURN v_run_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. record_model_success
DROP FUNCTION IF EXISTS record_model_success(TEXT, TEXT, UUID) CASCADE;
CREATE OR REPLACE FUNCTION record_model_success(
  p_model_id TEXT,
  p_connector_id TEXT DEFAULT NULL,
  p_task_id UUID DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO model_scores (model_id, success_count, total_count, updated_at)
  VALUES (p_model_id, 1, 1, NOW())
  ON CONFLICT (model_id) DO UPDATE SET
    success_count = model_scores.success_count + 1,
    total_count = model_scores.total_count + 1,
    updated_at = NOW();
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6. record_model_failure
DROP FUNCTION IF EXISTS record_model_failure(TEXT, TEXT, UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION record_model_failure(
  p_model_id TEXT,
  p_connector_id TEXT DEFAULT NULL,
  p_task_id UUID DEFAULT NULL,
  p_error TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO model_scores (model_id, failure_count, total_count, updated_at)
  VALUES (p_model_id, 1, 1, NOW())
  ON CONFLICT (model_id) DO UPDATE SET
    failure_count = model_scores.failure_count + 1,
    total_count = model_scores.total_count + 1,
    updated_at = NOW();

  -- Also record the failure
  IF p_task_id IS NOT NULL THEN
    INSERT INTO failure_records (task_id, model_id, error_message, created_at)
    VALUES (p_task_id, p_model_id, p_error, NOW());
  END IF;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 7. set_processing
DROP FUNCTION IF EXISTS set_processing(UUID, TEXT, TEXT) CASCADE;
DROP FUNCTION IF EXISTS set_processing(TEXT, UUID, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION set_processing(
  p_table_name TEXT,
  p_record_id UUID,
  p_agent_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  EXECUTE format('UPDATE %I SET processing_by = %L, processing_at = NOW() WHERE id = %L AND processing_by IS NULL',
    p_table_name, p_agent_id, p_record_id);
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 8. clear_processing
DROP FUNCTION IF EXISTS clear_processing(UUID, TEXT) CASCADE;
DROP FUNCTION IF EXISTS clear_processing(TEXT, UUID) CASCADE;
CREATE OR REPLACE FUNCTION clear_processing(
  p_table_name TEXT,
  p_record_id UUID
)
RETURNS VOID AS $$
BEGIN
  EXECUTE format('UPDATE %I SET processing_by = NULL, processing_at = NULL WHERE id = %L',
    p_table_name, p_record_id);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 9. record_performance_metric
DROP FUNCTION IF EXISTS record_performance_metric(TEXT, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION record_performance_metric(
  p_metric_type TEXT,
  p_metric_name TEXT,
  p_value JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO performance_metrics (metric_type, metric_name, value, created_at)
  VALUES (p_metric_type, p_metric_name, p_value, NOW());
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 10. record_failure
DROP FUNCTION IF EXISTS record_failure(UUID, TEXT, TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION record_failure(
  p_task_id UUID,
  p_model_id TEXT DEFAULT NULL,
  p_error_type TEXT DEFAULT NULL,
  p_error_message TEXT DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO failure_records (task_id, model_id, error_type, error_message, created_at)
  VALUES (p_task_id, p_model_id, p_error_type, p_error_message, NOW())
  RETURNING id INTO v_id;
  RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ========================================
-- REQUIRED: calculate_run_costs (used by task handler)
-- ========================================

DROP FUNCTION IF EXISTS calculate_run_costs(UUID) CASCADE;
CREATE OR REPLACE FUNCTION calculate_run_costs(p_run_id UUID)
RETURNS TABLE(total_cost_usd NUMERIC, input_tokens INT, output_tokens INT) AS $$
BEGIN
  RETURN QUERY
  SELECT
    COALESCE(SUM((r.result->>'cost_usd')::NUMERIC), 0) AS total_cost_usd,
    COALESCE(SUM((r.result->>'tokens_in')::INT), 0) AS input_tokens,
    COALESCE(SUM((r.result->>'tokens_out')::INT), 0) AS output_tokens
  FROM task_runs r
  WHERE r.id = p_run_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ========================================
-- MISSING TABLES needed by the RPCs above
-- ========================================

CREATE TABLE IF NOT EXISTS model_scores (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id TEXT NOT NULL UNIQUE,
  success_count INT DEFAULT 0,
  failure_count INT DEFAULT 0,
  total_count INT DEFAULT 0,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS performance_metrics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  metric_type TEXT NOT NULL,
  metric_name TEXT NOT NULL,
  value JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS failure_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id),
  model_id TEXT,
  error_type TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ========================================
-- GRANT permissions
-- ========================================
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;

SELECT 'Migration 113 complete - all 11 core RPCs created' AS status;
