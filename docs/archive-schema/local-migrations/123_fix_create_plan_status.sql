-- Migration 123: Fix create_plan RPC - use 'draft' status instead of 'pending'
-- The plans table CHECK constraint allows: draft, council_review, revision_needed, pending_human, approved, archived
-- The RPC was inserting with 'pending' which violates the constraint.

CREATE OR REPLACE FUNCTION create_plan(
  p_project_id UUID,
  p_prd_path TEXT,
  p_plan_path TEXT DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
  v_plan_id UUID;
BEGIN
  INSERT INTO plans (
    project_id, prd_path, plan_path, status, created_at, updated_at
  ) VALUES (
    p_project_id, p_prd_path, p_plan_path, 'draft', NOW(), NOW()
  ) RETURNING id INTO v_plan_id;

  RETURN v_plan_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
