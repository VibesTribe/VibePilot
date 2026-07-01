-- ============================================================================
-- Migration 052: PIF Phase G — RPC functions for per-project systems
-- ============================================================================

-- 1. KANBAN CRUD functions
CREATE OR REPLACE FUNCTION create_todo(
    p_project_id UUID,
    p_title TEXT,
    p_description TEXT DEFAULT '',
    p_status TEXT DEFAULT 'todo',
    p_priority TEXT DEFAULT 'medium',
    p_category TEXT DEFAULT 'general',
    p_source TEXT DEFAULT 'manual',
    p_sort_order INTEGER DEFAULT 0
) RETURNS UUID AS $$
DECLARE
    new_id UUID;
BEGIN
    INSERT INTO project_todos (project_id, title, description, status, priority, category, source, sort_order)
    VALUES (p_project_id, p_title, p_description, p_status, p_priority, p_category, p_source, p_sort_order)
    RETURNING id INTO new_id;
    RETURN new_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_todo(
    p_id UUID,
    p_title TEXT DEFAULT NULL,
    p_description TEXT DEFAULT NULL,
    p_status TEXT DEFAULT NULL,
    p_priority TEXT DEFAULT NULL,
    p_category TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    UPDATE project_todos SET
        title = COALESCE(p_title, title),
        description = COALESCE(p_description, description),
        status = COALESCE(p_status, status),
        priority = COALESCE(p_priority, priority),
        category = COALESCE(p_category, category),
        updated_at = NOW(),
        completed_at = CASE WHEN p_status = 'done' THEN NOW() ELSE completed_at END
    WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_todo(p_id UUID) RETURNS VOID AS $$
BEGIN
    DELETE FROM project_todos WHERE id = p_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reorder_todos(
    p_project_id UUID,
    p_item_ids UUID[]
) RETURNS VOID AS $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..array_length(p_item_ids, 1) LOOP
        UPDATE project_todos SET sort_order = i, updated_at = NOW()
        WHERE id = p_item_ids[i] AND project_id = p_project_id;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- 2. CODE GRAPH upsert function
CREATE OR REPLACE FUNCTION upsert_code_graph(
    p_project_id UUID,
    p_graph_data JSONB,
    p_node_count INTEGER DEFAULT 0,
    p_edge_count INTEGER DEFAULT 0,
    p_source_path TEXT DEFAULT ''
) RETURNS UUID AS $$
DECLARE
    graph_id UUID;
BEGIN
    INSERT INTO code_graph_snapshots (project_id, graph_data, node_count, edge_count, source_path, generated_at)
    VALUES (p_project_id, p_graph_data, p_node_count, p_edge_count, p_source_path, NOW())
    ON CONFLICT (project_id)
    DO UPDATE SET
        graph_data = EXCLUDED.graph_data,
        node_count = EXCLUDED.node_count,
        edge_count = EXCLUDED.edge_count,
        source_path = EXCLUDED.source_path,
        generated_at = NOW()
    RETURNING id INTO graph_id;
    RETURN graph_id;
END;
$$ LANGUAGE plpgsql;

-- Done
DO $$ BEGIN
    RAISE NOTICE 'Migration 052 complete: RPC functions created for per-project kanban and code graph.';
END $$;
