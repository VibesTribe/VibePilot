-- Migration 054: Add project_id to models, orchestrator_events, chat_usage
-- Enables per-project data isolation in the dashboard batch endpoint.
-- Each table gets project_id with a default pointing to VibePilot's canonical ID.
-- Existing rows are backfilled to VibePilot.
--
-- After this migration, update server.go to filter these tables by project_id.

BEGIN;

-- ============================================================
-- models: add project_id, default to VibePilot
-- ============================================================
ALTER TABLE models ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_models_project_id ON models (project_id);

-- ============================================================
-- orchestrator_events: add project_id
-- Events are created in context of tasks (which have project_id).
-- Default to Vibepilot for backward compatibility.
-- ============================================================
ALTER TABLE orchestrator_events ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_orchestrator_events_project_id ON orchestrator_events (project_id);

-- Backfill: set project_id from the associated task if available
UPDATE orchestrator_events oe
SET project_id = t.project_id
FROM tasks t
WHERE oe.task_id = t.id::text
  AND oe.project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd'
  AND t.project_id != '947c2db2-ac1f-4307-9048-8d838ef3aacd';

-- ============================================================
-- chat_usage: add project_id
-- Chat usage is tied to dashboard chat sessions.
-- ============================================================
ALTER TABLE chat_usage ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_chat_usage_project_id ON chat_usage (project_id);

-- ============================================================
-- visual_qa_runs: add project_id
-- ============================================================
ALTER TABLE visual_qa_runs ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_visual_qa_runs_project_id ON visual_qa_runs (project_id);

-- ============================================================
-- test_results: add project_id
-- ============================================================
ALTER TABLE test_results ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_test_results_project_id ON test_results (project_id);

-- ============================================================
-- model_health_snapshots: add project_id
-- ============================================================
ALTER TABLE model_health_snapshots ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_model_health_snapshots_project_id ON model_health_snapshots (project_id);

COMMIT;
