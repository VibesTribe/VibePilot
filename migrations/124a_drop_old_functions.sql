
-- Migration 124 PART 1: Drop old functions
-- Run this FIRST in Supabase SQL Editor
DROP FUNCTION IF EXISTS check_platform_availability(TEXT);
DROP FUNCTION IF EXISTS get_model_score_for_task(TEXT, TEXT, TEXT);
DROP FUNCTION IF EXISTS update_model_usage(TEXT, JSONB, TIMESTAMPTZ, TIMESTAMPTZ, INTEGER, TEXT, TEXT, INTEGER, JSONB);
