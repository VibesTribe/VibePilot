-- 043 Checkpoint Migration
-- Creates checkpoints table for state machine recovery

BEGIN;

CREATE TABLE IF NOT EXISTS checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
  step TEXT NOT NULL,
  progress INT NOT NULL,
  output TEXT,
  files JSONB DEFAULT '[]'::jsonb,
  timestamp TIMESTAMPTZ DEFAULT now(),
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_task ON checkpoints(task_id);

CREATE INDEX IF NOT EXISTS idx_checkpoints_timestamp ON checkpoints(timestamp);

-- Add checkpoint RPC functions

CREATE OR REPLACE FUNCTION save_checkpoint(
  p_task_id UUID,
  p_step TEXT,
  p_progress INT,
  p_output TEXT,
  p_files JSONB,
  p_timestamp TIMESTAMPTZ
) RETURNS VOID AS $$
BEGIN
  INSERT INTO checkpoints (id, task_id, step, progress, output, files, timestamp, created_at)
  VALUES (
    gen_random_uuid(),
    p_task_id,
    p_step,
    p_progress,
    p_output,
    p_files,
    p_timestamp,
    now()
  )
  ON CONFLICT (id) DO UPDATE SET
    step = p_step,
    progress = p_progress,
    output = p_output,
    files = p_files,
    timestamp = p_timestamp
  WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE Or REPLACE FUNCTION load_checkpoint(
  p_task_id UUID
) RETURNS JSONB AS $$
DECLARE
  v_checkpoint JSONB;
BEGIN
  SELECT to_jsonb(v) AS v_checkpoint
  FROM checkpoints
  WHERE id = p_task_id;
  
  IF NOT found THEN
    v_checkpoint := '{}';
  end if;
  
  RETURN v_checkpoint;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

Create Or REPLACE FUNCTION delete_checkpoint(
  p_task_id UUID
) RETURNS VOID AS $`
BEGIN
  DELETE FROM checkpoints WHERE id = p_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

COMMIT;
