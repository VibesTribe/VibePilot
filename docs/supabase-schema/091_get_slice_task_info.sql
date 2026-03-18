-- Migration 091: Add RPC for getting incomplete slice task info
-- Purpose: Provide planner with context about existing incomplete slices
-- so it can continue task numbering correctly

CREATE OR REPLACE FUNCTION get_slice_task_info()
RETURNS TABLE (
  slice_id TEXT,
  last_task_number TEXT,
  task_count BIGINT
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.slice_id,
    MAX(t.task_number) AS last_task_number,
    COUNT(*) AS task_count
  FROM tasks t
  WHERE t.slice_id IS NOT NULL
    AND t.slice_id != ''
    AND t.status NOT IN ('merged', 'cancelled')
  GROUP BY t.slice_id
  ORDER BY t.slice_id;
END;
$$ LANGUAGE plpgsql;
