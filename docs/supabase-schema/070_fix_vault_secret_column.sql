-- VibePilot Migration 070: Fix get_vault_secret RPC column name
-- Purpose: Fix column reference from 'key' to 'key_name'

CREATE OR REPLACE FUNCTION get_vault_secret(
  p_key TEXT
)
RETURNS JSONB AS $$
DECLARE
  v_secret RECORD;
BEGIN
  SELECT * INTO v_secret 
  FROM secrets_vault 
  WHERE key_name = p_key;
  
  IF NOT FOUND THEN
    RETURN jsonb_build_object(
      'success', false,
      'error', 'Secret not found',
      'key', p_key
    );
  END IF;
  
  RETURN jsonb_build_object(
    'success', true,
    'key', p_key,
    'encrypted_value', v_secret.encrypted_value,
    'created_at', v_secret.created_at
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION get_vault_secret(TEXT) TO service_role;

SELECT 'Migration 070 complete: get_vault_secret fixed' AS status;
