-- ============================================================================
-- Migration 051: PIF Phase G — Per-project Kanban, Code Graph, Knowledgebase
-- ============================================================================
-- Makes three VibePilot-only systems project-aware:
-- 1. project_todos: add project_id for per-project kanban
-- 2. kb_* tables: add project_id for per-project knowledgebase
-- 3. code_graph_snapshots: new table for per-project Understand Anything graphs
-- ============================================================================

-- 1. KANBAN: Add project_id to project_todos
ALTER TABLE project_todos ADD COLUMN IF NOT EXISTS project_id UUID;

-- Backfill: all existing todos belong to vibepilot
UPDATE project_todos 
SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') 
WHERE project_id IS NULL;

-- Add NOT NULL constraint after backfill
ALTER TABLE project_todos ALTER COLUMN project_id SET NOT NULL;

-- Index for project-filtered queries
CREATE INDEX IF NOT EXISTS idx_project_todos_project_id ON project_todos(project_id);

-- Add to RPC whitelist-safe tables
COMMENT ON TABLE project_todos IS 'Per-project kanban/todo items. project_id links to projects table.';

-- ============================================================================
-- 2. CODE GRAPH: New table for per-project Understand Anything snapshots
-- ============================================================================
CREATE TABLE IF NOT EXISTS code_graph_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    graph_data JSONB NOT NULL DEFAULT '{}',
    node_count INTEGER DEFAULT 0,
    edge_count INTEGER DEFAULT 0,
    source_path TEXT NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_code_graph_project UNIQUE (project_id)
);

COMMENT ON TABLE code_graph_snapshots IS 'Per-project code knowledge graph from Understand Anything. One snapshot per project, replaced on re-index.';

-- ============================================================================
-- 3. KNOWLEDGEBASE: Add project_id to KB tables
-- ============================================================================

-- KB Files
ALTER TABLE kb_files ADD COLUMN IF NOT EXISTS project_id UUID;
UPDATE kb_files SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') WHERE project_id IS NULL;
ALTER TABLE kb_files ALTER COLUMN project_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_files_project_id ON kb_files(project_id);

-- KB Knowledge Items
ALTER TABLE kb_knowledge_items ADD COLUMN IF NOT EXISTS project_id UUID;
UPDATE kb_knowledge_items SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') WHERE project_id IS NULL;
ALTER TABLE kb_knowledge_items ALTER COLUMN project_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_knowledge_items_project_id ON kb_knowledge_items(project_id);

-- KB Doc Sections
ALTER TABLE kb_doc_sections ADD COLUMN IF NOT EXISTS project_id UUID;
UPDATE kb_doc_sections SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') WHERE project_id IS NULL;
ALTER TABLE kb_doc_sections ALTER COLUMN project_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_doc_sections_project_id ON kb_doc_sections(project_id);

-- KB Canon (canonical decisions)
ALTER TABLE kb_canon ADD COLUMN IF NOT EXISTS project_id UUID;
UPDATE kb_canon SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') WHERE project_id IS NULL;
ALTER TABLE kb_canon ALTER COLUMN project_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_canon_project_id ON kb_canon(project_id);

-- KB Skills
ALTER TABLE kb_skills ADD COLUMN IF NOT EXISTS project_id UUID;
UPDATE kb_skills SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot') WHERE project_id IS NULL;
ALTER TABLE kb_skills ALTER COLUMN project_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_kb_skills_project_id ON kb_skills(project_id);

-- ============================================================================
-- Done
-- ============================================================================
DO $$ BEGIN
    RAISE NOTICE 'Migration 051 complete: project_id added to project_todos, kb_files, kb_knowledge_items, kb_doc_sections, kb_canon, kb_skills. code_graph_snapshots table created.';
END $$;
