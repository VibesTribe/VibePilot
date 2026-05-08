-- 132: Project costs table for ROI overhead tracking
-- Stores real-world expenses (subscriptions, cloud, utilities, etc.)
-- to calculate true ROI = token savings minus actual project costs

CREATE TABLE IF NOT EXISTS project_costs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category TEXT NOT NULL DEFAULT 'other',
    description TEXT NOT NULL,
    amount_usd DECIMAL(10,2) NOT NULL,
    frequency TEXT NOT NULL DEFAULT 'one_time'
        CHECK (frequency IN ('one_time', 'monthly', 'quarterly', 'annual')),
    incurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_project_costs_category ON project_costs(category);
CREATE INDEX IF NOT EXISTS idx_project_costs_archived ON project_costs(archived_at) WHERE archived_at IS NOT NULL;

-- RPC: List all active project costs
CREATE OR REPLACE FUNCTION list_project_costs(
    p_include_archived BOOLEAN DEFAULT FALSE
)
RETURNS SETOF project_costs AS $$
BEGIN
    IF p_include_archived THEN
        RETURN QUERY SELECT * FROM project_costs ORDER BY incurred_at DESC;
    ELSE
        RETURN QUERY SELECT * FROM project_costs WHERE archived_at IS NULL ORDER BY incurred_at DESC;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- RPC: Add a project cost
CREATE OR REPLACE FUNCTION add_project_cost(
    p_category TEXT,
    p_description TEXT,
    p_amount_usd DECIMAL(10,2),
    p_frequency TEXT DEFAULT 'one_time',
    p_incurred_at TIMESTAMPTZ DEFAULT NOW()
)
RETURNS project_costs AS $$
DECLARE
    v_row project_costs;
BEGIN
    INSERT INTO project_costs (category, description, amount_usd, frequency, incurred_at)
    VALUES (p_category, p_description, p_amount_usd, p_frequency, p_incurred_at)
    RETURNING * INTO v_row;
    RETURN v_row;
END;
$$ LANGUAGE plpgsql;

-- RPC: Update a project cost
CREATE OR REPLACE FUNCTION update_project_cost(
    p_id UUID,
    p_category TEXT DEFAULT NULL,
    p_description TEXT DEFAULT NULL,
    p_amount_usd DECIMAL(10,2) DEFAULT NULL,
    p_frequency TEXT DEFAULT NULL
)
RETURNS project_costs AS $$
DECLARE
    v_row project_costs;
BEGIN
    UPDATE project_costs SET
        category = COALESCE(p_category, category),
        description = COALESCE(p_description, description),
        amount_usd = COALESCE(p_amount_usd, amount_usd),
        frequency = COALESCE(p_frequency, frequency),
        created_at = created_at  -- unchanged
    WHERE id = p_id AND archived_at IS NULL
    RETURNING * INTO v_row;
    
    RETURN v_row;
END;
$$ LANGUAGE plpgsql;

-- RPC: Archive (soft-delete) a project cost
CREATE OR REPLACE FUNCTION archive_project_cost(
    p_id UUID
)
RETURNS BOOLEAN AS $$
BEGIN
    UPDATE project_costs SET archived_at = NOW() WHERE id = p_id AND archived_at IS NULL;
    RETURN FOUND;
END;
$$ LANGUAGE plpgsql;

-- RPC: Get aggregated project cost summary
CREATE OR REPLACE FUNCTION get_project_cost_summary()
RETURNS JSONB AS $$
DECLARE
    v_total_one_time DECIMAL(10,2);
    v_total_monthly_recurring DECIMAL(10,2);
    v_total_quarterly_recurring DECIMAL(10,2);
    v_total_annual_recurring DECIMAL(10,2);
    v_total_all DECIMAL(10,2);
    v_by_category JSONB;
    v_items JSONB;
BEGIN
    -- Sum by frequency
    SELECT 
        COALESCE(SUM(CASE WHEN frequency = 'one_time' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'monthly' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'quarterly' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN frequency = 'annual' THEN amount_usd ELSE 0 END), 0),
        COALESCE(SUM(amount_usd), 0)
    INTO v_total_one_time, v_total_monthly_recurring, v_total_quarterly_recurring, v_total_annual_recurring, v_total_all
    FROM project_costs WHERE archived_at IS NULL;

    -- By category
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'category', category,
        'total', SUM(amount_usd),
        'count', COUNT(*)
    )), '[]'::jsonb)
    INTO v_by_category
    FROM project_costs
    WHERE archived_at IS NULL
    GROUP BY category;

    -- All items for display
    SELECT COALESCE(jsonb_agg(row_to_json(r)::jsonb ORDER BY r.incurred_at DESC), '[]'::jsonb)
    INTO v_items
    FROM (SELECT * FROM project_costs WHERE archived_at IS NULL) r;

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
$$ LANGUAGE plpgsql;

-- Seed historical costs from what we know
INSERT INTO project_costs (category, description, amount_usd, frequency, incurred_at) VALUES
    ('subscription', 'Z.AI Pro (GLM-5) - Feb-Apr 2026 quarterly subscription', 45.00, 'quarterly', '2026-02-01'),
    ('subscription', 'DeepSeek API credits (Mar 2026)', 10.00, 'one_time', '2026-03-01'),
    ('subscription', 'Anthropic API credits (Mar 2026)', 10.00, 'one_time', '2026-03-01'),
    ('subscription', 'OpenRouter credits (Mar 2026)', 20.00, 'one_time', '2026-03-01'),
    ('subscription', 'OpenAI ChatGPT Plus (Jan-Feb 2026)', 40.00, 'one_time', '2026-01-01'),
    ('subscription', 'Kimi monthly subscription (Apr 2026)', 8.00, 'one_time', '2026-04-01'),
    ('cloud', 'GCE compute - Month 1 (Feb-Mar 2026)', 68.00, 'one_time', '2026-02-15'),
    ('cloud', 'GCE compute - Month 2 (Mar-Apr 2026)', 58.00, 'one_time', '2026-03-15'),
    ('cloud', 'GCE unexpected charge (Apr 2026)', 2.00, 'one_time', '2026-04-20');
