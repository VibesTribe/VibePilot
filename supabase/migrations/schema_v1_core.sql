-- VIBESPILOT CORE SCHEMA v1.0
-- Purpose: Production-grade multi-agent task orchestration
-- Design: Atomic task claiming, full traceability, revision chains, cost control

-- DROP EXISTING
DROP TABLE IF EXISTS task_runs CASCADE;
DROP TABLE IF EXISTS task_packets CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;
DROP TABLE IF EXISTS models CASCADE;

-- 1. MODELS & PLATFORMS REGISTRY
CREATE TABLE models (
  id TEXT PRIMARY KEY,
  platform TEXT NOT NULL,
  courier TEXT NOT NULL,
  
  -- Capabilities
  context_limit INT,
  strengths TEXT[] DEFAULT '{}',
  weaknesses TEXT[] DEFAULT '{}',
  
  -- Usage & Limits (Cost Control)
  request_limit INT,
  request_used INT DEFAULT 0,
  token_limit INT,
  token_used INT DEFAULT 0,
  cycle_resets_at TIMESTAMPTZ,
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'benched', 'paused', 'offline')),
  status_reason TEXT,
  
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. TASKS (Single source of truth)
CREATE TABLE tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title TEXT,
  type TEXT NOT NULL,
  priority INT DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
  
  -- Dependencies
  dependencies UUID[] DEFAULT '{}',
  
  -- Status Flow: pending → available → in_progress → review → testing → approval → merged
  status TEXT DEFAULT 'pending' CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged'
  )),
  
  -- Assignment
  assigned_to TEXT,
  attempts INT DEFAULT 0,
  max_attempts INT DEFAULT 5,
  
  -- Results (JSONB for flexibility)
  result JSONB,
  review JSONB,
  tests JSONB,
  approval JSONB,
  
  -- Failure handling (never abandon, always resolve)
  failure_notes TEXT,
  
  -- Branch tracking
  branch_name TEXT,
  
  -- Timestamps
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

-- 3. TASK PACKETS (Versioned work orders)
CREATE TABLE task_packets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
  
  -- What the agent needs
  prompt TEXT NOT NULL,
  tech_spec JSONB,
  expected_output TEXT,
  context JSONB,
  
  -- Versioning for revisions
  version INT DEFAULT 1,
  revision_reason TEXT,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 4. TASK RUNS (Full execution chain + chat URLs)
CREATE TABLE task_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id),
  
  -- Full chain: who → where → what
  courier TEXT NOT NULL,
  platform TEXT NOT NULL,
  model_id TEXT REFERENCES models(id),
  
  -- Revision capability (return to exact context)
  chat_url TEXT,
  
  -- Result
  status TEXT DEFAULT 'running' CHECK (status IN ('running', 'success', 'failed', 'timeout')),
  result JSONB,
  error TEXT,
  
  -- Cost tracking
  tokens_used INT,
  
  -- Timing
  started_at TIMESTAMPTZ DEFAULT NOW(),
  completed_at TIMESTAMPTZ
);

-- INDEXES FOR PERFORMANCE
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_priority ON tasks(priority, created_at);
CREATE INDEX idx_tasks_deps ON tasks USING GIN(dependencies);
CREATE INDEX idx_runs_task ON task_runs(task_id);
CREATE INDEX idx_runs_status ON task_runs(status);
CREATE INDEX idx_models_status ON models(status);

-- FUNCTION: Atomic task claiming (race-condition safe)
CREATE OR REPLACE FUNCTION claim_next_task(
  p_courier TEXT,
  p_platform TEXT,
  p_model_id TEXT
) RETURNS UUID AS $$
DECLARE
  v_task_id UUID;
BEGIN
  UPDATE tasks
  SET status = 'in_progress',
      assigned_to = p_model_id,
      attempts = attempts + 1,
      started_at = COALESCE(started_at, NOW()),
      updated_at = NOW()
  WHERE id = (
    SELECT id FROM tasks
    WHERE status = 'available'
      AND (
        dependencies = '{}'
        OR dependencies <@ (
          SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
        )
      )
    ORDER BY priority ASC, created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
  )
  RETURNING id INTO v_task_id;
  
  RETURN v_task_id;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Get available tasks (for dashboard)
CREATE OR REPLACE FUNCTION get_available_tasks()
RETURNS TABLE (
  id UUID,
  title TEXT,
  type TEXT,
  priority INT,
  attempts INT,
  created_at TIMESTAMPTZ
) AS $$
BEGIN
  RETURN QUERY
  SELECT t.id, t.title, t.type, t.priority, t.attempts, t.created_at
  FROM tasks t
  WHERE t.status = 'available'
    OR (
      t.status = 'pending' 
      AND (
        t.dependencies = '{}'
        OR t.dependencies <@ (
          SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
        )
      )
    )
  ORDER BY t.priority ASC, t.created_at ASC;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Mark task available (after dependency check)
CREATE OR REPLACE FUNCTION make_task_available(p_task_id UUID)
RETURNS VOID AS $$
BEGIN
  UPDATE tasks
  SET status = 'available',
      updated_at = NOW()
  WHERE id = p_task_id
    AND status = 'pending'
    AND (
      dependencies = '{}'
      OR dependencies <@ (
        SELECT ARRAY_AGG(id) FROM tasks WHERE status = 'merged'
      )
    );
END;
$$ LANGUAGE plpgsql;

-- SEED INITIAL MODEL (OpenCode/DeepSeek)
INSERT INTO models (id, platform, courier, context_limit, strengths, status, request_limit, cycle_resets_at)
VALUES (
  'deepseek-chat',
  'deepseek-api',
  'opencode',
  64000,
  ARRAY['code', 'reasoning', 'planning'],
  'active',
  1000,
  NOW() + INTERVAL '1 month'
);

-- SUCCESS
SELECT 'VibePilot schema initialized. Models registered: ' || COUNT(*) FROM models;
