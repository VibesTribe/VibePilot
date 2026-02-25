-- VibePilot Migration: Tester/Supervisor Learning (Phase 3)
-- Version: 028
-- Purpose: Learn from test results and supervisor rejections
-- Design: Track what tests catch bugs, what patterns flag issues
--
-- Run in Supabase SQL Editor

BEGIN;

-- ============================================================================
-- 1. TESTER LEARNED RULES
-- What tests catch bugs - learned from test failures
-- ============================================================================

CREATE TABLE IF NOT EXISTS tester_learned_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- When does this apply?
    applies_to TEXT NOT NULL,
    
    -- What test to run
    test_type TEXT NOT NULL,
    test_command TEXT NOT NULL,
    trigger_pattern TEXT,
    priority INT DEFAULT 5,
    
    -- Effectiveness tracking
    caught_bugs INT DEFAULT 0,
    false_positives INT DEFAULT 0,
    last_caught_at TIMESTAMPTZ,
    
    -- Source tracking
    source TEXT NOT NULL,
    source_task_id UUID,
    source_details JSONB DEFAULT '{}',
    
    -- Lifecycle
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tester_rules_applies ON tester_learned_rules(applies_to) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_tester_rules_priority ON tester_learned_rules(priority DESC) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_tester_rules_effectiveness ON tester_learned_rules(caught_bugs DESC) WHERE active = true;

COMMENT ON TABLE tester_learned_rules IS 'Tests that have proven to catch bugs';
COMMENT ON COLUMN tester_learned_rules.test_type IS 'Category: unit, lint, integration, typecheck, visual';
COMMENT ON COLUMN tester_learned_rules.trigger_pattern IS 'Keyword/regex that suggests running this test';

-- ============================================================================
-- 2. SUPERVISOR LEARNED RULES
-- What patterns in code flag issues - learned from rejections
-- ============================================================================

CREATE TABLE IF NOT EXISTS supervisor_learned_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- What triggers this rule?
    trigger_pattern TEXT NOT NULL,
    trigger_condition JSONB DEFAULT '{}',
    
    -- What action to take?
    action TEXT NOT NULL,
    reason TEXT NOT NULL,
    
    -- Effectiveness tracking
    times_triggered INT DEFAULT 0,
    times_caught_issue INT DEFAULT 0,
    false_positive_count INT DEFAULT 0,
    last_triggered_at TIMESTAMPTZ,
    
    -- Source tracking
    source TEXT NOT NULL,
    source_task_id UUID,
    
    -- Lifecycle
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sup_rules_pattern ON supervisor_learned_rules(trigger_pattern) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_sup_rules_action ON supervisor_learned_rules(action) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_sup_rules_effectiveness ON supervisor_learned_rules(times_caught_issue DESC) WHERE active = true;

COMMENT ON TABLE supervisor_learned_rules IS 'Patterns that have proven to flag issues in code';
COMMENT ON COLUMN supervisor_learned_rules.action IS 'Action to take: reject, warn, human_review';

-- ============================================================================
-- 3. RPC: get_tester_rules
-- Get active tester rules for a context
-- ============================================================================

