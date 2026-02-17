-- TEST DATA FOR ROI CALCULATOR
-- Run this in Supabase SQL Editor to populate sample task_runs
-- Then check the dashboard ROI panel

-- First, make sure we have a project
INSERT INTO projects (id, name, status, total_tasks, completed_tasks)
VALUES (
  '550e8400-e29b-41d4-a716-446655440001',
  'VibePilot Core',
  'active',
  5,
  3
) ON CONFLICT (id) DO NOTHING;

-- Sample tasks with slice_id
INSERT INTO tasks (id, title, type, status, slice_id, project_id, priority) VALUES
  ('550e8400-e29b-41d4-a716-446655440101', 'Setup authentication', 'setup', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 1),
  ('550e8400-e29b-41d4-a716-446655440102', 'Create user model', 'feature', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440103', 'Add login UI', 'ui_ux', 'merged', 'ui', '550e8400-e29b-41d4-a716-446655440001', 3),
  ('550e8400-e29b-41d4-a716-446655440104', 'Build API endpoints', 'api', 'in_progress', 'api', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440105', 'Write tests', 'test', 'pending', 'testing', '550e8400-e29b-41d4-a716-446655440001', 4)
ON CONFLICT (id) DO NOTHING;

-- Sample task_runs with ROI data
-- Simulating: Courier (gemini-api) went to Claude (free tier web)
INSERT INTO task_runs (
  id, task_id, model_id, platform, courier, status, tokens_used, started_at, completed_at
) VALUES
  -- Task 1: Claude via courier (free tier web, but courier cost)
  (
    '550e8400-e29b-41d4-a716-446655440201',
    '550e8400-e29b-41d4-a716-446655440101',
    'claude-3-sonnet',
    'claude',
    'gemini-api',
    'success',
    6700,   -- total tokens
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '1 hour 50 minutes'
  ),
  -- Task 2: Another Claude run
  (
    '550e8400-e29b-41d4-a716-446655440202',
    '550e8400-e29b-41d4-a716-446655440102',
    'claude-3-sonnet',
    'claude',
    'gemini-api',
    'success',
    4900,
    NOW() - INTERVAL '1 hour 30 minutes',
    NOW() - INTERVAL '1 hour 15 minutes'
  ),
  -- Task 3: ChatGPT via courier
  (
    '550e8400-e29b-41d4-a716-446655440203',
    '550e8400-e29b-41d4-a716-446655440103',
    'gpt-4',
    'chatgpt',
    'gemini-api',
    'success',
    9000,
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '30 minutes'
  ),
  -- Task 4: In progress - DeepSeek API direct (no courier)
  (
    '550e8400-e29b-41d4-a716-446655440204',
    '550e8400-e29b-41d4-a716-446655440104',
    'deepseek-chat',
    'deepseek-api',
    'internal',
    'running',
    1500,
    NOW() - INTERVAL '10 minutes',
    NULL
  )
ON CONFLICT (id) DO NOTHING;

-- Update project totals
UPDATE projects SET
  total_tasks = 5,
  completed_tasks = 3,
  updated_at = NOW()
WHERE id = '550e8400-e29b-41d4-a716-446655440001';

-- Add subscription info to a model (Kimi) - only if columns exist
-- Run these manually after verifying v1.4 schema is applied
-- UPDATE models SET
--   subscription_cost_usd = 0.99,
--   subscription_started_at = NOW() - INTERVAL '14 days',
--   subscription_ends_at = NOW() + INTERVAL '14 days',
--   subscription_status = 'active',
--   tasks_completed = 47,
--   tokens_used = 125000
-- WHERE id = 'kimi-cli';

-- Verify the data
SELECT 
  'tasks' as table_name, 
  COUNT(*) as count
FROM tasks
WHERE project_id = '550e8400-e29b-41d4-a716-446655440001';

SELECT 
  'task_runs' as table_name, 
  COUNT(*) as count,
  SUM(tokens_used) as total_tokens
FROM task_runs
WHERE tokens_used IS NOT NULL;
