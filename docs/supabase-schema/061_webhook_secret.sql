-- Migration: 061_webhook_secret.sql
-- Purpose: Add webhook secret to vault for signature verification
-- Date: 2026-03-05
-- Author: GLM-5 Session 52

-- Webhook secret for verifying Supabase webhook signatures
-- Encrypted using governor vault (AES-GCM with VAULT_KEY)
-- Governor retrieves automatically using: vault.GetSecret(ctx, "webhook_secret")

-- First, delete any existing webhook_secret (in case of format change)
DELETE FROM secrets_vault WHERE key_name = 'webhook_secret';

-- Insert new webhook_secret (Go vault format)
-- The encrypted value below is for secret: <RUN GO ENCRYPT TOOL LOCALLY>
-- Encrypted with VAULT_KEY using Go vault.Encrypt() function
INSERT INTO secrets_vault (key_name, encrypted_value, created_at)
VALUES ('webhook_secret', '<OUTPUT FROM STEP 3>', NOW());

-- INSTRUCTIONS FOR APPLYING
-- =====================================================
-- 1. Generate a new secret:
--    openssl rand -hex 32
--
-- 2. Encrypt it using Go vault:
--    cd ~/vibepilot/governor
--    go run ./cmd/encrypt_secret/main.go "YOUR_VAULT_KEY" "YOUR_GENERATED_SECRET"
--
-- 3. Replace the encrypted_value above with the output
--
-- 4. Run this migration in Supabase SQL Editor
--
-- 5. Governor will retrieve this automatically using:
--    vault.GetSecret(ctx, "webhook_secret")
-- =====================================================
