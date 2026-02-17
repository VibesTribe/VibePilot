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
  ('550e8400-e29b-41d4-a716-446655440101', 'Setup authentication', 'code', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 1),
  ('550e8400-e29b-41d4-a716-446655440102', 'Create user model', 'code', 'merged', 'auth', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440103', 'Add login UI', 'code', 'merged', 'ui', '550e8400-e29b-41d4-a716-446655440001', 3),
  ('550e8400-e29b-41d4-a716-446655440104', 'Build API endpoints', 'code', 'in_progress', 'api', '550e8400-e29b-41d4-a716-446655440001', 2),
  ('550e8400-e29b-41d4-a716-446655440105', 'Write tests', 'code', 'pending', 'testing', '550e8400-e29b-41d4-a716-446655440001', 4)
ON CONFLICT (id) DO NOTHING;

-- Sample task_runs with ROI data
-- Simulating: Courier (gemini-api) went to Claude (free tier web)
INSERT INTO task_runs (
  id, task_id, model_id, platform, courier, status,
  tokens_in, tokens_out, courier_model_id, courier_tokens, courier_cost_usd,
  platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
  started_at, completed_at
) VALUES
  -- Task 1: Claude via courier (free tier web, but courier cost)
  (
    '550e8400-e29b-41d4-a716-446655440201',
    '550e8400-e29b-41d4-a716-446655440101',
    'claude-3-sonnet',
    'claude',
    'gemini-api',
    'success',
    2500,   -- tokens_in (prompt sent to Claude)
    4200,   -- tokens_out (Claude's response)
    'gemini-api',
    800,    -- courier_tokens (gemini driving browser-use)
    0.02,   -- courier_cost_usd (gemini is free tier API = $0)
    0.35,   -- theoretical: (2.5K * $0.003) + (4.2K * $0.015) = $0.0075 + $0.063 = ~$0.07.. wait let me recalc
            -- Claude API: $3/M input, $15/M output = $0.003/1K input, $0.015/1K output
            -- 2.5K * $0.003 + 4.2K * $0.015 = $0.0075 + $0.063 = $0.0705... 
            -- Actually let's use realistic numbers
    0.0705,
    0.02,   -- actual cost (courier only)
    0.0505, -- savings
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
    1800,
    3100,
    'gemini-api',
    650,
    0.015,
    0.0513,  -- 1.8K * $0.003 + 3.1K * $0.015
    0.015,
    0.0363,
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
    3200,
    5800,
    'gemini-api',
    920,
    0.025,
    0.174,   -- GPT-4 API: $10/M input, $30/M output = 3.2K*$0.01 + 5.8K*$0.03
    0.025,
    0.149,
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '30 minutes'
  ),
  -- Task 4: In progress - DeepSeek API direct (no courier)
  (
    '550e8400-e29b-41d4-a716-446655440204',
    '550e8400-e29b-41d4-a716-446655440104',
    'deepseek-chat',
    'deepseek-api',
    NULL,
    'running',
    1500,
    0,
    NULL,
    0,
    0,
    0.00021, -- DeepSeek pricing is cheap: $0.14/M input, $0.28/M output
    0.00021,
    0,
    NOW() - INTERVAL '10 minutes',
    NULL
  )
ON CONFLICT (id) DO NOTHING;

-- Update project totals (trigger should do this, but let's be sure)
UPDATE projects SET
  total_tasks = 5,
  completed_tasks = 3,
  total_tokens_used = (2500+4200) + (1800+3100) + (3200+5800),
  total_theoretical_cost = 0.0705 + 0.0513 + 0.174,
  total_actual_cost = 0.02 + 0.015 + 0.025,
  total_savings = 0.0505 + 0.0363 + 0.149,
  updated_at = NOW()
WHERE id = '550e8400-e29b-41d4-a716-446655440001';

-- Add subscription info to a model (Kimi)
UPDATE models SET
  subscription_cost_usd = 0.99,
  subscription_started_at = NOW() - INTERVAL '14 days',
  subscription_ends_at = NOW() + INTERVAL '14 days',
  subscription_status = 'active',
  tasks_completed = 47,
  tasks_failed = 2,
  tokens_used = 125000
WHERE id = 'kimi-cli';

-- Add subscription info to opencode (GLM-5)
UPDATE models SET
  subscription_cost_usd = 45.00,
  subscription_started_at = NOW() - INTERVAL '30 days',
  subscription_ends_at = NOW() + INTERVAL '60 days',
  subscription_status = 'active',
  tasks_completed = 156,
  tasks_failed = 8,
  tokens_used = 890000
WHERE id = 'opencode';

-- Add theoretical costs to platforms (what their API would cost)
UPDATE platforms SET
  theoretical_cost_input_per_1k_usd = 0.003,
  theoretical_cost_output_per_1k_usd = 0.015
WHERE id = 'claude';

UPDATE platforms SET
  theoretical_cost_input_per_1k_usd = 0.01,
  theoretical_cost_output_per_1k_usd = 0.03
WHERE id = 'chatgpt';

UPDATE platforms SET
  theoretical_cost_input_per_1k_usd = 0.00014,
  theoretical_cost_output_per_1k_usd = 0.00028
WHERE id = 'deepseek';

-- Verify the data
SELECT 
  'task_runs' as table_name, 
  COUNT(*) as count,
  SUM(tokens_in) as total_tokens_in,
  SUM(tokens_out) as total_tokens_out,
  SUM(total_savings_usd) as total_savings
FROM task_runs
WHERE tokens_in IS NOT NULL;

SELECT * FROM slice_roi;

SELECT * FROM get_all_subscriptions();
