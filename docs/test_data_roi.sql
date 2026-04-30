-- TEST DATA FOR ROI CALCULATOR
-- Run this in Supabase SQL Editor to populate sample data
-- Uses only existing model and platform IDs from config

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
-- Valid types: setup, feature, bugfix, test, docs, refactor, ui_ux, api, infrastructure
INSERT INTO tasks (id, title, type, status, slice_id, project_id, priority) VALUES
  ('550e8400-e29b-41d4-a716-446655440101', 'Setup authentication', 'setup', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 1),
  ('550e8400-e29b-41d4-a716-446655440102', 'Create user model', 'feature', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440103', 'Add login UI', 'ui_ux', 'merged', 'ui', '550e8400-e29b-41d4-a716-446655440001', 3),
  ('550e8400-e29b-41d4-a716-446655440104', 'Build API endpoints', 'api', 'in_progress', 'api', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440105', 'Write tests', 'test', 'pending', 'testing', '550e8400-e29b-41d4-a716-446655440001', 4)
ON CONFLICT (id) DO NOTHING;

-- Sample task_runs using ONLY existing models (gemini-api, deepseek-chat, kimi-cli, opencode)
-- and existing platforms (copilot, claude, huggingchat, gemini, chatgpt, deepseek)
INSERT INTO task_runs (
  id, task_id, model_id, platform, courier, status, tokens_used, started_at, completed_at
) VALUES
  -- Task 1: kimi-cli executed directly on claude web platform
  (
    '550e8400-e29b-41d4-a716-446655440201',
    '550e8400-e29b-41d4-a716-446655440101',
    'kimi-cli',
    'claude',
    'kimi-cli',
    'success',
    6700,
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '1 hour 50 minutes'
  ),
  -- Task 2: gemini-api on chatgpt web
  (
    '550e8400-e29b-41d4-a716-446655440202',
    '550e8400-e29b-41d4-a716-446655440102',
    'gemini-api',
    'chatgpt',
    'gemini-api',
    'success',
    4900,
    NOW() - INTERVAL '1 hour 30 minutes',
    NOW() - INTERVAL '1 hour 15 minutes'
  ),
  -- Task 3: deepseek-chat on gemini web
  (
    '550e8400-e29b-41d4-a716-446655440203',
    '550e8400-e29b-41d4-a716-446655440103',
    'deepseek-chat',
    'gemini',
    'deepseek-chat',
    'success',
    9000,
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '30 minutes'
  ),
  -- Task 4: In progress - opencode on deepseek
  (
    '550e8400-e29b-41d4-a716-446655440204',
    '550e8400-e29b-41d4-a716-446655440104',
    'opencode',
    'deepseek',
    'opencode',
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

-- Verify the data
SELECT 'tasks' as tbl, COUNT(*) as count FROM tasks WHERE project_id = '550e8400-e29b-41d4-a716-446655440001'
UNION ALL
SELECT 'task_runs' as tbl, COUNT(*) as count FROM task_runs WHERE task_id IN (
  SELECT id FROM tasks WHERE project_id = '550e8400-e29b-41d4-a716-446655440001'
);
