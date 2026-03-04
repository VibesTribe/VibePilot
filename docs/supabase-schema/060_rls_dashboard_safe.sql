-- Migration: 060_rls_dashboard_safe.sql
-- Purpose: Enable RLS on all tables with safe dashboard access
-- Date: 2026-03-04
-- Author: GLM-5 Session 48
-- 
-- Fixes Supabase security warnings:
-- - RLS disabled on public tables
-- - SECURITY DEFINER views
--
-- Protects dashboard by granting anon SELECT on tables it needs
-- Keeps vault and internal tables locked down

-- =====================================================
-- 1. ENABLE ROW LEVEL SECURITY ON ALL TABLES
-- =====================================================

ALTER TABLE public.platforms ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.models_new ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.task_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.tools ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.task_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.task_packets ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.models ENABLE ROW LEVEL SECURITY;

-- =====================================================
-- 2. SERVICE ROLE BYPASS (Governor uses this)
-- =====================================================
-- Governor uses SERVICE_KEY which bypasses RLS
-- These policies are explicit documentation of that access

CREATE POLICY "service_role_full_access" ON public.platforms
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.projects
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.models_new
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.task_history
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.tools
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.tasks
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.task_runs
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.task_packets
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

CREATE POLICY "service_role_full_access" ON public.models
    FOR ALL TO service_role
    USING (true)
    WITH CHECK (true);

-- =====================================================
-- 3. DASHBOARD READ ACCESS (anon key)
-- =====================================================
-- Dashboard uses anon key (client-side, must be safe)
-- Read-only access to tables it needs

-- tasks: Dashboard displays task status
CREATE POLICY "anon_read_tasks" ON public.tasks
    FOR SELECT TO anon
    USING (true);

-- task_runs: Dashboard shows execution history
CREATE POLICY "anon_read_task_runs" ON public.task_runs
    FOR SELECT TO anon
    USING (true);

-- models: Dashboard shows model status
CREATE POLICY "anon_read_models" ON public.models
    FOR SELECT TO anon
    USING (true);

-- platforms: Dashboard shows platform status
CREATE POLICY "anon_read_platforms" ON public.platforms
    FOR SELECT TO anon
    USING (true);

-- orchestrator_events: Dashboard shows event feed
-- Already has policy from migration 025, creating if not exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies 
        WHERE tablename = 'orchestrator_events' 
        AND policyname = 'anon_read_orchestrator_events'
    ) THEN
        CREATE POLICY "anon_read_orchestrator_events" ON public.orchestrator_events
            FOR SELECT TO anon
            USING (true);
    END IF;
END $$;

-- =====================================================
-- 4. INTERNAL TABLES - NO ANON ACCESS
-- =====================================================
-- These tables are for governor internal use only
-- No policies = no access for anon

-- task_packets: Contains prompts and specs (internal)
-- tools: Tool registry (internal)
-- projects: Project tracking (internal)
-- models_new: Legacy/alternate table (internal)
-- task_history: Legacy table (internal)

-- Explicitly deny (optional, but clear)
CREATE POLICY "anon_no_access" ON public.task_packets
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.tools
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.projects
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.models_new
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.task_history
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

-- =====================================================
-- 5. FIX SECURITY DEFINER VIEWS
-- =====================================================
-- Recreate views without SECURITY DEFINER
-- This removes the elevated privilege issue

-- Drop and recreate platform_health view
DROP VIEW IF EXISTS public.platform_health CASCADE;

CREATE VIEW public.platform_health AS
SELECT 
    p.id,
    p.name,
    p.status,
    COUNT(tr.id) as total_runs,
    AVG(CASE WHEN tr.status = 'success' THEN 1 ELSE 0 END) as success_rate
FROM public.platforms p
LEFT JOIN public.task_runs tr ON tr.platform = p.id
GROUP BY p.id, p.name, p.status;

-- Drop and recreate slice_roi view
DROP VIEW IF EXISTS public.slice_roi CASCADE;

CREATE VIEW public.slice_roi AS
SELECT 
    t.slice_id,
    COUNT(t.id) as task_count,
    SUM(tr.total_actual_cost_usd) as total_cost,
    SUM(tr.total_savings_usd) as total_savings,
    AVG(tr.total_savings_usd) as avg_savings_per_task
FROM public.tasks t
LEFT JOIN public.task_runs tr ON tr.task_id = t.id
WHERE t.slice_id IS NOT NULL
GROUP BY t.slice_id;

-- Drop and recreate roi_dashboard view  
DROP VIEW IF EXISTS public.roi_dashboard CASCADE;

CREATE VIEW public.roi_dashboard AS
SELECT 
    p.id as project_id,
    p.name as project_name,
    COUNT(t.id) as total_tasks,
    SUM(CASE WHEN t.status = 'merged' THEN 1 ELSE 0 END) as completed_tasks,
    AVG(tr.total_savings_usd) as avg_savings_per_task
FROM public.projects p
LEFT JOIN public.tasks t ON t.slice_id = ANY(p.slice_ids)
LEFT JOIN public.task_runs tr ON tr.task_id = t.id
GROUP BY p.id, p.name;

-- =====================================================
-- 6. VERIFICATION QUERIES
-- =====================================================
-- Run these in Supabase SQL Editor to verify:

-- Check RLS is enabled:
-- SELECT schemaname, tablename, rowsecurity 
-- FROM pg_tables 
-- WHERE schemaname = 'public' 
-- AND tablename IN ('tasks', 'task_runs', 'models', 'platforms');

-- Check policies exist:
-- SELECT schemaname, tablename, policyname, roles, cmd 
-- FROM pg_policies 
-- WHERE schemaname = 'public'
-- ORDER BY tablename, policyname;

-- =====================================================
-- NOTES
-- =====================================================
-- After applying this migration:
-- 1. All public tables have RLS enabled (fixes security warnings)
-- 2. SERVICE_KEY bypasses RLS (governor works unchanged)
-- 3. ANON_KEY can read: tasks, task_runs, models, platforms, orchestrator_events
-- 4. ANON_KEY cannot read: task_packets, tools, projects, models_new, task_history
-- 5. Dashboard continues working (uses anon key)
-- 6. SECURITY DEFINER views removed (security issue fixed)
-- 7. Vault table already has RLS (anon blocked, service_role only)
