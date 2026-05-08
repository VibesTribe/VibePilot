-- Slice-based task numbering to prevent branch collisions
-- Each slice (module) has its own task sequence: T001, T002, T003...
-- Prevents collisions when multiple plans create tasks with same number in same slice

CREATE OR REPLACE FUNCTION get_next_task_number_for_slice(p_slice_id TEXT)
RETURNS TEXT AS $$
DECLARE
    v_next_num INTEGER;
    v_task_number TEXT;
BEGIN
    -- Get the highest task number for this slice
    -- Extract numeric part from task_number (e.g., "T001" -> 1)
    SELECT COALESCE(MAX(CAST(SUBSTRING(task_number FROM 2) AS INTEGER)), 0)
    INTO v_next_num
    FROM tasks
    WHERE slice_id = p_slice_id
      AND task_number ~ '^T\d{3}$';  -- Only count T001, T002, etc.

    -- Increment for next task
    v_next_num := v_next_num + 1;

    -- Format as T001, T002, etc.
    v_task_number := 'T' || LPAD(v_next_num::TEXT, 3, '0');

    RETURN v_task_number;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Grant execute permission
GRANT EXECUTE ON FUNCTION get_next_task_number_for_slice(TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_next_task_number_for_slice(TEXT) TO anon;

-- Add comment
COMMENT ON FUNCTION get_next_task_number_for_slice(TEXT) IS 'Get the next sequential task number for a slice/module. Returns T001, T002, T003 etc. Each slice tracks its own sequence independently.';
