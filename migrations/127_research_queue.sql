CREATE TABLE IF NOT EXISTS research_queue (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    url TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    note TEXT DEFAULT '',
    source TEXT DEFAULT 'bookmarklet',
    processed_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_research_queue_unprocessed ON research_queue (created_at) WHERE processed_at IS NULL;

ALTER TABLE research_queue ENABLE ROW LEVEL SECURITY;

CREATE POLICY research_queue_service_all ON research_queue FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY research_queue_anon_insert ON research_queue FOR INSERT WITH CHECK (true);
CREATE POLICY research_queue_anon_select ON research_queue FOR SELECT USING (true);

CREATE OR REPLACE FUNCTION get_unprocessed_bookmarks(p_limit INT DEFAULT 50)
RETURNS TABLE(id UUID, url TEXT, title TEXT, note TEXT, source TEXT, created_at TIMESTAMPTZ)
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    RETURN QUERY
    SELECT rq.id, rq.url, rq.title, rq.note, rq.source, rq.created_at
    FROM research_queue rq
    WHERE rq.processed_at IS NULL
    ORDER BY rq.created_at ASC
    LIMIT p_limit;
END;
$$;

CREATE OR REPLACE FUNCTION mark_bookmark_processed(p_id UUID)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    UPDATE research_queue SET processed_at = now() WHERE id = p_id;
END;
$$;

CREATE OR REPLACE FUNCTION add_bookmark(p_url TEXT, p_title TEXT DEFAULT '', p_note TEXT DEFAULT '', p_source TEXT DEFAULT 'bookmarklet')
RETURNS UUID
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_id UUID;
BEGIN
    INSERT INTO research_queue (url, title, note, source)
    VALUES (p_url, p_title, p_note, p_source)
    RETURNING id INTO v_id;
    RETURN v_id;
END;
$$;

GRANT EXECUTE ON FUNCTION get_unprocessed_bookmarks(INT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION mark_bookmark_processed(UUID) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION add_bookmark(TEXT, TEXT, TEXT, TEXT) TO anon, authenticated, service_role;
