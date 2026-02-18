-- RPC function to increment access usage
-- Run in Supabase SQL Editor

CREATE OR REPLACE FUNCTION increment_access_usage(
    p_access_id UUID,
    p_tokens INT,
    p_success BOOLEAN
)
RETURNS void AS $$
BEGIN
    UPDATE access
    SET 
        requests_today = COALESCE(requests_today, 0) + 1,
        tokens_today = COALESCE(tokens_today, 0) + p_tokens,
        total_tasks = COALESCE(total_tasks, 0) + 1,
        successful_tasks = CASE WHEN p_success THEN COALESCE(successful_tasks, 0) + 1 ELSE successful_tasks END,
        failed_tasks = CASE WHEN NOT p_success THEN COALESCE(failed_tasks, 0) + 1 ELSE failed_tasks END,
        avg_tokens_per_task = (
            COALESCE(avg_tokens_per_task, 0) * COALESCE(total_tasks, 0) + p_tokens
        ) / (COALESCE(total_tasks, 0) + 1),
        updated_at = NOW()
    WHERE id = p_access_id;
END;
$$ LANGUAGE plpgsql;

-- Function to reset daily usage (call via cron or at midnight)
CREATE OR REPLACE FUNCTION reset_daily_usage()
RETURNS void AS $$
BEGIN
    UPDATE access
    SET 
        requests_today = 0,
        tokens_today = 0,
        updated_at = NOW()
    WHERE status = 'active';
END;
$$ LANGUAGE plpgsql;

-- Verify functions exist
SELECT proname FROM pg_proc WHERE proname IN ('increment_access_usage', 'reset_daily_usage');
