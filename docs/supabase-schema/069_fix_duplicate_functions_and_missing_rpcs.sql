-- VibePilot Migration 069: Fix duplicate functions and add missing RPCs
-- Purpose: 
--   1. Drop old create_task_with_packet function (conflicts with 068)
--   2. Add check_platform_availability RPC (router needs this)
--   3. Add get_vault_secret RPC (for accessing vault keys like GEMINI_API_KEY)

-- ============================================================================
-- PART 1: Drop the OLD create_task_with_packet function
-- ============================================================================
-- The old signature had p_prompt as 5th parameter, 068 moved it later
-- This causes "Could not choose best candidate" error

DROP FUNCTION IF EXISTS public.create_task_with_packet(
  p_plan_id uuid,
  p_task_number text,
  p_title text,
  p_type text,
  p_prompt text,
  p_status text,
  p_priority integer,
  p_confidence double precision,
  p_category text,
  p_routing_flag text,
  p_routing_flag_reason text,
  p_dependencies jsonb,
  p_expected_output text,
  p_context jsonb
);

-- ============================================================================
-- PART 2: Add check_platform_availability RPC
-- ============================================================================
-- Used by router.go to check if a web platform is available for courier tasks

CREATE OR REPLACE FUNCTION check_platform_availability(
  p_platform_id TEXT
)
RETURNS JSONB AS $$
DECLARE
  v_platform RECORD;
  v_available BOOLEAN := true;
  v_reason TEXT := '';
BEGIN
  -- Check if platform exists and is active
  SELECT * INTO v_platform 
  FROM platforms 
  WHERE id = p_platform_id;
  
  IF NOT FOUND THEN
    RETURN jsonb_build_object(
      'available', false,
      'reason', 'Platform not found'
    );
  END IF;
  
  IF v_platform.status != 'active' THEN
    v_available := false;
    v_reason := 'Platform status is ' || v_platform.status;
  END IF;
  
  RETURN jsonb_build_object(
    'available', v_available,
    'reason', v_reason,
    'platform_id', p_platform_id
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ============================================================================
-- PART 3: Add get_vault_secret RPC
-- ============================================================================
-- Used to retrieve secrets from the secrets_vault table
-- Requires VAULT_KEY environment variable (passed via service role)

CREATE OR REPLACE FUNCTION get_vault_secret(
  p_key TEXT
)
RETURNS JSONB AS $$
DECLARE
  v_secret RECORD;
  v_decrypted_value TEXT;
BEGIN
  -- Retrieve encrypted secret
  SELECT * INTO v_secret 
  FROM secrets_vault 
  WHERE key = p_key;
  
  IF NOT FOUND THEN
    RETURN jsonb_build_object(
      'success', false,
      'error', 'Secret not found',
      'key', p_key
    );
  END IF;
  
  -- Return the secret (decryption happens in Go using VAULT_KEY)
  -- The encrypted_value is what Go will decrypt
  RETURN jsonb_build_object(
    'success', true,
    'key', p_key,
    'encrypted_value', v_secret.encrypted_value,
    'created_at', v_secret.created_at
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Grant execute permissions
GRANT EXECUTE ON FUNCTION check_platform_availability(TEXT) TO service_role;
GRANT EXECUTE ON FUNCTION get_vault_secret(TEXT) TO service_role;

SELECT 'Migration 069 complete: duplicate function dropped, missing RPCs added' AS status;
