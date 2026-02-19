-- Fix: Clean up dependency data after migration
-- The to_jsonb() added extra quotes to UUIDs
-- This fixes existing data

BEGIN;

-- Fix dependencies that have double-quoted UUIDs
-- Convert ["\"uuid\""] → ["uuid"]
UPDATE tasks
SET dependencies = (
    SELECT jsonb_agg(
        CASE 
            WHEN jsonb_typeof(dep) = 'string' AND dep #>> '{}' LIKE '"%' 
            THEN (dep #>> '{}')::jsonb  -- Unwrap the double-quoted string
            ELSE dep
        END
    )
    FROM jsonb_array_elements(dependencies) AS dep
)
WHERE dependencies IS NOT NULL 
AND jsonb_array_length(dependencies) > 0
AND EXISTS (
    SELECT 1 FROM jsonb_array_elements(dependencies) AS dep
    WHERE jsonb_typeof(dep) = 'string' AND dep #>> '{}' LIKE '"%'
);

-- Verify fix
SELECT id, title, dependencies 
FROM tasks 
WHERE dependencies IS NOT NULL 
AND jsonb_array_length(dependencies) > 0
LIMIT 5;

COMMIT;
