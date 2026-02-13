-- VIBESPILOT PROJECT TRACKING + ENHANCED MODELS
-- For cumulative ROI and real-time tracking

-- PROJECTS TABLE
CREATE TABLE projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT,
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed', 'archived')),
  
  -- Cumulative Metrics
  total_tasks INT DEFAULT 0,
  completed_tasks INT DEFAULT 0,
  total_tokens_used BIGINT DEFAULT 0,
  total_theoretical_cost FLOAT DEFAULT 0,
  total_actual_cost FLOAT DEFAULT 0,
  total_savings FLOAT DEFAULT 0,
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Add project_id to tasks
ALTER TABLE tasks ADD COLUMN project_id UUID REFERENCES projects(id);

-- Add API costs to models (for ROI calculation)
ALTER TABLE models ADD COLUMN api_cost_per_1k_tokens FLOAT DEFAULT 0;
ALTER TABLE models ADD COLUMN subscription_monthly_cost FLOAT DEFAULT 0;

-- Update model costs
UPDATE models SET 
  api_cost_per_1k_tokens = 0.14,  -- DeepSeek pricing
  subscription_monthly_cost = 0   -- Pay per use
WHERE id = 'deepseek-chat';

-- TASK RUN ENHANCEMENTS (add ROI fields)
ALTER TABLE task_runs ADD COLUMN theoretical_api_cost FLOAT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN actual_cost FLOAT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN savings FLOAT DEFAULT 0;

-- FUNCTION: Calculate ROI for completed task
CREATE OR REPLACE FUNCTION calculate_task_roi(p_run_id UUID)
RETURNS VOID AS $$
DECLARE
  v_tokens INT;
  v_model_id TEXT;
  v_api_cost FLOAT;
  v_theoretical FLOAT;
  v_actual FLOAT;
  v_platform_id TEXT;
BEGIN
  -- Get run data
  SELECT tokens_used, model_id, platform INTO v_tokens, v_model_id, v_platform_id
  FROM task_runs WHERE id = p_run_id;
  
  -- Get API cost for model
  SELECT api_cost_per_1k_tokens INTO v_api_cost
  FROM models WHERE id = v_model_id;
  
  -- Calculate theoretical API cost
  v_theoretical := (COALESCE(v_tokens, 0) / 1000.0) * COALESCE(v_api_cost, 0);
  
  -- Actual cost (0 for free tier web platforms, courier time for others)
  SELECT theoretical_api_cost_per_1k_tokens * (COALESCE(v_tokens, 0) / 1000.0) INTO v_actual
  FROM platforms WHERE id = v_platform_id;
  
  v_actual := COALESCE(v_actual, 0); -- Usually 0 for free tier
  
  -- Update run with ROI
  UPDATE task_runs
  SET theoretical_api_cost = v_theoretical,
      actual_cost = v_actual,
      savings = v_theoretical - v_actual
  WHERE id = p_run_id;
  
  -- Update project cumulative totals
  UPDATE projects p
  SET total_tokens_used = total_tokens_used + COALESCE(v_tokens, 0),
      total_theoretical_cost = total_theoretical_cost + v_theoretical,
      total_actual_cost = total_actual_cost + v_actual,
      total_savings = total_savings + (v_theoretical - v_actual),
      updated_at = NOW()
  FROM tasks t
  WHERE t.id = (SELECT task_id FROM task_runs WHERE id = p_run_id)
    AND p.id = t.project_id;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Get project ROI summary (REAL-TIME)
CREATE OR REPLACE FUNCTION get_project_roi(p_project_id UUID)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  SELECT jsonb_build_object(
    'project_id', p_project_id,
    'project_name', name,
    'status', status,
    'total_tasks', total_tasks,
    'completed_tasks', completed_tasks,
    'completion_rate', CASE WHEN total_tasks > 0 
                           THEN ROUND((completed_tasks::FLOAT / total_tasks) * 100, 1)
                           ELSE 0 END,
    'total_tokens', total_tokens_used,
    'total_theoretical_cost', ROUND(total_theoretical_cost, 2),
    'total_actual_cost', ROUND(total_actual_cost, 2),
    'total_savings', ROUND(total_savings, 2),
    'roi_percentage', CASE WHEN total_actual_cost > 0 
                          THEN ROUND((total_savings / total_actual_cost) * 100, 1)
                          ELSE 0 END,
    'avg_tokens_per_task', CASE WHEN completed_tasks > 0 
                                THEN ROUND(total_tokens_used::FLOAT / completed_tasks, 0)
                                ELSE 0 END
  ) INTO v_result
  FROM projects WHERE id = p_project_id;
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- FUNCTION: Get all projects summary
CREATE OR REPLACE FUNCTION get_all_projects_roi()
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  SELECT jsonb_agg(
    jsonb_build_object(
      'project_id', id,
      'project_name', name,
      'status', status,
      'completed_tasks', completed_tasks,
      'total_savings', ROUND(total_savings, 2),
      'completion_rate', CASE WHEN total_tasks > 0 
                             THEN ROUND((completed_tasks::FLOAT / total_tasks) * 100, 1)
                             ELSE 0 END
    )
  ) INTO v_result
  FROM projects
  ORDER BY updated_at DESC;
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- VIEW: Real-time ROI dashboard
CREATE VIEW roi_dashboard AS
SELECT 
  p.id as project_id,
  p.name as project_name,
  p.status,
  p.total_tasks,
  p.completed_tasks,
  ROUND((p.completed_tasks::FLOAT / NULLIF(p.total_tasks, 0)) * 100, 1) as completion_pct,
  p.total_tokens_used,
  ROUND(p.total_theoretical_cost, 2) as would_have_cost,
  ROUND(p.total_actual_cost, 2) as actually_cost,
  ROUND(p.total_savings, 2) as savings,
  CASE WHEN p.total_actual_cost > 0 
       THEN ROUND((p.total_savings / p.total_actual_cost) * 100, 1)
       ELSE 0 END as roi_pct
FROM projects p
ORDER BY p.updated_at DESC;

-- TRIGGER: Update project task counts
CREATE OR REPLACE FUNCTION update_project_counts()
RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    UPDATE projects 
    SET total_tasks = total_tasks + 1,
        updated_at = NOW()
    WHERE id = NEW.project_id;
  ELSIF TG_OP = 'UPDATE' AND NEW.status = 'merged' AND OLD.status != 'merged' THEN
    UPDATE projects 
    SET completed_tasks = completed_tasks + 1,
        updated_at = NOW()
    WHERE id = NEW.project_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_project_counts
AFTER INSERT OR UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION update_project_counts();

SELECT 'Project tracking + ROI schema applied';
