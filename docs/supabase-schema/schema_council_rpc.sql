-- Council RPC Functions for Iterative Consensus
-- Date: 2026-02-14
-- Purpose: Multi-model Council review with feedback aggregation

-- =============================================================================
-- Council Review Storage
-- =============================================================================

CREATE TABLE IF NOT EXISTS council_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL,
    round INT NOT NULL DEFAULT 1,
    model_id TEXT NOT NULL,
    lens TEXT NOT NULL CHECK (lens IN ('user_alignment', 'ideal_vision', 'technical_security')),
    vote TEXT NOT NULL CHECK (vote IN ('APPROVED', 'REVISION_NEEDED', 'BLOCKED')),
    confidence FLOAT CHECK (confidence >= 0 AND confidence <= 1),
    approach TEXT,
    user_intent_check TEXT,
    tech_drift_check TEXT,
    dependencies_check TEXT,
    preventative_issues JSONB DEFAULT '[]'::jsonb,
    concerns JSONB DEFAULT '[]'::jsonb,
    suggestions JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(plan_id, round, model_id)
);

-- Index for quick lookups
CREATE INDEX IF NOT EXISTS idx_council_reviews_plan_round 
ON council_reviews(plan_id, round);

-- =============================================================================
-- RPC: Submit Council Review
-- =============================================================================

CREATE OR REPLACE FUNCTION submit_council_review(
    p_plan_id UUID,
    p_round INT,
    p_model_id TEXT,
    p_lens TEXT,
    p_vote TEXT,
    p_confidence FLOAT DEFAULT NULL,
    p_approach TEXT DEFAULT NULL,
    p_user_intent_check TEXT DEFAULT NULL,
    p_tech_drift_check TEXT DEFAULT NULL,
    p_dependencies_check TEXT DEFAULT NULL,
    p_preventative_issues JSONB DEFAULT '[]'::jsonb,
    p_concerns JSONB DEFAULT '[]'::jsonb,
    p_suggestions JSONB DEFAULT '[]'::jsonb
)
RETURNS UUID AS $$
DECLARE
    v_review_id UUID;
BEGIN
    INSERT INTO council_reviews (
        plan_id, round, model_id, lens, vote, confidence,
        approach, user_intent_check, tech_drift_check, dependencies_check,
        preventative_issues, concerns, suggestions
    ) VALUES (
        p_plan_id, p_round, p_model_id, p_lens, p_vote, p_confidence,
        p_approach, p_user_intent_check, p_tech_drift_check, p_dependencies_check,
        p_preventative_issues, p_concerns, p_suggestions
    )
    ON CONFLICT (plan_id, round, model_id) DO UPDATE SET
        vote = EXCLUDED.vote,
        confidence = EXCLUDED.confidence,
        approach = EXCLUDED.approach,
        user_intent_check = EXCLUDED.user_intent_check,
        tech_drift_check = EXCLUDED.tech_drift_check,
        dependencies_check = EXCLUDED.dependencies_check,
        preventative_issues = EXCLUDED.preventative_issues,
        concerns = EXCLUDED.concerns,
        suggestions = EXCLUDED.suggestions,
        created_at = NOW()
    RETURNING id INTO v_review_id;
    
    RETURN v_review_id;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- RPC: Get Council Summary for Round
-- =============================================================================

CREATE OR REPLACE FUNCTION get_council_summary(
    p_plan_id UUID,
    p_round INT
)
RETURNS JSONB AS $$
DECLARE
    v_result JSONB;
