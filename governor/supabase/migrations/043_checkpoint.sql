-- 043 Checkpoint Migration
-- Creates checkpoints table for state machine recovery

BEGIN;

CREATE TABLE IF NOT EXISTS checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE CASCADE UNIQUE,
  step TEXT NOT NULL,
  progress INT NOT NULL,
  output TEXT,
  files JSONB DEFAULT '[]'::jsonb,
  timestamp TIMESTAMPTZ DEFAULT now(),
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_task ON checkpoints(task_id);
CREATE INDEX IF NOT EXISTS idx_checkpoints_timestamp ON checkpoints(timestamp);

-- Save or update a checkpoint for a task
CREATE OR REPLACE FUNCTION save_checkpoint(
  p_task_id UUID,
  p_step TEXT,
  p_progress INT,
  p_output TEXT,
  p_files JSONB,
  p_timestamp TIMESTAMPTZ
) RETURNS VOID AS $$
BEGIN
  INSERT INTO checkpoints (task_id, step, progress, output, files, timestamp, created_at)
  VALUES (
    p_task_id,
    p_step,
    p_progress,
    p_output,
    p_files,
    p_timestamp,
    now()
  )
  ON CONFLICT (task_id) DO UPDATE SET
    step = EXCLUDED.step,
    progress = EXCLUDED.progress,
    output = EXCLUDED.output,
    files = EXCLUDED.files,
    timestamp = EXCLUDED.timestamp;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Load a checkpoint for a task
CREATE OR REPLACE FUNCTION load_checkpoint(
  p_task_id UUID
) RETURNS JSONB AS $$
DECLARE
  v_checkpoint JSONB;
BEGIN
  SELECT to_jsonb(c) INTO v_checkpoint
  FROM checkpoints c
  WHERE c.task_id = p_task_id;
  
  IF v_checkpoint IS NULL THEN
    RETURN '{}'::jsonb;
  END IF;
  
  RETURN v_checkpoint;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Delete a checkpoint for a task
CREATE OR REPLACE FUNCTION delete_checkpoint(
  p_task_id UUID
) RETURNS VOID AS $$
BEGIN
  DELETE FROM checkpoints WHERE task_id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMIT;
