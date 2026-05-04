-- Migration 136: Chat usage tracking for dashboard chat costs in ROI calculator
-- Tracks input/output tokens and costs from dashboard chat sessions

CREATE TABLE IF NOT EXISTS chat_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id TEXT NOT NULL,           -- Hermes session ID
    model_id TEXT NOT NULL,             -- e.g. 'gemini-2.5-flash'
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    theoretical_cost_usd DOUBLE PRECISION DEFAULT 0,
    token_source TEXT DEFAULT 'exact',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_chat_usage_session ON chat_usage(session_id);
CREATE INDEX idx_chat_usage_model ON chat_usage(model_id);
CREATE INDEX idx_chat_usage_created ON chat_usage(created_at);

-- RPC to record a chat usage event
CREATE OR REPLACE FUNCTION record_chat_usage(
    p_session_id TEXT,
    p_model_id TEXT,
    p_tokens_in INTEGER DEFAULT 0,
    p_tokens_out INTEGER DEFAULT 0,
    p_token_source TEXT DEFAULT 'exact'
)
RETURNS UUID
LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
    v_cost_result JSONB;
BEGIN
    -- Calculate cost
    SELECT * INTO v_cost_result FROM calc_run_costs(
        p_model_id, p_tokens_in, p_tokens_out, 0
    );

    INSERT INTO chat_usage (
        session_id, model_id, tokens_in, tokens_out,
        theoretical_cost_usd, token_source
    ) VALUES (
        p_session_id, p_model_id, p_tokens_in, p_tokens_out,
        COALESCE((v_cost_result->>'theoretical_cost_usd')::DOUBLE PRECISION, 0),
        p_token_source
    ) RETURNING id INTO v_id;

    -- Also increment lifetime counters so ROI dashboard stays accurate
    PERFORM increment_lifetime_counters(p_tokens_in + p_tokens_out, 0);

    RETURN v_id;
END;
$function$;

-- RPC to get aggregated chat costs for ROI display
CREATE OR REPLACE FUNCTION get_chat_cost_summary()
RETURNS TABLE(
    model_id TEXT,
    total_chats BIGINT,
    total_tokens_in BIGINT,
    total_tokens_out BIGINT,
    total_cost_usd DOUBLE PRECISION,
    first_chat TIMESTAMPTZ,
    last_chat TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        cu.model_id,
        count(*) as total_chats,
        sum(cu.tokens_in)::bigint as total_tokens_in,
        sum(cu.tokens_out)::bigint as total_tokens_out,
        round(sum(cu.theoretical_cost_usd), 6) as total_cost_usd,
        min(cu.created_at) as first_chat,
        max(cu.created_at) as last_chat
    FROM chat_usage cu
    GROUP BY cu.model_id
    ORDER BY sum(cu.theoretical_cost_usd) DESC;
END;
$function$;
