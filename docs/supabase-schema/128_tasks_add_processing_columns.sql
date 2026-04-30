-- Migration 128: Add processing_by and processing_at to tasks table
-- These columns exist on plans but were missing from tasks after Supabase migration.
-- The set_processing/clear_processing RPCs use these for distributed locking.
-- Recovery, realtime, and pgnotify all read processing_by to skip locked rows.

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS processing_by text,
  ADD COLUMN IF NOT EXISTS processing_at timestamptz;

-- Update the vp_notify_change trigger function to include processing_by in the payload
-- so pgnotify listener can check it (it already reads it, we just need the column to exist)
