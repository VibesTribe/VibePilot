-- VibePilot Migration 075: Add merge_pending status
-- System handles merge failures automatically, no human needed

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;

ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN ('pending','available','in_progress','review','testing','approval','merged','merge_pending','escalated','blocked'));

SELECT 'Migration 075 complete - added merge_pending status' AS status;
