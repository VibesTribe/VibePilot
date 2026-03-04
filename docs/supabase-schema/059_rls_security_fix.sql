-- Migration: 059_rls_security_fix.sql
-- Purpose: Enable RLS on all public tables and fix security issues
-- Date: 2026-03-04
-- Author: GLM-5 Session 47

-- =====================================================
-- 1. ENABLE ROW LEVEL SECURITY ON ALL TABLES
-- =====================================================

-- Enable RLS on tables flagged as insecure
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
-- 2. CREATE SERVICE ROLE BYPASS POLICIES
-- =====================================================
-- The governor uses the service_role key and needs full access

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
-- 3. DENY ANON ACCESS BY DEFAULT
-- =====================================================
-- Anon key should have no access to internal tables

CREATE POLICY "anon_no_access" ON public.platforms
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

CREATE POLICY "anon_no_access" ON public.tools
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.tasks
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.task_runs
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.task_packets
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

CREATE POLICY "anon_no_access" ON public.models
    FOR ALL TO anon
    USING (false)
    WITH CHECK (false);

-- =====================================================
-- 4. FIX SECURITY DEFINER Views
-- =====================================================
-- Recreate views without SECURITY DEFINER

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
-- 5. VERIFY RLS IS ENABLED
-- =====================================================
-- Run this query to verify all tables have RLS enabled:
-- SELECT schemaname, tablename, rowsecurity 
-- FROM pg_tables 
-- WHERE schemaname = 'public' 
-- AND rowsecurity = false;

-- =====================================================
-- 6. NOTES
-- =====================================================
-- After applying this migration:
-- 1. All public tables will have RLS enabled
-- 2. Service role will have full access (governor uses this)
-- 3. Anon key will be denied access
-- 4. SECURITY DEFINER views recreated without elevated privileges
--
-- If you need to expose specific tables to anon users (e.g., for dashboard):
-- Create specific policies like:
-- CREATE POLICY "anon_read_own" ON public.tasks
--     FOR SELECT TO anon
--     USING (created_by = auth.uid());
