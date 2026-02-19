-- Direct fix: Strip quotes from dependency UUIDs
-- Run this in Supabase SQL Editor

BEGIN;

-- First, let's see what we're working with
SELECT id, title, dependencies 
FROM tasks 
WHERE dependencies IS NOT NULL 
AND jsonb_array_length(dependencies) > 0
LIMIT 3;

-- Fix: Convert double-quoted strings to plain strings
-- From ["\"uuid\""] to ["uuid"]
UPDATE tasks
SET dependencies = (
    SELECT jsonb_agg(
        trim(BOTH '"' FROM dep #>> '{}')
    )
    FROM jsonb_array_elements(dependencies) AS dep
)
WHERE dependencies IS NOT NULL 
AND jsonb_array_length(dependencies) > 0;

-- Verify fix
SELECT id, title, dependencies 
FROM tasks 
WHERE dependencies IS NOT NULL 
AND jsonb_array_length(dependencies) > 0
LIMIT 3;

COMMIT;
