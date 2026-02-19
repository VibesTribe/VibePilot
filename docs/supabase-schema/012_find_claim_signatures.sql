-- First: See what functions exist
SELECT 
    p.proname AS name,
    pg_get_function_identity_arguments(p.oid) AS args
FROM pg_proc p
JOIN pg_namespace n ON p.pronamespace = n.oid
WHERE n.nspname = 'public'
AND p.proname = 'claim_next_task';

-- Then run the appropriate DROP based on what you see above
-- Example:
-- DROP FUNCTION IF EXISTS public.claim_next_task(text, text, text);
-- DROP FUNCTION IF EXISTS public.claim_next_task(character varying, character varying, character varying);
-- etc.