CREATE OR REPLACE FUNCTION get_tester_rules(
    p_applies_to TEXT DEFAULT NULL,
    p_limit INT DEFAULT 20
)
RETURNS TABLE (
    id UUID,
    applies_to TEXT,
    test_type TEXT,
    test_command TEXT,
    trigger_pattern TEXT,
    priority INT,
    caught_bugs INT,
    false_positives INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        r.id,
        r.applies_to,
        r.test_type,
        r.test_command,
        r.trigger_pattern,
        r.priority,
        r.caught_bugs,
        r.false_positives
    FROM tester_learned_rules r
    WHERE r.active = true
      AND (p_applies_to IS NULL OR r.applies_to = p_applies_to OR r.applies_to = '*')
    ORDER BY r.priority DESC, r.caught_bugs DESC
    LIMIT p_limit;
END;
$$;

-- ============================================================================
-- 4. RPC: create_tester_rule
-- Create a new tester rule
-- ============================================================================

CREATE OR REPLACE FUNCTION create_tester_rule(
    p_applies_to TEXT,
    p_test_type TEXT,
    p_test_command TEXT,
    p_source TEXT,
    p_trigger_pattern TEXT DEFAULT NULL,
    p_priority INT DEFAULT 5,
    p_source_task_id UUID DEFAULT NULL,
    p_source_details JSONB DEFAULT '{}'
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_rule_id UUID;
BEGIN
    INSERT INTO tester_learned_rules (
        applies_to, test_type, test_command, trigger_pattern,
        priority, source, source_task_id, source_details
    ) VALUES (
        p_applies_to, p_test_type, p_test_command, p_trigger_pattern,
        p_priority, p_source, p_source_task_id, p_source_details
    )
    RETURNING id INTO v_rule_id;
    
    RETURN v_rule_id;
END;
$$;

-- ============================================================================
-- 5. RPC: record_tester_rule_caught_bug
-- Mark that a tester rule caught a real bug
-- ============================================================================

CREATE OR REPLACE FUNCTION record_tester_rule_caught_bug(
    p_rule_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE tester_learned_rules
    SET 
        caught_bugs = caught_bugs + 1,
        last_caught_at = NOW(),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- ============================================================================
-- 6. RPC: record_tester_rule_false_positive
-- Mark that a tester rule produced a false positive
-- ============================================================================

CREATE OR REPLACE FUNCTION record_tester_rule_false_positive(
    p_rule_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE tester_learned_rules
    SET 
        false_positives = false_positives + 1,
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- ============================================================================
-- 7. RPC: get_supervisor_rules
-- Get active supervisor rules for a context
-- ============================================================================

CREATE OR REPLACE FUNCTION get_supervisor_rules(
    p_task_type TEXT DEFAULT NULL,
    p_limit INT DEFAULT 20
)
RETURNS TABLE (
    id UUID,
    trigger_pattern TEXT,
    trigger_condition JSONB,
    action TEXT,
    reason TEXT,
    times_caught_issue INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        r.id,
        r.trigger_pattern,
        r.trigger_condition,
        r.action,
        r.reason,
        r.times_caught_issue
    FROM supervisor_learned_rules r
    WHERE r.active = true
      AND (
          p_task_type IS NULL 
          OR r.trigger_condition->>'task_type' IS NULL
          OR r.trigger_condition->>'task_type' = p_task_type
      )
    ORDER BY r.times_caught_issue DESC
    LIMIT p_limit;
END;
$$;

-- ============================================================================
-- 8. RPC: create_supervisor_rule
-- Create a new supervisor rule
-- ============================================================================

CREATE OR REPLACE FUNCTION create_supervisor_rule(
    p_trigger_pattern TEXT,
    p_action TEXT,
    p_reason TEXT,
    p_source TEXT,
    p_trigger_condition JSONB DEFAULT '{}',
    p_source_task_id UUID DEFAULT NULL
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_rule_id UUID;
BEGIN
    INSERT INTO supervisor_learned_rules (
        trigger_pattern, action, reason, source,
        trigger_condition, source_task_id
    ) VALUES (
        p_trigger_pattern, p_action, p_reason, p_source,
        p_trigger_condition, p_source_task_id
    )
    RETURNING id INTO v_rule_id;
    
    RETURN v_rule_id;
END;
$$;

-- ============================================================================
-- 9. RPC: record_supervisor_rule_triggered
-- Mark that a supervisor rule was triggered
-- ============================================================================

CREATE OR REPLACE FUNCTION record_supervisor_rule_triggered(
    p_rule_id UUID,
    p_caught_issue BOOLEAN
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE supervisor_learned_rules
    SET 
        times_triggered = times_triggered + 1,
        times_caught_issue = times_caught_issue + CASE WHEN p_caught_issue THEN 1 ELSE 0 END,
        false_positive_count = false_positive_count + CASE WHEN p_caught_issue THEN 0 ELSE 1 END,
        last_triggered_at = NOW(),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- ============================================================================
-- 10. RPC: create_rule_from_supervisor_rejection
-- Auto-create supervisor rule from rejection
-- ============================================================================

CREATE OR REPLACE FUNCTION create_rule_from_supervisor_rejection(
    p_task_id UUID,
    p_issue_pattern TEXT,
    p_issue_text TEXT,
    p_task_type TEXT DEFAULT NULL
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_rule_id UUID;
    v_trigger_pattern TEXT;
    v_action TEXT;
    v_reason TEXT;
BEGIN
    -- Determine trigger pattern from issue
    v_trigger_pattern := p_issue_pattern;
    
    -- Determine action based on issue severity
    v_action := CASE 
        WHEN p_issue_text ILIKE '%secret%' OR p_issue_text ILIKE '%credential%' THEN 'reject'
        WHEN p_issue_text ILIKE '%missing%' OR p_issue_text ILIKE '%incomplete%' THEN 'reject'
        ELSE 'warn'
    END;
    
    -- Create reason
    v_reason := 'Learned from rejection: ' || COALESCE(p_issue_text, 'quality issue');
    IF length(v_reason) > 200 THEN
        v_reason := substring(v_reason, 1, 197) || '...';
    END IF;
    
    -- Build trigger condition
    INSERT INTO supervisor_learned_rules (
        trigger_pattern, action, reason, source,
        trigger_condition, source_task_id
    ) VALUES (
        v_trigger_pattern,
        v_action,
        v_reason,
        'supervisor_rejection',
        CASE WHEN p_task_type IS NOT NULL 
             THEN jsonb_build_object('task_type', p_task_type)
             ELSE '{}'::jsonb END,
        p_task_id
    )
    RETURNING id INTO v_rule_id;
    
    RETURN v_rule_id;
END;
$$;

-- ============================================================================
-- 11. RPC: deactivate_tester_rule / deactivate_supervisor_rule
-- ============================================================================

CREATE OR REPLACE FUNCTION deactivate_tester_rule(
    p_rule_id UUID,
    p_reason TEXT DEFAULT NULL
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE tester_learned_rules
    SET 
        active = false,
        source_details = jsonb_set(
            COALESCE(source_details, '{}'),
            '{deactivation_reason}',
            to_jsonb(COALESCE(p_reason, 'No reason provided'))
        ),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

CREATE OR REPLACE FUNCTION deactivate_supervisor_rule(
    p_rule_id UUID,
    p_reason TEXT DEFAULT NULL
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE supervisor_learned_rules
    SET 
        active = false,
        trigger_condition = jsonb_set(
            COALESCE(trigger_condition, '{}'),
            '{deactivation_reason}',
            to_jsonb(COALESCE(p_reason, 'No reason provided'))
        ),
        updated_at = NOW()
    WHERE id = p_rule_id;
END;
$$;

-- ============================================================================
-- 12. RPC: get_learning_stats
-- Get effectiveness stats for all learning tables
-- ============================================================================

CREATE OR REPLACE FUNCTION get_learning_stats()
RETURNS TABLE (
    table_name TEXT,
    total_rules INT,
    active_rules INT,
    total_applications INT,
    total_issues_caught INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Planner rules
    RETURN QUERY
    SELECT 
        'planner'::TEXT,
        COUNT(*)::INT,
        COUNT(*) FILTER (WHERE active)::INT,
        COALESCE(SUM(times_applied), 0)::INT,
        COALESCE(SUM(times_prevented_issue), 0)::INT
    FROM planner_learned_rules
    
    UNION ALL
    
    -- Tester rules
    SELECT 
        'tester'::TEXT,
        COUNT(*)::INT,
        COUNT(*) FILTER (WHERE active)::INT,
        COALESCE(SUM(caught_bugs + false_positives), 0)::INT,
        COALESCE(SUM(caught_bugs), 0)::INT
    FROM tester_learned_rules
    
    UNION ALL
    
    -- Supervisor rules
    SELECT 
        'supervisor'::TEXT,
        COUNT(*)::INT,
        COUNT(*) FILTER (WHERE active)::INT,
        COALESCE(SUM(times_triggered), 0)::INT,
        COALESCE(SUM(times_caught_issue), 0)::INT
    FROM supervisor_learned_rules;
END;
$$;

-- ============================================================================
-- 13. TRIGGERS: Update updated_at
-- ============================================================================

CREATE OR REPLACE FUNCTION update_tester_rules_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION update_supervisor_rules_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trigger_tester_rules_updated_at ON tester_learned_rules;
CREATE TRIGGER trigger_tester_rules_updated_at
    BEFORE UPDATE ON tester_learned_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_tester_rules_updated_at();

DROP TRIGGER IF EXISTS trigger_supervisor_rules_updated_at ON supervisor_learned_rules;
CREATE TRIGGER trigger_supervisor_rules_updated_at
    BEFORE UPDATE ON supervisor_learned_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_supervisor_rules_updated_at();

-- ============================================================================
-- 14. GRANTS
-- ============================================================================

GRANT SELECT, INSERT, UPDATE ON tester_learned_rules TO authenticated;
GRANT SELECT, INSERT, UPDATE ON supervisor_learned_rules TO authenticated;

GRANT EXECUTE ON FUNCTION get_tester_rules(TEXT, INT) TO authenticated;
GRANT EXECUTE ON FUNCTION create_tester_rule(TEXT, TEXT, TEXT, TEXT, TEXT, INT, UUID, JSONB) TO authenticated;
GRANT EXECUTE ON FUNCTION record_tester_rule_caught_bug(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION record_tester_rule_false_positive(UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION get_supervisor_rules(TEXT, INT) TO authenticated;
GRANT EXECUTE ON FUNCTION create_supervisor_rule(TEXT, TEXT, TEXT, TEXT, JSONB, UUID) TO authenticated;
GRANT EXECUTE ON FUNCTION record_supervisor_rule_triggered(UUID, BOOLEAN) TO authenticated;
GRANT EXECUTE ON FUNCTION create_rule_from_supervisor_rejection(UUID, TEXT, TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION deactivate_tester_rule(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION deactivate_supervisor_rule(UUID, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_learning_stats() TO authenticated;

-- ============================================================================
-- VERIFICATION
-- ============================================================================

DO $$
BEGIN
    RAISE NOTICE 'Migration 028 complete';
    RAISE NOTICE '  - tester_learned_rules table';
    RAISE NOTICE '  - supervisor_learned_rules table';
    RAISE NOTICE '  - 10 new RPCs';
END $$;

COMMIT;
