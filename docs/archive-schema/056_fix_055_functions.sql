-- VibePilot Migration 056: Fix 055 Function Parameter Order
-- Purpose: Fix parameter ordering in record_performance_metric
-- Error: Parameters with defaults must come after parameters without defaults

-- ============================================================================
-- PART 1: Fix record_performance_metric function
-- ============================================================================

-- Drop the incorrect version
DROP FUNCTION IF EXISTS record_performance_metric;

-- Create corrected version with proper parameter order
-- Required parameters first, then optional parameters
CREATE OR REPLACE FUNCTION record_performance_metric(
  p_metric_type TEXT,
  p_duration_seconds FLOAT,
  p_success BOOLEAN,
  p_entity_id UUID DEFAULT NULL,
  p_metadata JSONB DEFAULT '{}'::jsonb
)
RETURNS UUID AS $$
DECLARE
  v_metric_id UUID;
BEGIN
  INSERT INTO performance_metrics (
    metric_type, 
    entity_id, 
    duration_seconds, 
    success, 
    metadata
  ) VALUES (
    p_metric_type, 
    p_entity_id, 
    p_duration_seconds, 
    p_success, 
    p_metadata
  )
  RETURNING id INTO v_metric_id;
  
  RETURN v_metric_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 056 complete - record_performance_metric fixed' AS status;
