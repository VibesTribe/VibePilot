-- Migration 142: Visual QA runs table
-- Tracks automated visual regression detection runs for the dashboard
-- GitHub is source of truth for baselines; this table tracks run history

CREATE TABLE IF NOT EXISTS visual_qa_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    triggered_by TEXT NOT NULL,          -- 'vercel_deploy', 'manual', 'post_merge', 'cron'
    trigger_detail TEXT,                 -- deploy URL, commit SHA, etc
    status TEXT NOT NULL DEFAULT 'running',  -- 'running', 'passed', 'failed', 'error'
    pages_checked INTEGER DEFAULT 0,
    pages_passed INTEGER DEFAULT 0,
    pages_failed INTEGER DEFAULT 0,
    results JSONB,                       -- array of {page, viewport, passed, differences}
    error_message TEXT,                  -- error details if status='error'
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER
);

CREATE INDEX IF NOT EXISTS idx_visual_qa_runs_status ON visual_qa_runs(status);
CREATE INDEX IF NOT EXISTS idx_visual_qa_runs_started ON visual_qa_runs(started_at DESC);

-- Add trigger for pg_notify so dashboard gets realtime updates
CREATE OR REPLACE FUNCTION notify_visual_qa_change() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('visual_qa_changes', json_build_object(
        'op', TG_OP,
        'id', COALESCE(NEW.id, OLD.id),
        'status', COALESCE(NEW.status, OLD.status)
    )::text);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS visual_qa_notify ON visual_qa_runs;
CREATE TRIGGER visual_qa_notify
    AFTER INSERT OR UPDATE ON visual_qa_runs
    FOR EACH ROW EXECUTE FUNCTION notify_visual_qa_change();
