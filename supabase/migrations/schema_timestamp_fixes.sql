-- Schema Fixes: Add Missing Timestamps (Senior Engineer Rules)
-- Date: 2026-02-14
-- Purpose: Ensure all tables have created_at and updated_at for auditing

-- =============================================================================
-- Add missing timestamps
-- =============================================================================

-- task_packets: Add updated_at
ALTER TABLE task_packets 
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- models: Add created_at
ALTER TABLE models 
ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();

-- task_runs: Add created_at and updated_at
ALTER TABLE task_runs 
ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();

ALTER TABLE task_runs 
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- =============================================================================
-- Add triggers for auto-updating updated_at
-- =============================================================================

-- Create update function if not exists
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add triggers to tables with updated_at
DROP TRIGGER IF EXISTS update_task_packets_updated_at ON task_packets;
CREATE TRIGGER update_task_packets_updated_at
    BEFORE UPDATE ON task_packets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_models_updated_at ON models;
CREATE TRIGGER update_models_updated_at
    BEFORE UPDATE ON models
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_task_runs_updated_at ON task_runs;
CREATE TRIGGER update_task_runs_updated_at
    BEFORE UPDATE ON task_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Verification
-- =============================================================================

-- Run this to verify:
-- SELECT table_name, column_name 
-- FROM information_schema.columns 
-- WHERE table_schema = 'public' 
-- AND column_name IN ('created_at', 'updated_at')
-- ORDER BY table_name, column_name;
