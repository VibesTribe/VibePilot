-- ============================================================================
-- Migration 048: PIF Phase C — Project-scoped costs and counters
-- ============================================================================
-- Adds project_id to project_costs and system_counters so each project has its own cost tracking
-- and token/cost counters for the dashboard ROI panel.
-- Backfill: all existing rows assigned to vibepilot project.
-- ============================================================================

BEGIN;

-- ========================================
-- project_costs: add project_id column
-- ========================================
ALTER TABLE project_costs ADD COLUMN IF NOT EXISTS project_id uuid;

-- Backfill existing rows to vibepilot
UPDATE project_costs SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot')
WHERE project_id IS NULL;

ALTER TABLE project_costs ALTER COLUMN project_id SET NOT NULL;
-- Use literal vibepilot UUID for default (subqueries not allowed in DEFAULT)
ALTER TABLE project_costs ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';

CREATE INDEX IF NOT EXISTS idx_project_costs_project_id ON project_costs(project_id);

-- FK (add if not exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'project_costs_project_id_fkey') THEN
        ALTER TABLE project_costs
        ADD CONSTRAINT project_costs_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
    END IF;
END $$;

-- ========================================
-- system_counters: add project_id column
-- ========================================
ALTER TABLE system_counters ADD COLUMN IF NOT EXISTS project_id uuid;

UPDATE system_counters SET project_id = (SELECT id FROM projects WHERE slug = 'vibepilot')
WHERE project_id IS NULL;

ALTER TABLE system_counters ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE system_counters ALTER COLUMN project_id SET DEFAULT '947c2db2-ac1f-4307-9048-8d838ef3aacd';

CREATE INDEX IF NOT EXISTS idx_system_counters_project_id ON system_counters(project_id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'system_counters_project_id_fkey') THEN
        ALTER TABLE system_counters
        ADD CONSTRAINT system_counters_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
    END IF;
END $$;

COMMIT;

-- ========================================
-- Update RPC functions (run outside transaction — CREATE FUNCTION is transactional in PostgreSQL)
-- ========================================

-- add_project_cost: now accepts p_project_id (defaults to vibepilot)
CREATE OR REPLACE FUNCTION add_project_cost(
    p_category text,
    p_description text,
    p_amount_usd numeric,
    p_frequency text DEFAULT 'one_time',
    p_incurred_at timestamp with time zone DEFAULT now(),
    p_project_id uuid DEFAULT NULL
) RETURNS project_costs
LANGUAGE plpgsql
AS $func$
DECLARE
    v_row project_costs;
    v_proj_id uuid;
BEGIN
    IF p_project_id IS NULL THEN
        SELECT id INTO v_proj_id FROM projects WHERE slug = 'vibepilot';
    ELSE
        v_proj_id := p_project_id;
    END IF;

    INSERT INTO project_costs (category, description, amount_usd, frequency, incurred_at, project_id)
    VALUES (p_category, p_description, p_amount_usd, p_frequency, p_incurred_at, v_proj_id)
    RETURNING * INTO v_row;
    RETURN v_row;
END;
$func$;

-- list_project_costs: now accepts p_project_id to filter
CREATE OR REPLACE FUNCTION list_project_costs(
    p_include_archived boolean DEFAULT false,
    p_project_id uuid DEFAULT NULL
) RETURNS SETOF project_costs
LANGUAGE plpgsql
AS $func$
BEGIN
    IF p_project_id IS NOT NULL THEN
        IF p_include_archived THEN
            RETURN QUERY SELECT * FROM project_costs WHERE project_id = p_project_id ORDER BY incurred_at DESC;
        ELSE
            RETURN QUERY SELECT * FROM project_costs WHERE project_id = p_project_id AND archived_at IS NULL ORDER BY incurred_at DESC;
        END IF;
    ELSE
        IF p_include_archived THEN
            RETURN QUERY SELECT * FROM project_costs ORDER BY incurred_at DESC;
        ELSE
            RETURN QUERY SELECT * FROM project_costs WHERE archived_at IS NULL ORDER BY incurred_at DESC;
        END IF;
    END IF;
END;
$func$;

-- get_project_cost_summary: now accepts p_project_id
CREATE OR REPLACE FUNCTION get_project_cost_summary(
    p_project_id uuid DEFAULT NULL
) RETURNS jsonb
LANGUAGE plpgsql
AS $func$
DECLARE
    v_total_one_time DECIMAL(10,2);
    v_total_monthly_recurring DECIMAL(10,2);
    v_total_quarterly_recurring DECIMAL(10,2);
    v_total_annual_recurring DECIMAL(10,2);
    v_total_all DECIMAL(10,2);
    v_by_category JSONB;
    v_items JSONB;
BEGIN
    SELECT
        COALESCE(SUM(CASE WHEN frequency = 'one_time' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'monthly' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'quarterly' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'annual' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(amount_usd), 0)
    INTO v_total_one_time, v_total_monthly_recurring, v_total_quarterly_recurring, v_total_annual_recurring, v_total_all
    FROM project_costs
    WHERE archived_at IS NULL
      AND (p_project_id IS NULL OR project_id = p_project_id);

    SELECT COALESCE(jsonb_agg(cat_row), '[]'::jsonb) INTO v_by_category
    FROM (
        SELECT jsonb_build_object(
            'category', category,
            'total', SUM(amount_usd),
            'count', COUNT(*)
        ) AS cat_row
        FROM project_costs
        WHERE archived_at IS NULL
          AND (p_project_id IS NULL OR project_id = p_project_id)
        GROUP BY category
    ) sub;

    SELECT COALESCE(jsonb_agg(row_to_json(r)::jsonb ORDER BY r.incurred_at DESC), '[]'::jsonb)
    INTO v_items
    FROM (SELECT * FROM project_costs WHERE archived_at IS NULL
           AND (p_project_id IS NULL OR project_id = p_project_id)) r;

    RETURN jsonb_build_object(
        'total_one_time', v_total_one_time,
        'total_monthly_recurring', v_total_monthly_recurring,
        'total_quarterly_recurring', v_total_quarterly_recurring,
        'total_annual_recurring', v_total_annual_recurring,
        'total_all', v_total_all,
        'estimated_monthly_burn', v_total_monthly_recurring + (v_total_quarterly_recurring / 3.0) + (v_total_annual_recurring / 12.0),
        'by_category', v_by_category,
        'items', v_items
    );
END;
$func$;
