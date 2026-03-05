-- Migration: 063_enable_realtime.sql
-- Purpose: Enable Supabase Realtime on monitored tables (replaces broken pg_net webhooks)
-- Date: 2026-03-05
-- Author: GLM-5 Session 52
--
-- BACKGROUND:
-- pg_net webhooks are not firing (worker not processing requests).
-- Supabase Realtime uses WebSocket connections instead:
--   - No egress charges (inbound connection)
--   - Real-time notifications
--   - Works independently of pg_net
--   - Included in free tier
--
-- INSTRUCTIONS: Run this in Supabase SQL Editor

-- =====================================================
-- STEP 1: Add tables to supabase_realtime publication
-- =====================================================
-- This enables Postgres Changes feature for these tables

ALTER PUBLICATION supabase_realtime ADD TABLE public.plans;
ALTER PUBLICATION supabase_realtime ADD TABLE public.tasks;
ALTER PUBLICATION supabase_realtime ADD TABLE public.maintenance_commands;
ALTER PUBLICATION supabase_realtime ADD TABLE public.research_suggestions;
ALTER PUBLICATION supabase_realtime ADD TABLE public.test_results;

-- =====================================================
-- STEP 2: Verify publication contents
-- =====================================================
SELECT 
    schemaname,
    tablename 
FROM pg_publication_tables 
WHERE pubname = 'supabase_realtime'
ORDER BY tablename;

-- =====================================================
-- EXPECTED OUTPUT:
-- 
-- schemaname | tablename
-- -----------+---------------------
-- public     | maintenance_commands
-- public     | plans
-- public     | research_suggestions
-- public     | tasks
-- public     | test_results
-- =====================================================

-- =====================================================
-- STEP 3: (Optional) Set replica identity for old records
-- =====================================================
-- If you need old_record in UPDATE/DELETE events, enable this:
-- ALTER TABLE plans REPLICA IDENTITY FULL;
-- ALTER TABLE tasks REPLICA IDENTITY FULL;
-- 
-- Note: This increases WAL volume. Only enable if needed.
-- =====================================================

-- =====================================================
-- HOW IT WORKS:
-- 
-- 1. Governor connects via WebSocket to Supabase Realtime
-- 2. Subscribes to Postgres Changes on all tables
-- 3. When INSERT/UPDATE/DELETE happens, Realtime sends event
-- 4. Governor routes event through existing EventRouter
-- 
-- This replaces pg_net webhooks completely.
-- =====================================================
