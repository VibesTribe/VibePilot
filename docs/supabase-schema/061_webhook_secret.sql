-- Migration: 061_webhook_secret.sql
-- Purpose: Add webhook secret to vault for signature verification
-- Date: 2026-03-04
-- Author: GLM-5 Session 48

-- Webhook secret for verifying Supabase webhook signatures
-- Encrypted using governor vault (AES-GCM with VAULT_KEY)
-- Governor retrieves automatically using: vault.GetSecret(ctx, "webhook_secret")
INSERT INTO secrets_vault (key_name, encrypted_value, created_at)
VALUES ('webhook_secret', 'r7N7RGj4nTp2ND5KiL7/JtwOqLQivMeQDOFhAlCejpY2lg+kpd8mfMhv7CrPn/6VxIePy', NOW())
ON CONFLICT (key_name) DO UPDATE
SET encrypted_value = 'r7N7RGj4nTp2ND5KiL7/JtwOqLQivMeQDOFhAlCejpY2lg+kpd8mfMhv7CrPn/6VxIePy',
    updated_at = NOW();
-- INSTRUCTIONS FOR APPLYING
-- =====================================================
-- 1. Generate a new secret:
--    openssl rand -hex 32
--
-- 2. Encrypt it using vault_manager.py:
--    cd ~/vibepilot/scripts
--    python3 vault_manager.py encrypt YOUR_GENERATED_SECRET
--
-- 3. Replace REPLACE_WITH_ENCRYPTED_SECRET above with the encrypted value
--
-- 4. Run this migration in Supabase SQL Editor
--
-- 5. Governor will retrieve this automatically using:
--    vault.GetSecret(ctx, "webhook_secret")
-- =====================================================
