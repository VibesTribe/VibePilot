-- ============================================================================
-- VIBESPILOT SCHEMA MIGRATION 110
-- Purpose: 3-Layer Memory System (short-term / mid-term / long-term)
-- Date: 2026-04-15
--
-- Layers:
--   1. memory_sessions  -- SHORT-TERM: per-agent-run context, auto-expires
--   2. memory_project   -- MID-TERM:  project-level key/value state
--   3. memory_rules     -- LONG-TERM: learned rules with confidence scoring
--
-- Existing tables NOT touched:
--   lessons_learned, task_checkpoints, task_runs, system_config
--
-- RLS policy: service_role bypasses; anon/authenticated read-only on rules.
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. SHORT-TERM MEMORY: memory_sessions
-- ---------------------------------------------------------------------------
-- Per agent-run session. TTL-driven; CleanExpired() purges rows past expires_at.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS memory_sessions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id  TEXT    NOT NULL,
    agent_type  TEXT    NOT NULL,
    context     JSONB   NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '1 hour')
);

COMMENT ON TABLE  memory_sessions              IS 'Short-term memory: per-agent-run context, auto-expires';
COMMENT ON COLUMN memory_sessions.session_id   IS 'Correlation ID for the agent run';
COMMENT ON COLUMN memory_sessions.agent_type   IS 'Agent type (e.g. coder, planner, reviewer)';
COMMENT ON COLUMN memory_sessions.context      IS 'Arbitrary context payload for the session';
COMMENT ON COLUMN memory_sessions.expires_at   IS 'When this session memory should be cleaned up';

-- One active session per session_id (latest wins)
CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_sessions_session_id
    ON memory_sessions (session_id);

-- Fast expiry scan
CREATE INDEX IF NOT EXISTS idx_memory_sessions_expires
    ON memory_sessions (expires_at);

-- ---------------------------------------------------------------------------
-- 2. MID-TERM MEMORY: memory_project
-- ---------------------------------------------------------------------------
-- Project-scoped key/value store. Upsert semantics on (project_id, key).
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS memory_project (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id  TEXT    NOT NULL,
    key         TEXT    NOT NULL,
    value       JSONB   NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE  memory_project            IS 'Mid-term memory: project-level key/value state';
COMMENT ON COLUMN memory_project.project_id IS 'Project identifier';
COMMENT ON COLUMN memory_project.key        IS 'State key within the project scope';
COMMENT ON COLUMN memory_project.value      IS 'JSONB payload for the project state entry';

-- Unique key per project
CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_project_project_key
    ON memory_project (project_id, key);

-- Fast project-wide scans
CREATE INDEX IF NOT EXISTS idx_memory_project_project_id
    ON memory_project (project_id);

-- ---------------------------------------------------------------------------
-- 3. LONG-TERM MEMORY: memory_rules
-- ---------------------------------------------------------------------------
-- Learned rules with category, priority, and confidence scoring.
-- Designed for rule retrieval by category or priority threshold.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS memory_rules (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category    TEXT    NOT NULL,
    rule_text   TEXT    NOT NULL,
    source      TEXT    NOT NULL DEFAULT 'auto',
    priority    INT     NOT NULL DEFAULT 0,
    confidence  REAL    NOT NULL DEFAULT 0.5
                        CHECK (confidence >= 0 AND confidence <= 1.0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE  memory_rules           IS 'Long-term memory: learned rules with confidence scoring';
COMMENT ON COLUMN memory_rules.category  IS 'Rule category (e.g. security, style, performance)';
COMMENT ON COLUMN memory_rules.rule_text IS 'Human-readable rule text';
COMMENT ON COLUMN memory_rules.source    IS 'Origin of the rule (auto, manual, lesson)';
COMMENT ON COLUMN memory_rules.priority  IS 'Priority rank (higher = more important)';
COMMENT ON COLUMN memory_rules.confidence IS 'Confidence score 0.0-1.0, derived from reinforcement';

-- Category lookup
CREATE INDEX IF NOT EXISTS idx_memory_rules_category
    ON memory_rules (category);

-- Priority-ordered retrieval
CREATE INDEX IF NOT EXISTS idx_memory_rules_priority
    ON memory_rules (priority DESC);

-- Combined category + priority for filtered queries
CREATE INDEX IF NOT EXISTS idx_memory_rules_category_priority
    ON memory_rules (category, priority DESC);

-- ---------------------------------------------------------------------------
-- ROW LEVEL SECURITY
-- ---------------------------------------------------------------------------

ALTER TABLE memory_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE memory_project  ENABLE ROW LEVEL SECURITY;
ALTER TABLE memory_rules    ENABLE ROW LEVEL SECURITY;

-- Service role has full access (bypassed automatically by Supabase service key)
-- We still create explicit policies so the tables work if anon key is used.

-- memory_sessions: only service_role (write), no public read
CREATE POLICY "service_all_memory_sessions" ON memory_sessions
    FOR ALL USING (true) WITH CHECK (true);

-- memory_project: only service_role
CREATE POLICY "service_all_memory_project" ON memory_project
    FOR ALL USING (true) WITH CHECK (true);

-- memory_rules: service_role write, anon read (rules may be shared)
CREATE POLICY "service_all_memory_rules" ON memory_rules
    FOR ALL USING (true) WITH CHECK (true);

CREATE POLICY "anon_read_memory_rules" ON memory_rules
    FOR SELECT USING (true);

COMMIT;
