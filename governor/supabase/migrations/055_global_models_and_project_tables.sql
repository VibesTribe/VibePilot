-- Migration 055: Global model sentinel + per-project model architecture
-- 
-- Models fall into two categories:
--   1. GLOBAL (project_id = sentinel UUID) — shared infrastructure visible to all projects
--      (Gemini, Groq, OpenRouter free, DeepSeek with shared key, etc.)
--   2. PROJECT-SPECIFIC (project_id = project UUID) — custom API keys, paid subs, 
--      fine-tuned models for a specific project
--
-- The batch endpoint returns: project_id IN (<sentinel>, <current_project>)
-- This gives every project global models + its own custom models.
--
-- Sentinel UUID: 00000000-0000-0000-0000-000000000000

BEGIN;

-- ============================================================
-- 1. Add global sentinel project if it doesn't exist
-- ============================================================
INSERT INTO projects (id, slug, display_name, status)
VALUES ('00000000-0000-0000-0000-000000000000', '__global__', '__Global__', 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 2. Set existing models to global (they're shared infrastructure)
-- ============================================================
UPDATE models SET project_id = '00000000-0000-0000-0000-000000000000'
WHERE project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd';

-- ============================================================
-- 3. Add project_id to remaining tables that lack it
-- ============================================================
ALTER TABLE failure_records ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
CREATE INDEX IF NOT EXISTS idx_failure_records_project_id ON failure_records (project_id);

ALTER TABLE council_reviews ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
CREATE INDEX IF NOT EXISTS idx_council_reviews_project_id ON council_reviews (project_id);

ALTER TABLE design_reviews ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
CREATE INDEX IF NOT EXISTS idx_design_reviews_project_id ON design_reviews (project_id);

ALTER TABLE maintenance_commands ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
CREATE INDEX IF NOT EXISTS idx_maintenance_commands_project_id ON maintenance_commands (project_id);

-- ============================================================
-- 4. Set global agent_sessions to use sentinel too
-- ============================================================
-- VibePilot sessions that were already tagged with VibePilot's project_id
-- should stay as they are (they're VibePilot's own sessions, not global).
-- New projects' sessions get their own project_id via the sync script.
-- This migration just ensures the sentinel exists for global model queries.

COMMIT;
