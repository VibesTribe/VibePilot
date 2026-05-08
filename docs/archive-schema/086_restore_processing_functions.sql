-- Migration 086: Restore set_processing/clear_processing for plans
-- Purpose: 084 dropped these but plan handlers still need them
-- Date: 2026-03-11

CREATE OR REPLACE FUNCTION set_processing(
  p_id UUID,
  p_processing_by TEXT,
  p_table TEXT
) RETURNS BOOLEAN AS $$
DECLARE v_updated INT;
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans SET processing_by = p_processing_by, processing_at = NOW(), updated_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'maintenance_commands' THEN
    UPDATE maintenance_commands SET processing_by = p_processing_by, processing_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSIF p_table = 'research_suggestions' THEN
    UPDATE research_suggestions SET processing_by = p_processing_by, processing_at = NOW()
    WHERE id = p_id AND (processing_by IS NULL OR processing_at < NOW() - INTERVAL '10 minutes');
  ELSE RAISE EXCEPTION 'Invalid table: %', p_table;
  END IF;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION clear_processing(
  p_id UUID,
  p_table TEXT
) RETURNS BOOLEAN AS $$
DECLARE v_updated INT;
BEGIN
  IF p_table = 'plans' THEN
    UPDATE plans SET processing_by = NULL, processing_at = NULL, updated_at = NOW() WHERE id = p_id;
  ELSIF p_table = 'maintenance_commands' THEN
    UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE id = p_id;
  ELSIF p_table = 'research_suggestions' THEN
    UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE id = p_id;
  ELSE RAISE EXCEPTION 'Invalid table: %', p_table;
  END IF;
  GET DIAGNOSTICS v_updated = ROW_COUNT;
  RETURN v_updated > 0;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

SELECT '086_restore_processing_functions applied' AS status;
