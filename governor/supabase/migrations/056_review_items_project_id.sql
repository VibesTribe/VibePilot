-- Migration 056: Add project_id to review_items for per-project review queues
BEGIN;

ALTER TABLE review_items ADD COLUMN IF NOT EXISTS project_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
CREATE INDEX IF NOT EXISTS idx_review_items_project_id ON review_items (project_id);

COMMIT;
