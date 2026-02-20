-- Agent Real-time Messaging
-- Enables instant push-based communication between GLM-5 and Kimi

CREATE TABLE IF NOT EXISTS agent_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  from_agent TEXT NOT NULL,
  to_agent TEXT NOT NULL,
  message_type TEXT DEFAULT 'chat' CHECK (message_type IN ('chat', 'task', 'alert', 'review')),
  content JSONB NOT NULL,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast queries
CREATE INDEX IF NOT EXISTS idx_agent_messages_to_agent ON agent_messages(to_agent, read_at, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_messages_from_agent ON agent_messages(from_agent, created_at DESC);

-- Enable realtime publication
ALTER PUBLICATION supabase_realtime ADD TABLE agent_messages;

-- RPC: Get unread messages
CREATE OR REPLACE FUNCTION get_unread_messages(p_agent TEXT)
RETURNS TABLE (
  id UUID,
  from_agent TEXT,
  message_type TEXT,
  content JSONB,
  created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT 
    am.id,
    am.from_agent,
    am.message_type,
    am.content,
    am.created_at
  FROM agent_messages am
  WHERE am.to_agent = p_agent
    AND am.read_at IS NULL
  ORDER BY am.created_at ASC;
END;
$$ LANGUAGE plpgsql;

-- RPC: Mark message as read
CREATE OR REPLACE FUNCTION mark_message_read(p_message_id UUID)
RETURNS VOID AS $$
BEGIN
  UPDATE agent_messages SET read_at = NOW() WHERE id = p_message_id;
END;
$$ LANGUAGE plpgsql;

-- RPC: Send message (convenience)
CREATE OR REPLACE FUNCTION send_agent_message(
  p_from TEXT,
  p_to TEXT,
  p_type TEXT DEFAULT 'chat',
  p_content JSONB
)
RETURNS UUID AS $$
DECLARE
  v_id UUID;
BEGIN
  INSERT INTO agent_messages (from_agent, to_agent, message_type, content)
  VALUES (p_from, p_to, p_type, p_content)
  RETURNING id INTO v_id;
  RETURN v_id;
END;
$$ LANGUAGE plpgsql;
