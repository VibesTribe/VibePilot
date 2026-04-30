-- VibePilot Migration 044: Processing State for All Tables
-- Purpose: Extend processing_by mechanism to ALL tables that have event handlers
-- 
-- Problem: Several event handlers don't claim processing, causing duplicate
-- event firing when handlers take longer than poll interval (1s).
-- 
-- Solution: Add processing_by to research_suggestions, test_results, and
-- maintenance_commands tables. Update RPCs to handle all tables.
-- 
-- Tables Covered:
--   - plans (from 042)
--   - tasks (from 042)
--   - research_suggestions (NEW)
--   - test_results (NEW)
--   - maintenance_commands (NEW)

-- ============================================================================
-- RESEARCH_SUGGESTIONS TABLE
-- ============================================================================

ALTER TABLE research_suggestions ADD COLUMN IF NOT EXISTS processing_by TEXT;
ALTER TABLE research_suggestions ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_research_suggestions_processing ON research_suggestions(processing_by) 
  WHERE processing_by IS NOT NULL;

COMMENT ON COLUMN research_suggestions.processing_by IS 'Agent working on this suggestion: "agent_type:session_id" format. NULL when idle.';
COMMENT ON COLUMN research_suggestions.processing_at IS 'Timestamp when processing started. Used for timeout recovery.';

-- ============================================================================
-- TEST_RESULTS TABLE
-- ============================================================================

ALTER TABLE test_results ADD COLUMN IF NOT EXISTS processing_by TEXT;
ALTER TABLE test_results ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_test_results_processing ON test_results(processing_by) 
  WHERE processing_by IS NOT NULL;

COMMENT ON COLUMN test_results.processing_by IS 'Agent working on this test result: "agent_type:session_id" format. NULL when idle.';
COMMENT ON COLUMN test_results.processing_at IS 'Timestamp when processing started. Used for timeout recovery.';

-- ============================================================================
-- MAINTENANCE_COMMANDS TABLE
-- ============================================================================

ALTER TABLE maintenance_commands ADD COLUMN IF NOT EXISTS processing_by TEXT;
ALTER TABLE maintenance_commands ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_maintenance_commands_processing ON maintenance_commands(processing_by) 
  WHERE processing_by IS NOT NULL;

COMMENT ON COLUMN maintenance_commands.processing_by IS 'Agent working on this command: "agent_type:session_id" format. NULL when idle.';
COMMENT ON COLUMN maintenance_commands.processing_at IS 'Timestamp when processing started. Used for timeout recovery.';

-- ============================================================================
-- RPC: SET PROCESSING (Extended to all tables)
-- ============================================================================

DROP FUNCTION IF EXISTS set_processing(TEXT, UUID, TEXT);

CREATE OR REPLACE FUNCTION set_processing(
  p_table TEXT,
  p_id UUID,
  p_processing_by TEXT
) RETURNS BOOLEAN AS $$
DECLARE
  v_updated INT;
BEGIN
  -- Validate table name to prevent SQL injection
  IF p_table NOT IN ('plans', 'tasks', 'research_suggestions', 'test_results', 'maintenance_commands') THEN
    RAISE EXCEPTION 'Invalid table name for set_processing: %', p_table;
    RETURN FALSE;
  END IF;
  
  -- Dynamic SQL with proper validation
  EXECUTE format('
    UPDATE %I 
    SET processing_by = $1, 
        processing_at = NOW(),
        updated_at = NOW()
    WHERE id = $2 AND processing_by IS NULL',
    p_table
  ) USING p_processing_by, p_id;
  
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION set_processing(TEXT, UUID, TEXT) IS 
'Atomically claim processing of a plan, task, research suggestion, test result, or maintenance command. Returns TRUE if claim succeeded (was NULL), FALSE if already claimed.';

-- ============================================================================
-- RPC: CLEAR PROCESSING (Extended to all tables)
-- ============================================================================

DROP FUNCTION IF EXISTS clear_processing(TEXT, UUID);

CREATE OR REPLACE FUNCTION clear_processing(
  p_table TEXT,
  p_id UUID
) RETURNS VOID AS $$
BEGIN
  -- Validate table name to prevent SQL injection
  IF p_table NOT IN ('plans', 'tasks', 'research_suggestions', 'test_results', 'maintenance_commands') THEN
    RAISE EXCEPTION 'Invalid table name for clear_processing: %', p_table;
    RETURN;
  END IF;
  
  -- Dynamic SQL with proper validation
  EXECUTE format('
    UPDATE %I 
    SET processing_by = NULL, 
        processing_at = NULL,
        updated_at = NOW()
    WHERE id = $1',
    p_table
  ) USING p_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION clear_processing(TEXT, UUID) IS 
