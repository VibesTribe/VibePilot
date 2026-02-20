-- Vibes Query RPC
-- Main function for Vibes AI assistant to query system state
-- Returns real project data, ROI summary, platform health, and recent activity

CREATE OR REPLACE FUNCTION vibes_query(
  p_user_id TEXT,
  p_question TEXT,
  p_context JSONB DEFAULT '{}'
)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
  v_user_prefs JSONB;
BEGIN
  -- Get user preferences (if table exists, handle gracefully if not)
  BEGIN
    SELECT to_jsonb(vp.*) INTO v_user_prefs
    FROM vibes_preferences vp
    WHERE vp.user_id = p_user_id;
  EXCEPTION
    WHEN undefined_table THEN
      v_user_prefs := NULL;
  END;

  -- Build comprehensive response with real system data
  v_result := jsonb_build_object(
    -- Response to user question
    'response', 'Vibes here! I received your question: ' || p_question,
    'status', 'ok',
    
    -- Project status
    'active_projects', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', p.id,
        'name', p.name,
        'status', p.status,
        'progress', CASE WHEN p.total_tasks > 0 THEN ROUND((p.completed_tasks::float / p.total_tasks) * 100) ELSE 0 END,
        'tasks_completed', p.completed_tasks,
        'tasks_pending', p.total_tasks - p.completed_tasks
      ))
      FROM projects p
      WHERE p.status = 'active'
    ),
    
    -- ROI summary (last 7 days)
    'roi_summary', (
      SELECT jsonb_build_object(
        'total_tasks', COUNT(*),
        'tokens_in', COALESCE(SUM(tokens_in), 0),
        'tokens_out', COALESCE(SUM(tokens_out), 0),
        'actual_cost_usd', COALESCE(SUM(actual_cost_usd), 0),
        'theoretical_cost_usd', COALESCE(SUM(theoretical_cost_usd), 0),
        'savings_usd', COALESCE(SUM(theoretical_cost_usd - actual_cost_usd), 0)
      )
      FROM task_runs
      WHERE completed_at > NOW() - INTERVAL '7 days'
    ),
    
    -- Platform health
    'platform_health', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', m.id,
        'name', m.name,
        'status', m.status,
        'success_rate', m.success_rate,
        'tokens_used_24h', (
          SELECT COALESCE(SUM(tokens_in + tokens_out), 0)
          FROM task_runs tr
          WHERE tr.model_id = m.id 
          AND tr.completed_at > NOW() - INTERVAL '24 hours'
        )
      ))
      FROM models m
      WHERE m.status IN ('active', 'paused')
    ),
    
    -- Recent activity (last 5 tasks)
    'recent_activity', (
      SELECT jsonb_agg(jsonb_build_object(
        'task_id', t.id,
        'title', t.title,
        'status', t.status,
        'updated_at', t.updated_at
      ))
      FROM tasks t
      ORDER BY t.updated_at DESC
      LIMIT 5
    ),
    
    -- Alerts requiring attention (escalated tasks)
    'alerts', (
      SELECT jsonb_agg(jsonb_build_object(
        'type', 'escalated_task',
        'task_id', t.id,
        'title', t.title,
        'reason', t.status_reason
      ))
      FROM tasks t
      WHERE t.status = 'escalated'
    ),
    
    -- User preferences
    'user_preferences', v_user_prefs,
    
    -- Query metadata
    'query_meta', jsonb_build_object(
      'timestamp', NOW(),
      'question', p_question
    )
  );
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- Grant execute to anon and authenticated users
GRANT EXECUTE ON FUNCTION vibes_query(TEXT, TEXT, JSONB) TO anon;
GRANT EXECUTE ON FUNCTION vibes_query(TEXT, TEXT, JSONB) TO authenticated;

-- Add comment for documentation
COMMENT ON FUNCTION vibes_query(TEXT, TEXT, JSONB) IS 
'Vibes AI assistant query function. Returns real-time project status, ROI summary, platform health, and recent activity.';
