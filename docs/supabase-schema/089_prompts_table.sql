DROP TABLE IF EXISTS prompts CASCADE;

CREATE TABLE prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    content TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_prompts_name ON prompts(name);

GRANT SELECT ON prompts TO service_role;
GRANT SELECT ON prompts TO authenticated;
