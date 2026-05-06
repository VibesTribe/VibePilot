-- Migration 140: Upgrade kb_context_pack with doc section semantic search
-- Strategy: find a doc seed embedding (from keyword-matched doc section), then use
-- its embedding to find semantically similar doc sections via cosine similarity.
-- Previously only code symbols used semantic search; docs used pure keyword ILIKE.
-- With embedding backfill now running nightly, all 19k+ doc sections have vectors.
-- This change unlocks semantic doc search for agents.

CREATE OR REPLACE FUNCTION kb_context_pack(
    p_query text,
    p_repo_id text DEFAULT NULL,
    p_limit integer DEFAULT 30,
    p_use_semantic boolean DEFAULT true
)
RETURNS TABLE(section text, content jsonb)
LANGUAGE plpgsql
AS $function$
DECLARE
    v_query_text TEXT;
    v_repo_filter TEXT;
    v_pipeline JSONB;
    v_agents JSONB;
    v_decisions JSONB;
    v_keywords TEXT[];
    v_symbol_seed VECTOR;
    v_doc_seed VECTOR;
    v_semantic_docs JSONB;
BEGIN
    v_query_text := lower(trim(p_query));

    -- Split multi-word query into individual keywords for broader matching
    SELECT array_agg(DISTINCT w) INTO v_keywords
    FROM unnest(string_to_array(v_query_text, ' ')) AS w
    WHERE length(w) > 2;

    -- Find seed embeddings from symbols AND docs
    IF p_use_semantic THEN
        -- Symbol seed (existing): find a matching code symbol, grab its embedding
        SELECT embedding INTO v_symbol_seed
        FROM kb_code_symbols
        WHERE embedding IS NOT NULL
            AND (
                name ILIKE '%' || v_query_text || '%'
                OR qualified_name ILIKE '%' || v_query_text || '%'
                OR summary ILIKE '%' || v_query_text || '%'
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE name ILIKE '%' || kw || '%')
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE summary ILIKE '%' || kw || '%')
            )
        LIMIT 1;

        -- Doc seed (NEW): find a matching doc section, grab its embedding
        -- This enables semantic search across documentation, not just code
        SELECT embedding INTO v_doc_seed
        FROM kb_doc_sections
        WHERE embedding IS NOT NULL
            AND (
                title ILIKE '%' || v_query_text || '%'
                OR summary ILIKE '%' || v_query_text || '%'
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE title ILIKE '%' || kw || '%')
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE summary ILIKE '%' || kw || '%')
            )
        ORDER BY
            CASE
                WHEN title ILIKE '%' || v_query_text || '%' THEN 1
                WHEN summary ILIKE '%' || v_query_text || '%' THEN 2
                WHEN EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE title ILIKE '%' || kw || '%') THEN 3
                ELSE 4
            END,
            length(COALESCE(summary, '')) DESC
        LIMIT 1;
    END IF;

    -- 1. Symbols: semantic search via symbol seed + keyword fallback (unchanged)
    IF v_symbol_seed IS NOT NULL THEN
        RETURN QUERY
        SELECT 'symbols' as section,
            jsonb_agg(row_to_json(sym)::jsonb ORDER BY sym.score DESC) as content
        FROM (
            SELECT * FROM (
                -- Semantic matches
                SELECT s.name, s.qualified_name, s.kind, s.summary, s.file_id, s.line_start,
                    round((1 - (s.embedding <=> v_symbol_seed))::numeric, 2) * 5 as score
                FROM kb_code_symbols s
                WHERE s.embedding IS NOT NULL
                  AND (p_repo_id IS NULL OR s.repo_id = p_repo_id)
                  AND 1 - (s.embedding <=> v_symbol_seed) > 0.45
                  AND s.kind IN ('function', 'method', 'type', 'interface', 'struct', 'constant')
                ORDER BY s.embedding <=> v_symbol_seed
                LIMIT p_limit
            ) sem

            UNION ALL

            -- Keyword fallback
            SELECT s.name, s.qualified_name, s.kind, s.summary, s.file_id, s.line_start,
                CASE
                    WHEN s.name ILIKE '%' || v_query_text || '%' THEN 15
                    WHEN s.qualified_name ILIKE '%' || v_query_text || '%' THEN 10
                    WHEN s.summary ILIKE '%' || v_query_text || '%' THEN 5
                    ELSE 3
                END::numeric as score
            FROM kb_code_symbols s
            WHERE (p_repo_id IS NULL OR s.repo_id = p_repo_id)
                AND (
                    s.name ILIKE '%' || v_query_text || '%'
                    OR s.qualified_name ILIKE '%' || v_query_text || '%'
                    OR s.summary ILIKE '%' || v_query_text || '%'
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE s.name ILIKE '%' || kw || '%')
                )
                AND s.kind IN ('function', 'method', 'type', 'interface', 'struct', 'constant')
                -- Dedup: exclude rows already returned by semantic search
                AND (v_symbol_seed IS NULL OR s.embedding IS NULL OR 1 - (s.embedding <=> v_symbol_seed) <= 0.45)
        ) sym;
    ELSE
        -- Pure keyword matching (no symbol seed found)
        RETURN QUERY
        SELECT 'symbols' as section,
            jsonb_agg(row_to_json(sym)::jsonb ORDER BY sym.score DESC) as content
        FROM (
            SELECT s.name, s.qualified_name, s.kind, s.summary, s.file_id, s.line_start,
                CASE
                    WHEN s.name ILIKE '%' || v_query_text || '%' THEN 3
                    WHEN s.qualified_name ILIKE '%' || v_query_text || '%' THEN 2
                    WHEN s.summary ILIKE '%' || v_query_text || '%' THEN 1
                    ELSE 0
                END::numeric as score
            FROM kb_code_symbols s
            WHERE (p_repo_id IS NULL OR s.repo_id = p_repo_id)
                AND (
                    s.name ILIKE '%' || v_query_text || '%'
                    OR s.qualified_name ILIKE '%' || v_query_text || '%'
                    OR s.summary ILIKE '%' || v_query_text || '%'
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE s.name ILIKE '%' || kw || '%')
                )
            ORDER BY score DESC
            LIMIT p_limit
        ) sym;
    END IF;

    -- 2. Data flow: keyword split matching (unchanged)
    RETURN QUERY
    SELECT 'data_flow' as section,
        jsonb_agg(DISTINCT jsonb_build_object(
            'ui_component', df.ui_component,
            'api_route', df.api_route,
            'handler', df.handler,
            'db_table', df.db_table
        )) as content
    FROM kb_data_flow df
    WHERE df.ui_component ILIKE '%' || v_query_text || '%'
        OR df.api_route ILIKE '%' || v_query_text || '%'
        OR df.handler ILIKE '%' || v_query_text || '%'
        OR df.db_table ILIKE '%' || v_query_text || '%'
        OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE
            df.ui_component ILIKE '%' || kw || '%'
            OR df.api_route ILIKE '%' || kw || '%'
            OR df.handler ILIKE '%' || kw || '%'
            OR df.db_table ILIKE '%' || kw || '%');

    -- 3. Docs: semantic search via doc seed + keyword fallback (UPGRADED)
    IF v_doc_seed IS NOT NULL THEN
        -- Semantic matches via doc seed + keyword fallback (deduped)
        RETURN QUERY
        SELECT 'docs' as section,
            jsonb_agg(row_to_json(d)::jsonb ORDER BY d.score DESC) as content
        FROM (
            -- Semantic matches (wrapped in subquery for LIMIT + UNION compatibility)
            SELECT * FROM (
                SELECT doc.title as title, doc.doc_path as path, left(doc.summary, 300) as summary, doc.repo_id as repo,
                    round((1 - (doc.embedding <=> v_doc_seed))::numeric, 3) * 5 as score
                FROM kb_doc_sections doc
                WHERE doc.embedding IS NOT NULL
                    AND (p_repo_id IS NULL OR doc.repo_id = p_repo_id)
                    AND 1 - (doc.embedding <=> v_doc_seed) > 0.40
                ORDER BY doc.embedding <=> v_doc_seed
                LIMIT 15
            ) doc_sem

            UNION ALL

            -- Keyword fallback (exclude docs already returned by semantic)
            SELECT doc.title, doc.doc_path, left(doc.summary, 300), doc.repo_id,
                CASE
                    WHEN doc.title ILIKE '%' || v_query_text || '%' THEN 3
                    WHEN doc.summary ILIKE '%' || v_query_text || '%' THEN 1
                    WHEN EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE doc.title ILIKE '%' || kw || '%') THEN 2
                    ELSE 1
                END::numeric as score
            FROM kb_doc_sections doc
            WHERE (p_repo_id IS NULL OR doc.repo_id = p_repo_id)
                AND (
                    doc.title ILIKE '%' || v_query_text || '%'
                    OR doc.summary ILIKE '%' || v_query_text || '%'
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE doc.title ILIKE '%' || kw || '%')
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE doc.summary ILIKE '%' || kw || '%')
                )
                -- Dedup: exclude docs already returned by semantic search
                AND (v_doc_seed IS NULL OR doc.embedding IS NULL OR 1 - (doc.embedding <=> v_doc_seed) <= 0.40)
        ) d;
    ELSE
        -- Pure keyword matching (no doc seed found)
        RETURN QUERY
        SELECT 'docs' as section,
            jsonb_agg(jsonb_build_object(
                'title', doc.title,
                'path', doc.doc_path,
                'summary', left(doc.summary, 300),
                'repo', doc.repo_id
            )) as content
        FROM (
            SELECT title, doc_path, summary, repo_id,
                CASE
                    WHEN title ILIKE '%' || v_query_text || '%' THEN 3
                    WHEN summary ILIKE '%' || v_query_text || '%' THEN 1
                    WHEN EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE title ILIKE '%' || kw || '%') THEN 2
                    WHEN EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE summary ILIKE '%' || kw || '%') THEN 1
                    ELSE 0
                END as score
            FROM kb_doc_sections
            WHERE (p_repo_id IS NULL OR repo_id = p_repo_id)
                AND (
                    title ILIKE '%' || v_query_text || '%'
                    OR summary ILIKE '%' || v_query_text || '%'
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE title ILIKE '%' || kw || '%')
                    OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE summary ILIKE '%' || kw || '%')
                )
            ORDER BY score DESC
            LIMIT 10
        ) doc;
    END IF;

    -- 4. Related decisions: keyword split matching (unchanged)
    RETURN QUERY
    SELECT 'decisions' as section,
        jsonb_agg(jsonb_build_object(
            'name', dec.name,
            'title', dec.title,
            'summary', dec.summary,
            'date', dec.metadata->>'date',
            'decision', dec.metadata->>'decision',
            'rejected', dec.metadata->>'rejected'
        )) as content
    FROM kb_knowledge_items dec
    WHERE dec.item_type = 'decision'
        AND (
            dec.title ILIKE '%' || v_query_text || '%'
            OR dec.summary ILIKE '%' || v_query_text || '%'
            OR dec.metadata::text ILIKE '%' || v_query_text || '%'
            OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE
                dec.title ILIKE '%' || kw || '%'
                OR dec.summary ILIKE '%' || kw || '%')
        );

    -- 5. Knowledge items (rules, configs, patterns) - keyword split matching (unchanged)
    RETURN QUERY
    SELECT 'knowledge' as section,
        jsonb_agg(jsonb_build_object(
            'name', ki.name,
            'type', ki.item_type,
            'title', ki.title,
            'summary', left(ki.summary, 200)
        )) as content
    FROM (
        SELECT name, item_type, title, summary
        FROM kb_knowledge_items
        WHERE item_type != 'decision'
            AND (p_repo_id IS NULL OR repo_id = p_repo_id)
            AND (
                title ILIKE '%' || v_query_text || '%'
                OR summary ILIKE '%' || v_query_text || '%'
                OR name ILIKE '%' || v_query_text || '%'
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE
                    title ILIKE '%' || kw || '%'
                    OR name ILIKE '%' || kw || '%')
            )
        LIMIT 10
    ) ki;

    -- 6. Repo map snippet: find files containing matching symbols (unchanged)
    RETURN QUERY
    SELECT 'repo_map_snippet' as section,
        (SELECT jsonb_agg(
            jsonb_build_object(
                'file', f_id,
                'symbols', syms
            )
        )
        FROM (
            SELECT sym2.file_id as f_id,
                jsonb_agg(jsonb_build_object(
                    'name', sym2.name,
                    'kind', sym2.kind,
                    'line', sym2.line_start,
                    'summary', left(sym2.summary, 80)
                ) ORDER BY sym2.line_start) as syms
            FROM kb_code_symbols sym2
            WHERE sym2.file_id IN (
                SELECT DISTINCT s3.file_id
                FROM kb_code_symbols s3
                WHERE (p_repo_id IS NULL OR s3.repo_id = p_repo_id)
                    AND (
                        s3.name ILIKE '%' || v_query_text || '%'
                        OR s3.qualified_name ILIKE '%' || v_query_text || '%'
                        OR s3.summary ILIKE '%' || v_query_text || '%'
                        OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE
                            s3.name ILIKE '%' || kw || '%')
                    )
                LIMIT 5
            )
            AND sym2.kind IN ('function', 'method', 'type', 'interface', 'struct', 'constant')
            GROUP BY sym2.file_id
        ) sub
    );

    -- 7. Topic-specific rules (keyword split, unchanged)
    RETURN QUERY
    SELECT 'rules' as section,
        jsonb_agg(jsonb_build_object(
            'name', r.name,
            'summary', r.summary
        )) as content
    FROM (
        SELECT name, summary
        FROM kb_knowledge_items
        WHERE item_type = 'rule'
            AND (
                name ILIKE '%' || v_query_text || '%'
                OR summary ILIKE '%' || v_query_text || '%'
                OR EXISTS (SELECT 1 FROM unnest(v_keywords) AS kw WHERE
                    name ILIKE '%' || kw || '%'
                    OR summary ILIKE '%' || kw || '%')
            )
        LIMIT 10
    ) r;

    -- 8. Non-negotiable rules (ALWAYS included, unchanged)
    RETURN QUERY
    SELECT 'non_negotiable_rules' as section,
        jsonb_agg(jsonb_build_object(
            'rule', r.name,
            'summary', left(r.summary, 150)
        )) as content
    FROM kb_knowledge_items r
    WHERE r.item_type = 'rule'
        AND (
            r.name ILIKE '%NEVER%'
            OR r.name ILIKE '%ALWAYS%'
            OR r.name ILIKE '%human boundary%'
            OR r.name ILIKE '%guess%'
            OR r.name ILIKE '%hardcode%'
            OR r.name ILIKE '%shortcut%'
            OR r.name ILIKE '%push%'
        );

    -- 9. Project principles (ALWAYS included, unchanged)
    RETURN QUERY
    SELECT 'principles' as section,
        jsonb_agg(jsonb_build_object(
            'title', p.title,
            'summary', left(p.summary, 200)
        )) as content
    FROM (
        SELECT title, summary
        FROM kb_doc_sections
        WHERE (title ILIKE '%principle%' OR title ILIKE '%key principle%')
            AND summary IS NOT NULL AND summary != ''
        ORDER BY length(summary) DESC
        LIMIT 5
    ) p;

    -- 10. System overview (ALWAYS included, unchanged)
    v_pipeline := (
        SELECT jsonb_agg(jsonb_build_object(
            'stage', split_part(name, '_', 3)::int,
            'description', summary
        ) ORDER BY split_part(name, '_', 3)::int)
        FROM kb_knowledge_items
        WHERE item_type = 'pipeline_stage'
            AND name ~ 'pipeline_stages_\\d+'
    );

    v_agents := (
        SELECT jsonb_agg(jsonb_build_object(
            'agent', replace(name, 'prompt:', ''),
            'description', COALESCE(summary, title)
        ) ORDER BY name)
        FROM kb_knowledge_items
        WHERE item_type = 'prompt'
            AND name LIKE 'prompt:%'
            AND name NOT LIKE 'prompt:%simple%'
            AND name NOT LIKE 'prompt:test%'
    );

    v_decisions := (
        SELECT jsonb_agg(jsonb_build_object(
            'id', name,
            'decision', COALESCE(metadata->>'decision', summary)
        ) ORDER BY name)
        FROM (
            SELECT name, metadata, summary
            FROM kb_knowledge_items
            WHERE item_type = 'decision'
            ORDER BY name
            LIMIT 5
        ) sub
    );

    IF v_pipeline IS NOT NULL OR v_agents IS NOT NULL THEN
        RETURN QUERY
        SELECT 'system_overview' as section,
            jsonb_build_object(
                'pipeline_stages', COALESCE(v_pipeline, '[]'::jsonb),
                'agents', COALESCE(v_agents, '[]'::jsonb),
                'key_decisions', COALESCE(v_decisions, '[]'::jsonb),
                'data_flow_paths', (SELECT count(*)::int FROM kb_data_flow),
                'total_symbols', (SELECT count(*)::int FROM kb_code_symbols),
                'total_files', (SELECT count(*)::int FROM kb_files)
            ) as content;
    END IF;

    RETURN;
END;
$function$;
