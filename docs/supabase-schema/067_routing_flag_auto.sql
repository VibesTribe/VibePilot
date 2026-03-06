-- VibePilot Migration 067: Add 'auto' to routing_flag check constraint
-- Run in Supabase SQL Editor

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_routing_flag_check;

ALTER TABLE tasks ADD CONSTRAINT tasks_routing_flag_check 
  CHECK (routing_flag IN ('internal', 'web', 'mcp', 'auto'));

SELECT 'Migration 067 complete - routing_flag now accepts auto' AS status;
