-- VibePilot Migration 057: Task Checkpoints
-- Purpose: Enable checkpointing for long-running task execution
-- 
-- Minimal implementation for Phase 5 core wiring

-- ============================================================================
-- 1. TASK_CHECKPOINTS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS task_checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  
  -- Checkpoint data
  step TEXT NOT NULL,
  progress INT NOT NULL CHECK (progress >= 0 AND progress <= 100),
  output TEXT,
  files JSONB DEFAULT '[]',
  
  -- Metadata
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  
  -- One checkpoint per task
  CONSTRAINT task_checkpoints_task_unique UNIQUE (task_id)
);

CREATE INDEX IF NOT EXISTS idx_task_checkpoints_task ON task_checkpoints(task_id);

COMMENT ON TABLE task_checkpoints IS 'Stores checkpoint data for task execution recovery';
COMMENT ON COLUMN task_checkpoints.step IS 'Current execution step: execution, review, testing';
COMMENT ON COLUMN task_checkpoints.progress IS 'Progress percentage 0-100';

-- ============================================================================
-- 2. RPC: save_checkpoint
-- ============================================================================

CREATE OR REPLACE FUNCTION save_checkpoint(
  p_task_id UUID,
  p_step TEXT,
  p_progress INT,
  p_output TEXT DEFAULT NULL,
  p_files JSONB DEFAULT '[]'::jsonb
) RETURNS UUID AS $$
DECLARE
  v_checkpoint_id UUID;
BEGIN
  -- Upsert checkpoint (one per task)
  INSERT INTO task_checkpoints (task_id, step, progress, output, files, updated_at)
  VALUES (p_task_id, p_step, p_progress, p_output, p_files, NOW())
  ON CONFLICT (task_id) 
  DO UPDATE SET
    step = EXCLUDED.step,
    progress = EXCLUDED.progress,
    output = EXCLUDED.output,
    files = EXCLUDED.files,
    updated_at = NOW()
  RETURNING id INTO v_checkpoint_id;
  
  RETURN v_checkpoint_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION save_checkpoint(UUID, TEXT, INT, TEXT, JSONB) IS 
'Save or update checkpoint for task execution. Called periodically during long-running tasks.';

-- ============================================================================
-- 3. RPC: load_checkpoint
-- ============================================================================

CREATE OR REPLACE FUNCTION load_checkpoint(
  p_task_id UUID
) RETURNS JSONB AS $$
DECLARE
  v_checkpoint JSONB;
BEGIN
  SELECT jsonb_build_object(
    'step', step,
    'progress', progress,
    'output', output,
    'files', files,
    'created_at', created_at,
    'updated_at', updated_at
  ) INTO v_checkpoint
  FROM task_checkpoints
  WHERE task_id = p_task_id;
  
  RETURN v_checkpoint;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION load_checkpoint(UUID) IS 
'Load checkpoint data for task. Returns NULL if no checkpoint exists.';

-- ============================================================================
-- 4. RPC: delete_checkpoint
-- ============================================================================

CREATE OR REPLACE FUNCTION delete_checkpoint(
  p_task_id UUID
) RETURNS VOID AS $$
BEGIN
  DELETE FROM task_checkpoints WHERE task_id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION delete_checkpoint(UUID) IS 
'Delete checkpoint after task completion or merge.';

-- ============================================================================
-- 5. RPC: find_tasks_with_checkpoints
-- ============================================================================

CREATE OR REPLACE FUNCTION find_tasks_with_checkpoints(
  p_statuses TEXT[] DEFAULT ARRAY['in_progress', 'review', 'testing']
) RETURNS TABLE (
  task_id UUID,
  task_number TEXT,
  title TEXT,
  status TEXT,
  step TEXT,
  progress INT,
  checkpoint_created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    t.id AS task_id,
    t.task_number,
    t.title,
    t.status,
    tc.step,
    tc.progress,
    tc.created_at AS checkpoint_created_at
  FROM tasks t
  INNER JOIN task_checkpoints tc ON t.id = tc.task_id
  WHERE t.status = ANY(p_statuses)
  ORDER BY tc.created_at ASC;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMENT ON FUNCTION find_tasks_with_checkpoints(TEXT[]) IS 
'Find tasks that have checkpoints and are in specified statuses (default: in_progress, review, testing). Used for crash recovery.';

-- ============================================================================
-- VERIFICATION
-- ============================================================================

SELECT 'Migration 057 complete - task checkpoints enabled' AS status;

-- Verify table exists
SELECT 'task_checkpoints table created' AS check FROM information_schema.tables 
  WHERE table_name = 'task_checkpoints';

-- Verify RPCs exist
SELECT 'save_checkpoint RPC created' AS check FROM information_schema.routines 
  WHERE routine_name = 'save_checkpoint';
SELECT 'load_checkpoint RPC created' AS check FROM information_schema.routines 
  WHERE routine_name = 'load_checkpoint';
SELECT 'delete_checkpoint RPC created' AS check FROM information_schema.routines 
  WHERE routine_name = 'delete_checkpoint';
