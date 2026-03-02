-- VibePilot Migration 048: Add prd_incomplete status
-- Purpose: Allow plans to be marked as prd_incomplete when PRD lacks required info

ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_status_check;

ALTER TABLE plans ADD CONSTRAINT plans_status_check 
  CHECK (status IN ('draft', 'review', 'council_review', 'revision_needed', 'prd_incomplete', 'blocked', 'pending_human', 'error', 'approved', 'active', 'archived', 'cancelled'));

COMMENT ON COLUMN plans.status IS 'Plan status: draft → review → council_review/revision_needed/prd_incomplete → approved → active. prd_incomplete = PRD lacks info, blocked = revision limit reached';

SELECT 'Migration 048 complete - prd_incomplete status added' AS status;
