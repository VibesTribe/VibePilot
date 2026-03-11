-- Migration 085: Cleanup for Fresh Test
-- Purpose: Clear all tasks/plans/runs for a clean test
-- Date: 2026-03-11
-- 
-- Run this in Supabase SQL Editor before testing

DELETE FROM task_runs;
DELETE FROM task_packets;
DELETE FROM tasks;
DELETE FROM plans;
UPDATE maintenance_commands SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;
UPDATE research_suggestions SET processing_by = NULL, processing_at = NULL WHERE processing_by IS NOT NULL;

SELECT 'Cleanup complete' as status;
