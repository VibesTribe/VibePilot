-- Fix RLS for new tables - allow service role to insert/update
-- Run this in Supabase SQL Editor

-- Disable RLS on new tables (service role only, dashboard uses anon key for read)
ALTER TABLE models_new DISABLE ROW LEVEL SECURITY;
ALTER TABLE access DISABLE ROW LEVEL SECURITY;
ALTER TABLE task_history DISABLE ROW LEVEL SECURITY;

-- For tools table (already exists), also disable RLS
ALTER TABLE tools DISABLE ROW LEVEL SECURITY;

-- Alternative: If you want to keep RLS, use these policies instead:
-- CREATE POLICY "Service role can do anything on models_new" ON models_new FOR ALL TO service_role USING (true) WITH CHECK (true);
-- CREATE POLICY "Service role can do anything on access" ON access FOR ALL TO service_role USING (true) WITH CHECK (true);
-- CREATE POLICY "Service role can do anything on tools" ON tools FOR ALL TO service_role USING (true) WITH CHECK (true);
-- CREATE POLICY "Service role can do anything on task_history" ON task_history FOR ALL TO service_role USING (true) WITH CHECK (true);

-- Verify RLS is disabled
SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('models_new', 'tools', 'access', 'task_history');
