-- VibePilot Migration 029: Plans Table
-- Purpose: Track PRD → Plan → Approval flow before tasks reach orchestrator
-- 
-- Flow:
--   PRD → Planner creates plan (status: draft)
--   Simple → Supervisor approves → status: approved → tasks become available
--   Complex → Council review → Human approval → status: approved → tasks become available
--
-- No unapproved tasks ever reach orchestrator.

-- Plans table
CREATE TABLE IF NOT EXISTS plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id),
  
  -- GitHub source docs
  prd_path TEXT,
  plan_path TEXT,
  
  -- Status flow: draft → council_review/pending_human → approved/archived
  status TEXT DEFAULT 'draft' CHECK (status IN 
    ('draft', 'council_review', 'revision_needed', 'pending_human', 'approved', 'archived')
  ),
  
  -- Complexity flag (set by Supervisor)
  complexity TEXT CHECK (complexity IN ('simple', 'complex')),
  
  -- Council tracking
  council_round INT DEFAULT 0,
  council_consensus TEXT CHECK (council_consensus IN ('approved', 'revision_needed', 'blocked', null)),
  council_reviews JSONB DEFAULT '[]',
  
  -- Review data
  review_notes JSONB,
  human_decision TEXT,
  human_decision_at TIMESTAMPTZ,
  
  -- Version tracking (git commit hashes)
  prd_version TEXT,
  plan_version TEXT,
  
  -- Timestamps
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  last_reviewed_at TIMESTAMPTZ,
  approved_at TIMESTAMPTZ
);

-- Add plan_id to tasks (link tasks to their parent plan)
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_plans_status ON plans(status);
CREATE INDEX IF NOT EXISTS idx_plans_project ON plans(project_id);
CREATE INDEX IF NOT EXISTS idx_plans_complexity ON plans(complexity);
CREATE INDEX IF NOT EXISTS idx_tasks_plan ON tasks(plan_id);

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_plans_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS plans_updated_at ON plans;
CREATE TRIGGER plans_updated_at
  BEFORE UPDATE ON plans
  FOR EACH ROW
  EXECUTE FUNCTION update_plans_updated_at();

-- RPC: Create plan from PRD
CREATE OR REPLACE FUNCTION create_plan(
  p_project_id UUID,
  p_prd_path TEXT,
  p_plan_path TEXT DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_plan_id UUID;
BEGIN
  INSERT INTO plans (project_id, prd_path, plan_path, status)
  VALUES (p_project_id, p_prd_path, p_plan_path, 'draft')
  RETURNING id INTO v_plan_id;
  
  RETURN v_plan_id;
END;
$$ LANGUAGE plpgsql;

-- RPC: Update plan status
CREATE OR REPLACE FUNCTION update_plan_status(
  p_plan_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET 
    status = p_status,
    review_notes = COALESCE(p_review_notes, review_notes),
    updated_at = NOW(),
    approved_at = CASE WHEN p_status = 'approved' THEN NOW() ELSE approved_at END
  WHERE id = p_plan_id;
  
  -- If approved, flip all pending tasks to available
  IF p_status = 'approved' THEN
    UPDATE tasks
    SET status = 'available', updated_at = NOW()
    WHERE plan_id = p_plan_id AND status = 'pending';
  END IF;
END;
$$ LANGUAGE plpgsql;

-- RPC: Add council review
CREATE OR REPLACE FUNCTION add_council_review(
  p_plan_id UUID,
  p_round INT,
  p_lens TEXT,
  p_model_id TEXT,
  p_vote TEXT,
  p_confidence FLOAT DEFAULT NULL,
  p_concerns JSONB DEFAULT '[]',
  p_suggestions JSONB DEFAULT '[]',
  p_reasoning TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET 
    council_round = GREATEST(council_round, p_round),
    council_reviews = council_reviews || jsonb_build_object(
      'round', p_round,
      'lens', p_lens,
      'model_id', p_model_id,
      'vote', p_vote,
      'confidence', p_confidence,
      'concerns', p_concerns,
      'suggestions', p_suggestions,
      'reasoning', p_reasoning,
      'created_at', NOW()
    ),
    updated_at = NOW()
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql;

-- RPC: Set council consensus
CREATE OR REPLACE FUNCTION set_council_consensus(
  p_plan_id UUID,
  p_consensus TEXT
)
RETURNS VOID AS $$
BEGIN
  UPDATE plans
  SET 
    council_consensus = p_consensus,
    status = CASE 
      WHEN p_consensus = 'approved' THEN 'pending_human'
      WHEN p_consensus = 'revision_needed' THEN 'revision_needed'
      WHEN p_consensus = 'blocked' THEN 'pending_human'
      ELSE status
    END,
    updated_at = NOW()
  WHERE id = p_plan_id;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions
GRANT SELECT, INSERT, UPDATE ON plans TO authenticated;
GRANT EXECUTE ON FUNCTION create_plan TO authenticated;
GRANT EXECUTE ON FUNCTION update_plan_status TO authenticated;
GRANT EXECUTE ON FUNCTION add_council_review TO authenticated;
GRANT EXECUTE ON FUNCTION set_council_consensus TO authenticated;

-- Success notice
DO $$
BEGIN
  RAISE NOTICE 'Migration 029 complete:';
  RAISE NOTICE '  - plans table created';
  RAISE NOTICE '  - tasks.plan_id column added';
  RAISE NOTICE '  - RPCs: create_plan, update_plan_status, add_council_review, set_council_consensus';
END $$;
