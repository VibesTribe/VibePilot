-- VibePilot Migration 051: Fix task_packets table relationship
-- Purpose: Ensure task_packets has proper relationship to tasks table

-- Add task_id column if it doesn't exist
DO $$ 
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns 
    WHERE table_name = 'task_packets' AND column_name = 'task_id'
  ) THEN
    ALTER TABLE task_packets ADD COLUMN task_id UUID REFERENCES tasks(id) ON DELETE CASCADE;
    CREATE INDEX idx_task_packets_task_id ON task_packets(task_id);
  END IF;
END $$;

-- Ensure we have proper constraints
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'task_packets_task_id_fkey'
  ) THEN
    ALTER TABLE task_packets 
    ADD CONSTRAINT task_packets_task_id_fkey 
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

SELECT 'Migration 051 complete - task_packets relationship fixed' AS status;
