-- VibePilot Migration 041: Research Suggestions Table
-- Purpose: Store actionable research findings from System Researcher

DROP TABLE IF EXISTS research_suggestions CASCADE;

CREATE TABLE research_suggestions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Type of research finding
  type TEXT NOT NULL CHECK (type IN (
    'new_model',
    'new_platform', 
    'pricing_change',
    'config_tweak',
    'architecture',
    'new_data_store',
    'security',
    'workflow_change',
    'api_credit_exhausted',
    'ui_ux_change',
    'tool_update'
  )),
  
  -- Status flow: pending → in_review/council_review → approved/rejected/implemented
  status TEXT DEFAULT 'pending' CHECK (status IN (
    'pending',
    'in_review',
    'council_review',
    'approved',
    'rejected',
    'implemented',
    'pending_human'
  )),
  
  -- Complexity determines routing
  complexity TEXT CHECK (complexity IN ('simple', 'complex', 'human')),
  
  -- Source tracking
  source TEXT DEFAULT 'system_researcher',
  research_date DATE,
  findings_path TEXT,
  
  -- Content
  title TEXT NOT NULL,
  summary TEXT,
  details JSONB DEFAULT '{}',
  
  -- Review tracking
  reviewed_by TEXT,
  reviewed_at TIMESTAMPTZ,
  review_notes JSONB,
  
  -- Implementation tracking
  implemented_by TEXT,
  implemented_at TIMESTAMPTZ,
  maintenance_command_id UUID,
  
  -- Timestamps
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for pending items query
CREATE INDEX idx_research_suggestions_status ON research_suggestions(status);
CREATE INDEX idx_research_suggestions_type ON research_suggestions(type);

-- Trigger for updated_at
CREATE OR REPLACE FUNCTION update_research_suggestions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_research_suggestions_updated_at ON research_suggestions;
CREATE TRIGGER update_research_suggestions_updated_at
  BEFORE UPDATE ON research_suggestions
  FOR EACH ROW
  EXECUTE FUNCTION update_research_suggestions_updated_at();

-- RPC: Create research suggestion
CREATE OR REPLACE FUNCTION create_research_suggestion(
  p_type TEXT,
  p_title TEXT,
  p_summary TEXT,
  p_details JSONB DEFAULT '{}',
  p_findings_path TEXT DEFAULT NULL,
  p_research_date DATE DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_id UUID;
  v_complexity TEXT;
BEGIN
  -- Determine complexity based on type
  CASE p_type
    WHEN 'new_model' THEN v_complexity := 'simple';
    WHEN 'new_platform' THEN v_complexity := 'simple';
    WHEN 'pricing_change' THEN v_complexity := 'simple';
    WHEN 'config_tweak' THEN v_complexity := 'simple';
    WHEN 'architecture' THEN v_complexity := 'complex';
    WHEN 'new_data_store' THEN v_complexity := 'complex';
    WHEN 'security' THEN v_complexity := 'complex';
    WHEN 'workflow_change' THEN v_complexity := 'complex';
    WHEN 'api_credit_exhausted' THEN v_complexity := 'human';
    WHEN 'ui_ux_change' THEN v_complexity := 'human';
    ELSE v_complexity := 'complex';
  END CASE;
  
  INSERT INTO research_suggestions (
    type, title, summary, details, findings_path, research_date, complexity
  ) VALUES (
    p_type, p_title, p_summary, p_details, p_findings_path, 
    COALESCE(p_research_date, CURRENT_DATE), v_complexity
  ) RETURNING id INTO v_id;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- RPC: Update research suggestion status
CREATE OR REPLACE FUNCTION update_research_suggestion_status(
  p_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE research_suggestions
  SET 
    status = p_status,
    reviewed_at = NOW(),
    review_notes = COALESCE(p_review_notes, review_notes)
  WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

-- RPC: Create maintenance command from research
CREATE OR REPLACE FUNCTION create_maintenance_command(
  p_command_type TEXT,
  p_payload JSONB,
  p_source TEXT DEFAULT 'research_review',
  p_approved_by TEXT DEFAULT 'supervisor'
)
RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO maintenance_commands (
    command_type, 
    payload, 
    status,
    approved_by
  ) VALUES (
    p_command_type,
    p_payload,
    'approved',
    p_approved_by
  ) RETURNING id INTO v_id;
  
  RETURN v_id;
END;
$$ LANGUAGE plpgsql;

SELECT 'Migration 041 complete - research_suggestions table created' AS status;
