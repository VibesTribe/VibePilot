-- Migration 141: Contradiction Tracking Tables
-- Phase 3 from strategic plan v2
-- Tracks factual claims across docs, prompts, skills, and configs
-- Detects conflicts, enables human review, builds canonical truth

-- ============================================================
-- 1. kb_claims: Every factual statement extracted from sources
-- ============================================================
CREATE TABLE IF NOT EXISTS kb_claims (
    id SERIAL PRIMARY KEY,
    subject TEXT NOT NULL,              -- normalized subject (what claim is about)
    claim_text TEXT NOT NULL,           -- the factual statement
    source_type TEXT NOT NULL,          -- 'doc_section', 'skill', 'prompt', 'rule', 'config'
    source_id TEXT NOT NULL,            -- kb_doc_sections.id or kb_skills.name etc
    source_path TEXT NOT NULL,          -- human-readable path for display
    content_hash TEXT,                  -- source content hash at extraction time
    confidence TEXT DEFAULT 'medium' CHECK (confidence IN ('high', 'medium', 'low')),
    is_active BOOLEAN DEFAULT true,    -- false if superseded by re-extraction
    extracted_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(subject, claim_text, source_id)
);

CREATE INDEX IF NOT EXISTS idx_kb_claims_subject ON kb_claims(subject);
CREATE INDEX IF NOT EXISTS idx_kb_claims_source ON kb_claims(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_kb_claims_active ON kb_claims(is_active);

-- ============================================================
-- 2. kb_contradictions: Pairs of conflicting claims
-- ============================================================
CREATE TABLE IF NOT EXISTS kb_contradictions (
    id SERIAL PRIMARY KEY,
    claim_a_id INTEGER NOT NULL REFERENCES kb_claims(id) ON DELETE CASCADE,
    claim_b_id INTEGER NOT NULL REFERENCES kb_claims(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    conflict_type TEXT NOT NULL CHECK (conflict_type IN ('factual', 'temporal', 'preference')),
    status TEXT DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    detected_at TIMESTAMPTZ DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    notes TEXT,                        -- context about why they conflict
    UNIQUE(claim_a_id, claim_b_id)
);

CREATE INDEX IF NOT EXISTS idx_kb_contradictions_status ON kb_contradictions(status);
CREATE INDEX IF NOT EXISTS idx_kb_contradictions_subject ON kb_contradictions(subject);

-- Prevent self-contradiction
CREATE OR REPLACE FUNCTION check_no_self_contradiction()
RETURNS trigger AS $$
BEGIN
    IF NEW.claim_a_id = NEW.claim_b_id THEN
        RAISE EXCEPTION 'A claim cannot contradict itself (claim_a_id = claim_b_id = %)', NEW.claim_a_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS no_self_contradiction ON kb_contradictions;
CREATE TRIGGER no_self_contradiction
    BEFORE INSERT OR UPDATE ON kb_contradictions
    FOR EACH ROW EXECUTE FUNCTION check_no_self_contradiction();

-- ============================================================
-- 3. kb_canon: Single source of truth after human review
-- ============================================================
CREATE TABLE IF NOT EXISTS kb_canon (
    id SERIAL PRIMARY KEY,
    subject TEXT NOT NULL UNIQUE,       -- topic (matches kb_claims.subject)
    canonical_text TEXT NOT NULL,       -- the human-approved truth
    supersedes_id INTEGER REFERENCES kb_canon(id),  -- previous version of this truth
    superseded_at TIMESTAMPTZ,
    sources JSONB DEFAULT '[]'::jsonb, -- files that are verified to match this truth
    human_reviewed BOOLEAN DEFAULT false,
    reviewed_by TEXT,                   -- who reviewed it
    reviewed_at TIMESTAMPTZ,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kb_canon_subject ON kb_canon(subject);
CREATE INDEX IF NOT EXISTS idx_kb_canon_reviewed ON kb_canon(human_reviewed);

-- ============================================================
-- 4. RPC: Get open contradictions for dashboard display
-- ============================================================
CREATE OR REPLACE FUNCTION kb_get_open_contradictions(
    p_limit INTEGER DEFAULT 50,
    p_subject TEXT DEFAULT NULL
)
RETURNS TABLE(
    id INTEGER,
    subject TEXT,
    conflict_type TEXT,
    claim_a_id INTEGER,
    claim_a_text TEXT,
    claim_a_source TEXT,
    claim_b_id INTEGER,
    claim_b_text TEXT,
    claim_b_source TEXT,
    detected_at TIMESTAMPTZ,
    age_hours NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.subject,
        c.conflict_type,
        c.claim_a_id,
        ca.claim_text,
        ca.source_path || ' (' || ca.source_type || ')',
        c.claim_b_id,
        cb.claim_text,
        cb.source_path || ' (' || cb.source_type || ')',
        c.detected_at,
        ROUND(EXTRACT(EPOCH FROM (now() - c.detected_at)) / 3600, 1)
    FROM kb_contradictions c
    JOIN kb_claims ca ON ca.id = c.claim_a_id AND ca.is_active = true
    JOIN kb_claims cb ON cb.id = c.claim_b_id AND cb.is_active = true
    WHERE c.status = 'open'
        AND (p_subject IS NULL OR c.subject ILIKE '%' || p_subject || '%')
    ORDER BY c.detected_at DESC
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- 5. RPC: Get canon for all subjects
-- ============================================================
CREATE OR REPLACE FUNCTION kb_get_canon(
    p_subject TEXT DEFAULT NULL
)
RETURNS TABLE(
    subject TEXT,
    canonical_text TEXT,
    version INTEGER,
    human_reviewed BOOLEAN,
    reviewed_at TIMESTAMPTZ,
    supersedes_subject TEXT,
    source_count INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        kc.subject,
        kc.canonical_text,
        kc.version,
        kc.human_reviewed,
        kc.reviewed_at,
        sup.subject AS supersedes_subject,
        jsonb_array_length(kc.sources) AS source_count
    FROM kb_canon kc
    LEFT JOIN kb_canon sup ON sup.id = kc.supersedes_id
    WHERE p_subject IS NULL OR kc.subject ILIKE '%' || p_subject || '%'
    ORDER BY kc.subject;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- 6. RPC: Resolve a contradiction (create canon entry)
-- ============================================================
CREATE OR REPLACE FUNCTION kb_resolve_contradiction(
    p_contradiction_id INTEGER,
    p_canonical_text TEXT,
    p_reviewed_by TEXT DEFAULT 'human',
    p_notes TEXT DEFAULT NULL
)
RETURNS INTEGER AS $$
DECLARE
    v_subject TEXT;
    v_subject_exists INTEGER;
    v_old_canon_id INTEGER;
    v_new_canon_id INTEGER;
BEGIN
    -- Get the contradiction's subject
    SELECT c.subject INTO v_subject
    FROM kb_contradictions c
    WHERE c.id = p_contradiction_id;

    IF v_subject IS NULL THEN
        RAISE EXCEPTION 'Contradiction % not found', p_contradiction_id;
    END IF;

    -- Check if canon already exists for this subject
    SELECT id, version INTO v_old_canon_id, v_subject_exists
    FROM kb_canon WHERE subject = v_subject;

    IF v_subject_exists IS NOT NULL THEN
        -- Update existing canon as superseded
        UPDATE kb_canon
        SET superseded_at = now()
        WHERE id = v_old_canon_id;
    END IF;

    -- Insert new canon entry
    INSERT INTO kb_canon (subject, canonical_text, supersedes_id, human_reviewed, reviewed_by, reviewed_at, version)
    VALUES (v_subject, p_canonical_text, v_old_canon_id, true, p_reviewed_by, now(), COALESCE(v_subject_exists, 0) + 1)
    RETURNING id INTO v_new_canon_id;

    -- Mark contradiction as resolved
    UPDATE kb_contradictions
    SET status = 'resolved',
        resolved_at = now(),
        notes = COALESCE(p_notes, notes)
    WHERE id = p_contradiction_id;

    -- Also resolve any other open contradictions on the same subject
    UPDATE kb_contradictions
    SET status = 'resolved',
        resolved_at = now(),
        notes = 'Auto-resolved: canon set for subject'
    WHERE subject = v_subject
        AND status = 'open'
        AND id != p_contradiction_id;

    RETURN v_new_canon_id;
END;
$$ LANGUAGE plpgsql;
