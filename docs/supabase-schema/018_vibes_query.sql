-- Vibes Query RPC
-- Main function for Vibes AI assistant to query system state

CREATE OR REPLACE FUNCTION vibes_query(
  p_user_id TEXT,
  p_question TEXT,
  p_context JSONB DEFAULT '{}'
)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  -- Build response with current system state
  v_result := jsonb_build_object(
    'response', 'Vibes here! I received your question: ' || p_question,
    'status', 'ok',
    'timestamp', NOW(),
    'context', p_context
  );
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- Grant execute to anon and authenticated users
GRANT EXECUTE ON FUNCTION vibes_query(TEXT, TEXT, JSONB) TO anon;
GRANT EXECUTE ON FUNCTION vibes_query(TEXT, TEXT, JSONB) TO authenticated;
