# Task Execution Result Fix

**Date:** 2026-04-01  
**Issue:** task_runs.result was NULL, supervisor had nothing to review

## Root Cause

The `task_runs.result` column exists in schema but `create_task_run` function never wrote to it.

## Fix Applied

### 1. Supabase Function
**File:** `docs/supabase-schema/fix_task_run_result.sql`
- Run in Supabase SQL Editor
- Updates `create_task_run` to accept and store `p_result`

### 2. Go Code  
**File:** `governor/cmd/governor/handlers_task.go:227-258`
- Captures execution result (files, summary, raw_output)
- Passes to `create_task_run` as `p_result`

## Deployment

1. **Apply Supabase fix:** Run the SQL file
2. **Restart governor:** `sudo systemctl restart vibepilot-governor`
