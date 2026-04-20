-- Connector usage table (new): persists connector-level aggregated usage windows
-- so state survives governor restarts.
CREATE TABLE IF NOT EXISTS connector_usage (
    connector_id TEXT PRIMARY KEY,
    usage_windows TEXT,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Platform usage windows column: add to existing platforms table
-- to persist per-platform usage windows (3h, 8h, day, session).
ALTER TABLE platforms ADD COLUMN IF NOT EXISTS usage_windows TEXT;

-- RPC: upsert connector usage windows
CREATE OR REPLACE FUNCTION upsert_connector_usage(
    p_connector_id TEXT,
    p_usage_windows TEXT
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO connector_usage (connector_id, usage_windows, updated_at)
    VALUES (p_connector_id, p_usage_windows, now())
    ON CONFLICT (connector_id)
    DO UPDATE SET usage_windows = p_usage_windows, updated_at = now();
END;
$$;

-- RPC: update platform usage windows
CREATE OR REPLACE FUNCTION update_platform_usage(
    p_platform_id TEXT,
    p_usage_windows TEXT
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE platforms
    SET usage_windows = p_usage_windows
    WHERE id::TEXT = p_platform_id OR name = p_platform_id;
END;
$$;
