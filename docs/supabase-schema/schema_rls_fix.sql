-- RLS POLICIES FOR SERVICE ROLE ACCESS
-- Run this to allow VibePilot backend to access tables

-- Disable RLS for service role (simpler for backend access)
-- OR add policies below

-- Option A: Disable RLS entirely (for backend-only use)
ALTER TABLE projects DISABLE ROW LEVEL SECURITY;
ALTER TABLE tasks DISABLE ROW LEVEL SECURITY;
ALTER TABLE task_packets DISABLE ROW LEVEL SECURITY;
ALTER TABLE task_runs DISABLE ROW LEVEL SECURITY;
ALTER TABLE models DISABLE ROW LEVEL SECURITY;
ALTER TABLE platforms DISABLE ROW LEVEL SECURITY;

-- Option B: Keep RLS but add service role policy (uncomment if needed)
/*
CREATE POLICY "Service role full access" ON projects FOR ALL USING (true);
CREATE POLICY "Service role full access" ON tasks FOR ALL USING (true);
CREATE POLICY "Service role full access" ON task_packets FOR ALL USING (true);
CREATE POLICY "Service role full access" ON task_runs FOR ALL USING (true);
CREATE POLICY "Service role full access" ON models FOR ALL USING (true);
CREATE POLICY "Service role full access" ON platforms FOR ALL USING (true);
*/

SELECT 'RLS configured for backend access';
