-- Vibes Preferences Table
-- Stores user preferences for Vibes AI assistant

CREATE TABLE IF NOT EXISTS vibes_preferences (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL UNIQUE,
  preferences JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast user lookups
CREATE INDEX IF NOT EXISTS idx_vibes_preferences_user ON vibes_preferences(user_id);

-- Enable RLS
ALTER TABLE vibes_preferences ENABLE ROW LEVEL SECURITY;

-- Policy: Users can only see their own preferences
CREATE POLICY vibes_preferences_select_policy ON vibes_preferences
  FOR SELECT USING (auth.uid()::text = user_id);

-- Policy: Users can only update their own preferences  
CREATE POLICY vibes_preferences_update_policy ON vibes_preferences
  FOR UPDATE USING (auth.uid()::text = user_id);

-- Grant access
GRANT SELECT, UPDATE ON vibes_preferences TO authenticated;
GRANT ALL ON vibes_preferences TO anon;
