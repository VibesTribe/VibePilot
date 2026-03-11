-- Migration 088: Add missing task columns
-- Purpose: Align tasks table with dashboard expectations
-- Date: 2026-03-11
--
-- Dashboard expects:
 tasks table to have:
-- - id, title, status, assigned_to, slice_id, task_number, routing_flag
-- - result (jsonb with prompt_packet), - dependencies (jsonb)
-- - processing_by, processing_at, plan_id, confidence, category, max_attempts, attempts, failure_notes
-- - started_at, completed_at, updated_at

-- task_runs needs:
-- - id, task_id, model_id, courier, platform, tokens_in, tokens_out, tokens_used
-- - courier_tokens, courier_cost_usd, platform_theoretical_cost_usd
-- - total_actual_cost_usd, total_savings_usd
-- - started_at, completed_at

-- This migration adds the missing columns and ensures the tasks table is aligned with dashboard.

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS
    id UUID,
    title TEXT,
    status TEXT CHECK (status IN ('pending', 'available', 'in_progress', 'review', 'testing', 'approval', 'merged', 'awaiting_human', 'escalated', 'blocked')),
    assigned_to TEXT,
    slice_id TEXT DEFAULT 'general',
    task_number TEXT,
    routing_flag TEXT CHECK (routing_flag IN ('internal', 'mcp', 'web')),
    routing_flag_reason TEXT,
    result JSONB DEFAULT '{}'::jsonb,
    dependencies JSONB DEFAULT '[]'::jsonb,
    processing_by TEXT,
    processing_at TIMESTAMPTZ,
    plan_id UUID,
    confidence FLOAT,
    category TEXT,
    max_attempts INT DEFAULT 3,
    attempts INT DEFAULT 0,
    failure_notes TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS
    id UUID,
    task_id UUID,
    model_id TEXT,
    courier TEXT,
    platform TEXT,
    tokens_in INT,
    tokens_out INT,
    tokens_used INT,
    courier_tokens INT,
    courier_cost_usd DECIMAL,
    platform_theoretical_cost_usd DECIMAL,
    total_actual_cost_usd DECIMAL,
    total_savings_usd DECIMAL,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

SELECT '088_add_missing_task_columns applied' AS status;
