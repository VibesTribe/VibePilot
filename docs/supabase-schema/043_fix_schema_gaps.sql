-- VibePilot Migration 043: Fix Critical Schema Gaps
-- Purpose: Fix issues found in audit
-- 
-- Issues Fixed:
-- 1. record_supervisor_rule referenced non-existent supervisor_rules table
--    (should use supervisor_learned_rules from migration 028)
-- 2. test_results table missing but referenced in events.go

-- ============================================================================
-- 1. FIX record_supervisor_rule RPC
-- ============================================================================

DROP FUNCTION IF EXISTS record_supervisor_rule(TEXT, TEXT, TEXT, JSONB);

CREATE OR REPLACE FUNCTION record_supervisor_rule(
  p_rule_text TEXT,
  p_applies_to TEXT DEFAULT 'plan_review',
  p_source TEXT DEFAULT 'supervisor',
  p_details JSONB DEFAULT '{}'::jsonb
) RETURNS UUID AS $$
DECLARE
  v_rule_id UUID;
BEGIN
  INSERT INTO supervisor_learned_rules (
    trigger_pattern,
    trigger_condition,
    action,
    reason,
    source,
    active
  ) VALUES (
    p_applies_to,
    p_details,
    'warn',
    p_rule_text,
    p_source,
    true
  )
  RETURNING id INTO v_rule_id;
  
  RETURN v_rule_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION record_supervisor_rule(TEXT, TEXT, TEXT, JSONB) IS 
'Record a learned supervisor rule for future reviews. Uses supervisor_learned_rules table.';

-- ============================================================================
-- 2. CREATE test_results TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS test_results (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Which task was tested
  task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
  task_number TEXT,
  slice_id TEXT DEFAULT 'default',
  
  -- Test configuration
  test_type TEXT NOT NULL CHECK (test_type IN ('unit', 'integration', 'lint', 'typecheck', 'visual', 'accessibility')),
  
  -- Test outcome
  status TEXT DEFAULT 'pending_review' CHECK (status IN ('pending_review', 'passed', 'failed', 'awaiting_human')),
  outcome TEXT CHECK (outcome IN ('pass', 'fail', 'partial')),
  
  -- Test details
  test_command TEXT,
  output TEXT,
  error_message TEXT,
  coverage_pct INT,
  
  -- For visual tests
  screenshots JSONB DEFAULT '[]',
  accessibility_issues JSONB DEFAULT '[]',
  human_approval_token TEXT,
  human_approved_at TIMESTAMPTZ,
  human_feedback TEXT,
  
  -- Duration tracking
  duration_ms INT,
  
  -- Timestamps
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_test_results_status ON test_results(status);
CREATE INDEX IF NOT EXISTS idx_test_results_task ON test_results(task_id);
CREATE INDEX IF NOT EXISTS idx_test_results_pending ON test_results(status) WHERE status = 'pending_review';

COMMENT ON TABLE test_results IS 'Test execution results for tasks. Created by testers, reviewed by supervisor.';
COMMENT ON COLUMN test_results.test_type IS 'Type of test: unit, integration, lint, typecheck, visual, accessibility';
COMMENT ON COLUMN test_results.status IS 'pending_review = waiting for supervisor, passed = approved, failed = needs fix, awaiting_human = visual test needs human';
COMMENT ON COLUMN test_results.outcome IS 'pass = all tests passed, fail = tests failed, partial = some passed';

-- ============================================================================
-- 3. RPC: create_test_result
-- ============================================================================

CREATE OR REPLACE FUNCTION create_test_result(
  p_task_id UUID,
  p_task_number TEXT,
  p_slice_id TEXT DEFAULT 'default',
  p_test_type TEXT DEFAULT 'unit',
  p_outcome TEXT DEFAULT 'pass',
  p_output TEXT DEFAULT NULL,
  p_error_message TEXT DEFAULT NULL,
  p_coverage_pct INT DEFAULT NULL,
  p_duration_ms INT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_result_id UUID;
BEGIN
  INSERT INTO test_results (
    task_id, task_number, slice_id, test_type, outcome,
    output, error_message, coverage_pct, duration_ms, status
  ) VALUES (
    p_task_id, p_task_number, p_slice_id, p_test_type, p_outcome,
    p_output, p_error_message, p_coverage_pct, p_duration_ms, 'pending_review'
  )
  RETURNING id INTO v_result_id;
  
  RETURN v_result_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION create_test_result(UUID, TEXT, TEXT, TEXT, TEXT, TEXT, TEXT, INT, INT) IS
'Create a test result entry for supervisor review.';

-- ============================================================================
-- 4. RPC: update_test_result_status
-- ============================================================================

CREATE OR REPLACE FUNCTION update_test_result_status(
  p_result_id UUID,
  p_status TEXT,
  p_human_feedback TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
  UPDATE test_results SET
    status = p_status,
    human_feedback = COALESCE(human_feedback, p_human_feedback),
    human_approved_at = CASE WHEN p_status = 'passed' THEN NOW() ELSE human_approved_at END,
    updated_at = NOW()
  WHERE id = p_result_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION update_test_result_status(UUID, TEXT, TEXT) IS
'Update test result status after supervisor review.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 043 complete - schema gaps fixed' AS status;

-- Verify test_results table exists
SELECT 'test_results table exists' AS check FROM information_schema.tables 
  WHERE table_name = 'test_results';

-- Verify record_supervisor_rule uses supervisor_learned_rules
SELECT 'record_supervisor_rule RPC exists' AS check FROM information_schema.routines 
  WHERE routine_name = 'record_supervisor_rule';
