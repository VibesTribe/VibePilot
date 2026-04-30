-- Migration 115: 4 RPCs that failed silently in migration 114

-- Ensure tables exist first
CREATE TABLE IF NOT EXISTS state_transitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  from_state TEXT,
  to_state TEXT,
  transition_reason TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 1. get_latest_state
DROP FUNCTION IF EXISTS get_latest_state(TEXT, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION get_latest_state(
  p_entity_type TEXT,
  p_entity_id TEXT
)
RETURNS TABLE(to_state TEXT, transition_reason TEXT, created_at TIMESTAMPTZ) AS $$
BEGIN
  RETURN QUERY
  SELECT st.to_state, st.transition_reason, st.created_at
  FROM state_transitions st
  WHERE st.entity_type = p_entity_type AND st.entity_id = p_entity_id
  ORDER BY st.created_at DESC
  LIMIT 1;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. record_state_transition
DROP FUNCTION IF EXISTS record_state_transition(TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION record_state_transition(
  p_entity_type TEXT,
  p_entity_id TEXT,
  p_from_state TEXT DEFAULT NULL,
  p_to_state TEXT DEFAULT NULL,
  p_reason TEXT DEFAULT NULL,
  p_metadata JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO state_transitions (entity_type, entity_id, from_state, to_state, transition_reason, metadata)
  VALUES (p_entity_type, p_entity_id, p_from_state, p_to_state, p_reason, p_metadata);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. log_security_audit
DROP FUNCTION IF EXISTS log_security_audit(TEXT, TEXT, BOOLEAN, TEXT) CASCADE;
CREATE OR REPLACE FUNCTION log_security_audit(
  p_operation TEXT,
  p_key_name TEXT DEFAULT NULL,
  p_allowed BOOLEAN DEFAULT NULL,
  p_reason TEXT DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  INSERT INTO orchestrator_events (event_type, payload)
  VALUES ('security_audit', jsonb_build_object(
    'operation', p_operation,
    'key_name', p_key_name,
    'allowed', p_allowed,
    'reason', p_reason,
    'timestamp', NOW()
  ));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. update_research_suggestion_status
DROP FUNCTION IF EXISTS update_research_suggestion_status(UUID, TEXT, JSONB) CASCADE;
CREATE OR REPLACE FUNCTION update_research_suggestion_status(
  p_id UUID,
  p_status TEXT,
  p_review_notes JSONB DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
  UPDATE research_suggestions
  SET status = p_status,
      review_notes = COALESCE(p_review_notes, review_notes),
      updated_at = NOW()
  WHERE id = p_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;

SELECT 'Migration 115 complete - 4 remaining RPCs created' AS status;
