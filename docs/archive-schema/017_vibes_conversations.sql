-- Vibes Conversations Table
-- Stores chat history between human and Vibes AI assistant

CREATE TABLE IF NOT EXISTS vibes_conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  message_type TEXT NOT NULL CHECK (message_type IN ('human', 'vibes')),
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast session queries
CREATE INDEX IF NOT EXISTS idx_vibes_conversations_session ON vibes_conversations(session_id, created_at DESC);

-- Index for user queries
CREATE INDEX IF NOT EXISTS idx_vibes_conversations_user ON vibes_conversations(user_id, created_at DESC);
