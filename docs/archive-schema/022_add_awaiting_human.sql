-- Add awaiting_human status for tasks requiring human approval
-- Triggers: UI/UX visual changes, council recommendations, credit issues

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'escalated', 'awaiting_human'
  ));

-- Add comment for documentation
COMMENT ON CONSTRAINT tasks_status_check ON tasks IS 
'Status flow: pending → available → in_progress → review → testing → approval → merged
Special states: escalated (3+ failures), awaiting_human (needs human decision)';

SELECT 'awaiting_human status added to tasks table' AS status;
