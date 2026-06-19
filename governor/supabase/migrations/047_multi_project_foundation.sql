-- 047: Multi-Project Foundation
-- Enhances the existing projects table and adds project_id to tasks.
-- Purely additive: no existing columns or data are modified destructively.
-- All existing tasks get backfilled to a default "vibepilot" project.

BEGIN;

-- 1. ADD PRIMARY KEY to existing projects table (it exists from old schema but has no constraints)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'projects'::regclass AND contype = 'p'
    ) THEN
        ALTER TABLE projects ADD CONSTRAINT projects_pkey PRIMARY KEY (id);
    END IF;
END $$;

-- 2. ADD missing columns to projects table
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slug TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS display_name TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS github_owner TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS github_repo TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS repo_path TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS default_branch TEXT DEFAULT 'main';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS branch_prefix_task TEXT DEFAULT 'task/';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS branch_prefix_module TEXT DEFAULT 'module/';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS protected_branches TEXT[] DEFAULT ARRAY['main']::TEXT[];

ALTER TABLE projects ADD COLUMN IF NOT EXISTS tech_stack TEXT DEFAULT 'auto';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS build_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS test_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS lint_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS typecheck_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deploy_command TEXT;

ALTER TABLE projects ADD COLUMN IF NOT EXISTS deploy_target TEXT DEFAULT 'none';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS deploy_url TEXT;

ALTER TABLE projects ADD COLUMN IF NOT EXISTS model_keys JSONB DEFAULT '[]'::JSONB;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS connected_services JSONB DEFAULT '[]'::JSONB;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS theme JSONB DEFAULT '{}'::JSONB;

-- 3. ADD UNIQUE constraint on slug (needed for ON CONFLICT in seed)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'projects_slug_key'
    ) THEN
        ALTER TABLE projects ADD CONSTRAINT projects_slug_key UNIQUE (slug);
    END IF;
END $$;

-- 4. ADD project_id TO tasks (nullable for backward compatibility)
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id);

-- 5. CREATE INDEX on tasks.project_id
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);

-- 6. SEED the default "vibepilot" project
INSERT INTO projects (
    slug,
    display_name,
    description,
    status,
    github_owner,
    github_repo,
    repo_path,
    default_branch,
    deploy_target,
    model_keys,
    connected_services,
    theme
) VALUES (
    'vibepilot',
    'VibePilot',
    'VibePilot self-maintenance and infrastructure',
    'active',
    'VibesTribe',
    'VibePilot',
    '/home/vibes/vibepilot',
    'main',
    'vercel',
    '["GEMINI_API_KEY_1", "GEMINI_API_KEY_2", "GEMINI_API_KEY_3", "GEMINI_API_KEY_4", "GLM_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY"]'::JSONB,
    '[
        {"type": "github", "label": "VibePilot Repo", "url": "https://github.com/VibesTribe/VibePilot"},
        {"type": "github", "label": "VibeFlow Dashboard", "url": "https://github.com/VibesTribe/vibeflow"},
        {"type": "vercel", "label": "Vercel Dashboard", "url": "https://vercel.com/vibestribe"},
        {"type": "cloudflare", "label": "Cloudflare", "url": "https://dash.cloudflare.com"},
        {"type": "openrouter", "label": "OpenRouter", "url": "https://openrouter.ai/credits"},
        {"type": "google", "label": "Google AI Studio", "url": "https://aistudio.google.com"},
        {"type": "groq", "label": "Groq Console", "url": "https://console.groq.com"}
    ]'::JSONB,
    '{"primary_color": "#6366f1"}'::JSONB
)
ON CONFLICT (slug) DO NOTHING;

-- 7. BACKFILL all existing tasks with the vibepilot project_id
UPDATE tasks
SET project_id = (
    SELECT id FROM projects WHERE slug = 'vibepilot'
)
WHERE project_id IS NULL;

-- 8. UPDATE vibepilot project task count
UPDATE projects
SET total_tasks = (
    SELECT count(*) FROM tasks WHERE tasks.project_id = projects.id
)
WHERE slug = 'vibepilot';

COMMIT;
