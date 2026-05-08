-- VibePilot Migration 046: Add 'blocked' and 'error' to plans status
-- Purpose: Allow plans to transition to 'blocked' (revision limit) and 'error' states

-- Drop and recreate constraint with new statuses
ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_status_check;

ALTER TABLE plans ADD CONSTRAINT plans_status_check 
  CHECK (status IN ('draft', 'review', 'council_review', 'revision_needed', 'blocked', 'pending_human', 'error', 'approved', 'active', 'archived', 'cancelled'));

-- Update comment
COMMENT ON COLUMN plans.status IS 'Plan status: draft → review → council_review/revision_needed/blocked → approved → active. blocked = revision limit reached, error = task creation failed';

SELECT 'Migration 046 complete - blocked and error statuses added' AS status;
