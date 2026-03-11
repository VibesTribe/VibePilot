-- Migration: Add missing tables to realtime publication
-- Purpose: Dashboard needs these tables to show live data
-- Date: 2026-03-11

-- Add tables that dashboard needs but weren't in realtime publication
ALTER PUBLICATION supabase_realtime ADD TABLE public.task_runs;
ALTER PUBLICATION supabase_realtime ADD TABLE public.models;
ALTER PUBLICATION supabase_realtime ADD TABLE public.platforms;

-- Verify
SELECT tablename FROM pg_publication_tables 
WHERE pubname = 'supabase_realtime'
ORDER BY tablename;
