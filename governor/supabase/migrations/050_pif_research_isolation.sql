-- ============================================================================
-- Migration 050: PIF Phase D — Research isolation
-- ============================================================================
-- Adds project_id to research tables so each project has its own research pipeline.
-- Research suggestions, reports, bookmarks, and queue items are scoped per-project.
-- Backwards compatible: defaults to vibepilot.
-- ============================================================================

BEGIN;

-- Vibepilot UUID for defaults
-- '947c2db2-ac1f-4307-9048-8d838ef3aacd'

-- ========================================
-- research_suggestions: add project_id
-- ========================================
ALTER TABLE research_suggestions ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE research_suggestions SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE research_suggestions ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE research_suggestions ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_research_suggestions_project_id ON research_suggestions(project_id);

-- ========================================
-- research_reports: add project_id
-- ========================================
ALTER TABLE research_reports ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE research_reports SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE research_reports ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE research_reports ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_research_reports_project_id ON research_reports(project_id);

-- ========================================
-- research_bookmarks: add project_id
-- ========================================
ALTER TABLE research_bookmarks ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE research_bookmarks SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE research_bookmarks ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_research_bookmarks_project_id ON research_bookmarks(project_id);

-- ========================================
-- research_queue: add project_id
-- ========================================
ALTER TABLE research_queue ADD COLUMN IF NOT EXISTS project_id uuid;
UPDATE research_queue SET project_id = '947c2db2-ac1f-4307-9048-8d838ef3aacd' WHERE project_id IS NULL;
ALTER TABLE research_queue ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';
CREATE INDEX IF NOT EXISTS idx_research_queue_project_id ON research_queue(project_id);

COMMIT;