'Release processing claim on a plan, task, research suggestion, test result, or maintenance command. Always succeeds even if not claimed.';

-- ============================================================================
-- RPC: FIND STALE PROCESSING (Extended to all tables)
-- ============================================================================

DROP FUNCTION IF EXISTS find_stale_processing(TEXT, INT);

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
  -- Validate table name to prevent SQL injection
  IF p_table NOT IN ('plans', 'tasks', 'research_suggestions', 'test_results', 'maintenance_commands') THEN
    RAISE EXCEPTION 'Invalid table name for find_stale_processing: %', p_table;
    RETURN;
  END IF;
  
  RETURN QUERY EXECUTE format('
    SELECT 
      t.id,
      t.processing_by,
      t.processing_at,
      EXTRACT(EPOCH FROM (NOW() - t.processing_at))::INT AS seconds_stale
    FROM %I t
    WHERE t.processing_by IS NOT NULL 
      AND t.processing_at < NOW() - ($1 || '' seconds'')::interval
    ORDER BY t.processing_at',
    p_table
  ) USING p_timeout_seconds;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION find_stale_processing(TEXT, INT) IS 
'Find items that have been processing longer than timeout. Used for recovery.';

-- ============================================================================
-- RPC: RECOVER STALE PROCESSING (Extended to all tables)
-- ============================================================================

DROP FUNCTION IF EXISTS recover_stale_processing(TEXT, UUID, TEXT);

CREATE OR REPLACE FUNCTION recover_stale_processing(
  p_table TEXT,
  p_id UUID,
  p_reason TEXT DEFAULT 'timeout_recovery'
) RETURNS VOID AS $$
BEGIN
  -- Validate table name to prevent SQL injection
  IF p_table NOT IN ('plans', 'tasks', 'research_suggestions', 'test_results', 'maintenance_commands') THEN
    RAISE EXCEPTION 'Invalid table name for recover_stale_processing: %', p_table;
    RETURN;
  END IF;
  
  -- Handle each table's specific columns for audit trail
  CASE p_table
    WHEN 'plans' THEN
      UPDATE plans 
      SET processing_by = NULL, 
          processing_at = NULL,
          updated_at = NOW(),
          review_notes = COALESCE(review_notes, '{}'::jsonb) || 
            jsonb_build_object('recovery_reason', p_reason, 'recovered_at', NOW())
      WHERE id = p_id;
      
    WHEN 'tasks' THEN
      UPDATE tasks 
      SET processing_by = NULL, 
          processing_at = NULL,
          updated_at = NOW(),
          failure_notes = COALESCE(failure_notes || E'\n', '') || 
            p_reason || ' at ' || NOW()::text
      WHERE id = p_id;
      
    WHEN 'research_suggestions' THEN
      UPDATE research_suggestions 
      SET processing_by = NULL, 
          processing_at = NULL,
          updated_at = NOW(),
          review_notes = COALESCE(review_notes, '{}'::jsonb) || 
            jsonb_build_object('recovery_reason', p_reason, 'recovered_at', NOW())
      WHERE id = p_id;
      
    WHEN 'test_results' THEN
      UPDATE test_results 
      SET processing_by = NULL, 
          processing_at = NULL,
          updated_at = NOW()
      WHERE id = p_id;
      
    WHEN 'maintenance_commands' THEN
      UPDATE maintenance_commands 
      SET processing_by = NULL, 
          processing_at = NULL,
          updated_at = NOW(),
          error_message = COALESCE(error_message || E'\n', '') || 
            p_reason || ' at ' || NOW()::text
      WHERE id = p_id;
  END CASE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION recover_stale_processing(TEXT, UUID, TEXT) IS 
'Recover a stale item by clearing processing state. Preserves status for re-pickup. Adds recovery info to audit trail.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 044 complete - processing state enabled for all tables' AS status;

-- Verify columns exist on all tables
SELECT 'research_suggestions.processing_by' AS column_check FROM information_schema.columns 
  WHERE table_name = 'research_suggestions' AND column_name = 'processing_by';
SELECT 'test_results.processing_by' AS column_check FROM information_schema.columns 
  WHERE table_name = 'test_results' AND column_name = 'processing_by';
SELECT 'maintenance_commands.processing_by' AS column_check FROM information_schema.columns 
  WHERE table_name = 'maintenance_commands' AND column_name = 'processing_by';
