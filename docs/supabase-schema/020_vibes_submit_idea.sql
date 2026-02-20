-- VibePilot: Vibes Submit Idea RPC
-- Allows Vibes panel to submit ideas for processing
-- Returns plan_path for tracking

CREATE OR REPLACE FUNCTION vibes_submit_idea(
    p_idea TEXT,
    p_user_id TEXT DEFAULT 'anonymous',
    p_project_id UUID DEFAULT NULL
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_result JSONB;
BEGIN
    -- Insert idea into ideas table for tracking
    INSERT INTO vibes_ideas (
        user_id,
        idea_text,
        project_id,
        status,
        created_at
    ) VALUES (
        p_user_id,
        p_idea,
        p_project_id,
        'pending',
        NOW()
    ) RETURNING jsonb_build_object(
        'idea_id', id,
        'status', status,
        'created_at', created_at
    ) INTO v_result;
    
    RETURN v_result;
END;
$$;

-- Create ideas tracking table if not exists
CREATE TABLE IF NOT EXISTS vibes_ideas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT DEFAULT 'anonymous',
    idea_text TEXT NOT NULL,
    project_id UUID REFERENCES projects(id),
    status TEXT DEFAULT 'pending',
    prd_path TEXT,
    plan_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

-- RLS policies
ALTER TABLE vibes_ideas ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Anyone can submit ideas" ON vibes_ideas
    FOR INSERT WITH CHECK (true);

CREATE POLICY "Anyone can view ideas" ON vibes_ideas
    FOR SELECT USING (true);

-- Index for status queries
CREATE INDEX IF NOT EXISTS idx_vibes_ideas_status ON vibes_ideas(status);
CREATE INDEX IF NOT EXISTS idx_vibes_ideas_created ON vibes_ideas(created_at DESC);

-- Grant permissions
GRANT EXECUTE ON FUNCTION vibes_submit_idea TO anon, authenticated;
