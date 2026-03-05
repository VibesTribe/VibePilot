-- Migration: 062_webhook_diagnostic.sql
-- Purpose: Diagnose why Supabase webhooks aren't firing
-- Date: 2026-03-05
-- Author: GLM-5 Session 52
-- 
-- INSTRUCTIONS: Run this in Supabase SQL Editor and share results

-- =====================================================
-- STEP 1: Check if requests are queuing in pg_net
-- =====================================================
SELECT 
    id,
    method,
    url,
    created_at,
    -- Don't show full body (may contain sensitive data)
    CASE 
        WHEN body IS NOT NULL THEN 'HAS_BODY'
        ELSE 'NO_BODY'
    END as body_status
FROM net.http_request_queue 
ORDER BY created_at DESC 
LIMIT 10;

-- =====================================================
-- STEP 2: Check trigger definitions (what function they call)
-- =====================================================
SELECT 
    trigger_name,
    event_object_table,
    action_statement
FROM information_schema.triggers 
WHERE trigger_name LIKE 'governor-%'
ORDER BY trigger_name;

-- =====================================================
-- STEP 3: Check if pg_net worker is processing
-- =====================================================
-- This shows recent responses (if any)
SELECT 
    id,
    status_code,
    content_type,
    created_at
FROM net._http_response 
ORDER BY created_at DESC 
LIMIT 5;

-- =====================================================
-- STEP 4: Test pg_net directly
-- =====================================================
-- This should create a request in the queue
-- If it stays in queue, pg_net worker isn't running
SELECT net.http_request(
    'http://34.45.124.117:8080/webhooks',
    'POST',
    '{"Content-Type": "application/json", "Authorization": "251e1a419229c8a59139012449258112209ab055d614e091b5a645c9f976d0f3"}',
    '{"type": "INSERT", "table": "test_pg_net", "schema": "public", "record": {"test": true}}'
);

-- =====================================================
-- EXPECTED RESULTS:
-- 
-- If pg_net is working:
-- - Step 1 should show requests (may be empty if processed)
-- - Step 2 should show triggers calling supabase_functions.http_request
-- - Step 3 should show some responses (200 status)
-- - Step 4 should return a request ID
--
-- If pg_net is NOT working:
-- - Step 1 will show requests stuck in queue
-- - Step 3 will be empty or show errors
-- - Step 4 will queue but not process
-- =====================================================
