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
-- The encrypted value below is for secret: 251e1a419229c8a59139012449258112209ab055d614e091b5a645c9f976d0f3
-- Encrypted with VAULT_KEY using Go vault.Encrypt() function
INSERT INTO secrets_vault (key_name, encrypted_value, created_at)
VALUES ('webhook_secret', 'w23J7KPOy1ntFyegNsnYNVBYJ/ceLLFtEs2tWk2ulgUHgSnq7jwUKrl5k8eRg2mvcNGm1fqlwyleiZYW43lkpd6vTBMM6vKgmSJ7k+30Ldn9qp5oFnVRhuHZkHCm+Z5bn4vhLIIWDKfVAyO6', NOW());

-- INSTRUCTIONS FOR APPLYING
-- =====================================================
-- 1. Run this migration in Supabase SQL Editor
--
-- 2. Governor will retrieve this automatically using:
--    vault.GetSecret(ctx, "webhook_secret")
--
-- 3. The plaintext secret is: 251e1a419229c8a59139012449258112209ab055d614e091b5a645c9f976d0f3
--    (Use this in Supabase Dashboard → Database → Webhooks → Your Webhook → Secret)
-- =====================================================
