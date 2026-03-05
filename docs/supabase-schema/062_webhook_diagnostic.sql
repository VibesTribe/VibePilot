-- Migration: 062_webhook_diagnostic.sql
-- Purpose: Diagnose why Supabase webhooks aren't firing
-- Date: 2026-03-05
-- Author: GLM-5 Session 52
-- 
-- INSTRUCTIONS: Run each section separately in Supabase SQL Editor

-- =====================================================
-- STEP 1: Check pg_net queue structure
-- =====================================================
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_schema = 'net' AND table_name = 'http_request_queue';

-- =====================================================
-- STEP 2: Check if requests are queuing in pg_net
-- =====================================================
SELECT * FROM net.http_request_queue LIMIT 10;

-- =====================================================
-- STEP 3: Check trigger definitions
-- =====================================================
SELECT 
    trigger_name,
    event_object_table,
    action_statement
FROM information_schema.triggers 
WHERE trigger_name LIKE 'governor-%'
ORDER BY trigger_name;

-- =====================================================
-- STEP 4: Check pg_net responses
-- =====================================================
SELECT * FROM net._http_response LIMIT 5;

-- =====================================================
-- STEP 5: Test pg_net directly (creates a real request)
-- =====================================================
SELECT net.http_request(
    'http://34.45.124.117:8080/webhooks',
    'POST',
    '{"Content-Type": "application/json", "Authorization": "251e1a419229c8a59139012449258112209ab055d614e091b5a645c9f976d0f3"}',
    '{"type": "INSERT", "table": "test_pg_net", "schema": "public", "record": {"test": true}}'
);
