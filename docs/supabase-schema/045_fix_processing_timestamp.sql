-- VibePilot Migration 045: Fix Processing Timestamp Bug
-- Purpose: Remove updated_at update from clear_processing and recover_stale_processing
-- 
-- Problem: clear_processing was setting updated_at = NOW(), which caused events
-- to re-fire because the event detector uses updated_at to check for new work.
-- 
-- Solution: Only update processing_by and processing_at, NOT updated_at.
-- The updated_at should only change when actual state changes (status, etc.).

-- ============================================================================
-- RPC: CLEAR PROCESSING (Fixed - no updated_at)
-- ============================================================================

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
  
  -- Dynamic SQL - only clear processing, don't touch updated_at
  EXECUTE format('
    UPDATE %I 
    SET processing_by = NULL, 
        processing_at = NULL
    WHERE id = $1',
    p_table
  ) USING p_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION clear_processing(TEXT, UUID) IS 
'Release processing claim. Does NOT update updated_at to prevent event re-firing.';

-- ============================================================================
-- RPC: RECOVER STALE PROCESSING (Fixed - no updated_at)
-- ============================================================================

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
  -- Note: We update updated_at ONLY when adding audit info, not just clearing processing
  CASE p_table
    WHEN 'plans' THEN
      UPDATE plans 
      SET processing_by = NULL, 
          processing_at = NULL,
          review_notes = COALESCE(review_notes, '{}'::jsonb) || 
            jsonb_build_object('recovery_reason', p_reason, 'recovered_at', NOW())
      WHERE id = p_id;
      
    WHEN 'tasks' THEN
      UPDATE tasks 
      SET processing_by = NULL, 
          processing_at = NULL,
          failure_notes = COALESCE(failure_notes || E'\n', '') || 
            p_reason || ' at ' || NOW()::text
      WHERE id = p_id;
      
    WHEN 'research_suggestions' THEN
      UPDATE research_suggestions 
      SET processing_by = NULL, 
          processing_at = NULL,
          review_notes = COALESCE(review_notes, '{}'::jsonb) || 
            jsonb_build_object('recovery_reason', p_reason, 'recovered_at', NOW())
      WHERE id = p_id;
      
    WHEN 'test_results' THEN
      UPDATE test_results 
      SET processing_by = NULL, 
          processing_at = NULL
      WHERE id = p_id;
      
    WHEN 'maintenance_commands' THEN
      UPDATE maintenance_commands 
      SET processing_by = NULL, 
          processing_at = NULL,
          error_message = COALESCE(error_message || E'\n', '') || 
            p_reason || ' at ' || NOW()::text
      WHERE id = p_id;
  END CASE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION recover_stale_processing(TEXT, UUID, TEXT) IS 
'Recover a stale item. Does NOT update updated_at to prevent event re-firing. Adds recovery info to audit trail.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 045 complete - processing timestamp bug fixed' AS status;
