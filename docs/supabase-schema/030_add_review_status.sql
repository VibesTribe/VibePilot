-- VibePilot Migration 030: Add Review Status
-- Purpose: Add 'review' status to plans table for clean flow:
--   draft (planner working) → review (ready for supervisor) → [council_review] → approved
--
-- This separates "planner is creating the plan" from "plan is ready for review"

-- Update the status constraint to include 'review'
ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_status_check;

ALTER TABLE plans ADD CONSTRAINT plans_status_check 
  CHECK (status IN ('draft', 'review', 'council_review', 'revision_needed', 'pending_human', 'approved', 'archived'));

-- Add comment documenting the flow
COMMENT ON COLUMN plans.status IS 'Plan status flow: draft → review → council_review/revision_needed/pending_human → approved → archived';
