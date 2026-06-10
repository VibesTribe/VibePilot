-- Add 'paused' and 'cancelled' to the tasks status check constraint
-- Required for governor pause/resume and kill task control endpoints.
-- Without these values, UPDATE tasks SET status='paused' violates the check constraint.

ALTER TABLE tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status = ANY (ARRAY[
    'pending',
    'in_progress', 
    'received',
    'review',
    'testing',
    'complete',
    'merge_pending',
    'merged',
    'failed',
    'human_review',
    'design_review',
    'paused',
    'cancelled'
  ]));