BEGIN
    SELECT jsonb_build_object(
        'plan_id', p_plan_id,
        'round', p_round,
        'reviews', jsonb_agg(
            jsonb_build_object(
                'model_id', model_id,
                'lens', lens,
                'vote', vote,
                'confidence', confidence,
                'approach', approach,
                'concerns', concerns,
                'suggestions', suggestions
            )
        ),
        'consensus', (
            CASE 
                WHEN COUNT(*) FILTER (WHERE vote = 'APPROVED') = 3 THEN 'APPROVED'
                WHEN COUNT(*) FILTER (WHERE vote = 'BLOCKED') > 0 THEN 'BLOCKED'
                ELSE 'NO_CONSENSUS'
            END
        ),
        'all_approved', COUNT(*) FILTER (WHERE vote = 'APPROVED') = 3,
        'any_blocked', COUNT(*) FILTER (WHERE vote = 'BLOCKED') > 0,
        'common_concerns', (
            SELECT jsonb_agg(DISTINCT elem)
            FROM council_reviews, jsonb_array_elements_text(concerns) elem
            WHERE plan_id = p_plan_id AND round = p_round
            GROUP BY plan_id
            HAVING COUNT(*) > 1
        ),
        'all_suggestions', (
            SELECT jsonb_agg(DISTINCT elem)
            FROM council_reviews, jsonb_array_elements_text(suggestions) elem
            WHERE plan_id = p_plan_id AND round = p_round
        )
    ) INTO v_result
    FROM council_reviews
    WHERE plan_id = p_plan_id AND round = p_round;
    
    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- RPC: Check if Next Round Needed
-- =============================================================================

CREATE OR REPLACE FUNCTION needs_next_round(
    p_plan_id UUID,
    p_current_round INT
)
RETURNS BOOLEAN AS $$
DECLARE
    v_approved_count INT;
    v_blocked_count INT;
BEGIN
    SELECT 
        COUNT(*) FILTER (WHERE vote = 'APPROVED'),
        COUNT(*) FILTER (WHERE vote = 'BLOCKED')
    INTO v_approved_count, v_blocked_count
    FROM council_reviews
    WHERE plan_id = p_plan_id AND round = p_current_round;
    
    -- Need next round if:
    -- 1. Not all 3 approved
    -- 2. No blocks (blocks require human arbitration)
    -- 3. Haven't hit max rounds (5)
    RETURN (v_approved_count < 3) 
           AND (v_blocked_count = 0) 
           AND (p_current_round < 5);
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- RPC: Get Aggregated Feedback for Next Round
-- =============================================================================

CREATE OR REPLACE FUNCTION get_round_feedback(
    p_plan_id UUID,
    p_round INT
)
RETURNS JSONB AS $$
DECLARE
    v_feedback JSONB;
BEGIN
    SELECT jsonb_build_object(
        'round', p_round,
        'approaches', jsonb_agg(
            jsonb_build_object(
                'model', model_id,
                'lens', lens,
                'approach', approach
            )
        ),
        'all_concerns', (
            SELECT jsonb_agg(jsonb_build_object(
                'model', model_id,
                'concerns', concerns
            ))
            FROM council_reviews
            WHERE plan_id = p_plan_id AND round = p_round
        ),
        'all_suggestions', (
            SELECT jsonb_agg(jsonb_build_object(
                'model', model_id,
                'suggestions', suggestions
            ))
            FROM council_reviews
            WHERE plan_id = p_plan_id AND round = p_round
        ),
        'common_issues', (
            SELECT jsonb_agg(DISTINCT elem)
            FROM council_reviews, jsonb_array_elements_text(preventative_issues) elem
            WHERE plan_id = p_plan_id AND round = p_round
        )
    ) INTO v_feedback
    FROM council_reviews
    WHERE plan_id = p_plan_id AND round = p_round
    GROUP BY plan_id;
    
    RETURN v_feedback;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Example Usage
-- =============================================================================

-- Submit a review:
-- SELECT submit_council_review(
--     'plan-uuid-here',
--     1,
--     'gpt-4',
--     'user_alignment',
--     'APPROVED',
--     0.95,
--     'Approach A is recommended...',
--     'Yes - preserves user intent',
--     'No drift detected',
--     'All clear',
--     '["issue1"]'::jsonb,
--     '["concern1"]'::jsonb,
--     '["suggestion1"]'::jsonb
-- );

-- Get summary:
-- SELECT get_council_summary('plan-uuid-here', 1);

-- Check if next round needed:
-- SELECT needs_next_round('plan-uuid-here', 1);

-- Get feedback for models in next round:
-- SELECT get_round_feedback('plan-uuid-here', 1);
