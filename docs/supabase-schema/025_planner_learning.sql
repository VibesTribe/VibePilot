-- Phase 2: Planner Learning
-- Creates planner_learned_rules table and RPCs
-- Goal: Planner improves after every rejection

-- Table: planner_learned_rules
-- Stores rules learned from council/supervisor feedback
CREATE TABLE IF NOT EXISTS planner_learned_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- What situation does this apply to?
    applies_to TEXT NOT NULL,
    
    -- What did we learn?
    rule_type TEXT NOT NULL,
    rule_text TEXT NOT NULL,
    details JSONB DEFAULT '{}',
    
    -- Where did this come from?
    source TEXT NOT NULL,
    source_task_id UUID,
    source_review_type TEXT,
    
    -- How important is this?
    priority INT DEFAULT 1,
    
    -- Tracking effectiveness
    times_applied INT DEFAULT 0,
    times_prevented_issue INT DEFAULT 0,
    effectiveness_score FLOAT DEFAULT 0.5,
    
    -- Lifecycle
    active BOOLEAN DEFAULT true,
    expires_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for fast lookup
CREATE INDEX IF NOT EXISTS idx_planner_rules_applies_to ON planner_learned_rules(applies_to);
CREATE INDEX IF NOT EXISTS idx_planner_rules_active ON planner_learned_rules(active) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_planner_rules_priority ON planner_learned_rules(priority DESC);

-- RPC: Create a new planner rule
CREATE OR REPLACE FUNCTION create_planner_rule(
    p_applies_to TEXT,
    p_rule_type TEXT,
    p_rule_text TEXT,
    p_source TEXT,
    p_details JSONB DEFAULT '{}',
    p_source_task_id UUID DEFAULT NULL,
    p_source_review_type TEXT DEFAULT NULL,
    p_priority INT DEFAULT 1
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_rule_id UUID;
BEGIN
    INSERT INTO planner_learned_rules (
        applies_to,
        rule_type,
        rule_text,
        details,
        source,
        source_task_id,
        source_review_type,
        priority
    ) VALUES (
        p_applies_to,
        p_rule_type,
        p_rule_text,
        p_details,
        p_source,
        p_source_task_id,
        p_source_review_type,
        p_priority
    )
    RETURNING id INTO v_rule_id;
    
    RETURN v_rule_id;
END;
$$;

-- RPC: Get active rules for a context
CREATE OR REPLACE FUNCTION get_planner_rules(
    p_applies_to TEXT DEFAULT NULL,
    p_limit INT DEFAULT 20
)
RETURNS TABLE (
    id UUID,
    applies_to TEXT,
    rule_type TEXT,
    rule_text TEXT,
    details JSONB,
    source TEXT,
    priority INT,
    times_applied INT,
    times_prevented_issue INT,
    effectiveness_score FLOAT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        r.id,
        r.applies_to,
        r.rule_type,
        r.rule_text,
        r.details,
        r.source,
        r.priority,
        r.times_applied,
        r.times_prevented_issue,
        r.effectiveness_score
    FROM planner_learned_rules r
    WHERE r.active = true
      AND (r.expires_at IS NULL OR r.expires_at > NOW())
      AND (p_applies_to IS NULL OR r.applies_to = p_applies_to OR r.applies_to = '*')
    ORDER BY r.priority DESC, r.effectiveness_score DESC
    LIMIT p_limit;
END;
$$;

-- RPC: Mark a rule as applied (increment counter)
CREATE OR REPLACE FUNCTION record_planner_rule_applied(
    p_rule_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE planner_learned_rules
    SET times_applied = times_applied + 1,
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- RPC: Mark a rule as having prevented an issue
CREATE OR REPLACE FUNCTION record_planner_rule_prevented_issue(
    p_rule_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE planner_learned_rules
    SET times_prevented_issue = times_prevented_issue + 1,
        effectiveness_score = LEAST(1.0, effectiveness_score + 0.05),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- RPC: Deactivate a rule
CREATE OR REPLACE FUNCTION deactivate_planner_rule(
    p_rule_id UUID,
    p_reason TEXT DEFAULT NULL
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE planner_learned_rules
    SET active = false,
        details = jsonb_set(
            COALESCE(details, '{}'),
            '{deactivation_reason}',
            to_jsonb(COALESCE(p_reason, 'No reason provided'))
        ),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- RPC: Get rule effectiveness stats
CREATE OR REPLACE FUNCTION get_planner_rule_stats()
RETURNS TABLE (
    rule_type TEXT,
    total_rules INT,
    active_rules INT,
    avg_effectiveness FLOAT,
    total_applications INT,
    total_issues_prevented INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        r.rule_type,
        COUNT(*)::INT AS total_rules,
        COUNT(*) FILTER (WHERE r.active)::INT AS active_rules,
        AVG(r.effectiveness_score)::FLOAT AS avg_effectiveness,
        SUM(r.times_applied)::INT AS total_applications,
        SUM(r.times_prevented_issue)::INT AS total_issues_prevented
    FROM planner_learned_rules r
    GROUP BY r.rule_type
    ORDER BY total_rules DESC;
END;
$$;

-- RPC: Auto-create rule from rejection
-- Called when supervisor or council rejects a plan
CREATE OR REPLACE FUNCTION create_rule_from_rejection(
    p_task_id UUID,
    p_rejection_type TEXT,
    p_rejection_reason TEXT,
    p_applies_to TEXT,
    p_source TEXT
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_rule_id UUID;
    v_rule_text TEXT;
    v_rule_type TEXT;
BEGIN
    -- Determine rule type from rejection
    v_rule_type := CASE p_rejection_type
        WHEN 'hardcoded_value' THEN 'avoid_hardcoding'
        WHEN 'missing_context' THEN 'include_context'
        WHEN 'scope_creep' THEN 'limit_scope'
        WHEN 'dependency_issue' THEN 'check_dependencies'
        WHEN 'confidence_low' THEN 'improve_confidence'
        WHEN 'slice_violation' THEN 'respect_slice_boundary'
        ELSE 'general_guidance'
    END;
    
    -- Create rule text from rejection reason
    v_rule_text := 'Avoid: ' || p_rejection_reason;
    
    -- Insert rule
    INSERT INTO planner_learned_rules (
        applies_to,
        rule_type,
        rule_text,
        source,
        source_task_id,
        source_review_type,
        priority,
        details
    ) VALUES (
        p_applies_to,
        v_rule_type,
        v_rule_text,
        p_source,
        p_task_id,
        p_rejection_type,
        2,
        jsonb_build_object(
            'created_from_rejection', true,
            'rejection_type', p_rejection_type
        )
    )
    RETURNING id INTO v_rule_id;
    
    RETURN v_rule_id;
END;
$$;

-- Trigger: Update updated_at on change
CREATE OR REPLACE FUNCTION update_planner_rules_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trigger_planner_rules_updated_at ON planner_learned_rules;
CREATE TRIGGER trigger_planner_rules_updated_at
    BEFORE UPDATE ON planner_learned_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_planner_rules_updated_at();

-- Grant permissions
GRANT SELECT, INSERT, UPDATE ON planner_learned_rules TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
