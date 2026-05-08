-- VIBEPILOT SCHEMA MIGRATION v1.5
-- Purpose: Agent conversation usage tracking (Hermes, dashboard vibes, etc.)
-- Date: 2026-05-07
--
-- Run this AFTER schema_v1.4_roi_enhanced.sql
--
-- Changes:
--   - `agent_sessions` table - one row per conversation session
--   - `agent_session_messages` table - one row per message exchange
--   - Tracks tokens_in, tokens_out, cost per message per conversation
--   - Links to models table via model_id
--
-- This fills the gap where agent conversation costs (terminal chats,
-- dashboard vibes conversations, etc.) were invisible to the cost system.

-- ============================================
-- AGENT SESSIONS (one per conversation)
-- ============================================

CREATE TABLE IF NOT EXISTS agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id TEXT NOT NULL,                          -- External session/conversation ID
    platform TEXT NOT NULL DEFAULT 'terminal',          -- terminal, dashboard, telegram, discord
    model_id TEXT REFERENCES models(id),               -- Which model powered the session
    platform_model TEXT,                               -- Full model name from provider (e.g. 'deepseek-v4-flash')
    provider TEXT,                                     -- Provider name (e.g. 'deepseek', 'google')
    total_tokens_in INTEGER NOT NULL DEFAULT 0,
    total_tokens_out INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_usd NUMERIC(12,8) NOT NULL DEFAULT 0,
    message_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'abandoned')),
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Index for fast session lookups
CREATE INDEX IF NOT EXISTS idx_agent_sessions_session ON agent_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_platform ON agent_sessions(platform);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_started ON agent_sessions(started_at);

-- Notify dashboard of session changes
DROP TRIGGER IF EXISTS notify_agent_sessions ON agent_sessions;
CREATE TRIGGER notify_agent_sessions
    AFTER INSERT OR UPDATE OR DELETE ON agent_sessions
    FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

-- ============================================
-- AGENT SESSION MESSAGES (per message exchange)
-- ============================================

CREATE TABLE IF NOT EXISTS agent_session_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn_number INTEGER NOT NULL,                      -- Sequential turn in the conversation
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'tool', 'system')),
    tokens_in INTEGER NOT NULL DEFAULT 0,               -- Prompt tokens for this turn
    tokens_out INTEGER NOT NULL DEFAULT 0,              -- Completion tokens for this turn
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd NUMERIC(10,8) NOT NULL DEFAULT 0,
    model_id TEXT,                                      -- Model used for this specific turn
    tool_count INTEGER NOT NULL DEFAULT 0,              -- How many tool calls in this turn
    response_time_ms INTEGER,                           -- How long the model took
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for querying messages by session
CREATE INDEX IF NOT EXISTS idx_agent_msgs_session ON agent_session_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_msgs_turn ON agent_session_messages(session_id, turn_number);

-- Notify dashboard of new messages
DROP TRIGGER IF EXISTS notify_agent_session_messages ON agent_session_messages;
CREATE TRIGGER notify_agent_session_messages
    AFTER INSERT ON agent_session_messages
    FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

-- ============================================
-- HELPER VIEW: Recent agent costs alongside pipeline costs
-- ============================================

CREATE OR REPLACE VIEW agent_usage_summary AS
SELECT
    date_trunc('day', started_at) AS day,
    platform,
    model_id,
    COUNT(DISTINCT id) AS sessions,
    SUM(message_count) AS total_messages,
    SUM(total_tokens) AS total_tokens,
    SUM(total_tokens_in) AS total_tokens_in,
    SUM(total_tokens_out) AS total_tokens_out,
    SUM(total_cost_usd)::NUMERIC(10,4) AS total_cost_usd
FROM agent_sessions
WHERE status = 'completed'
GROUP BY date_trunc('day', started_at), platform, model_id
ORDER BY day DESC;
