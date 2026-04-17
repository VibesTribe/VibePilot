-- Migration 121b: Make task_runs.platform and model_id nullable
-- create_task_run RPC defaults these to NULL but table constraints block inserts.
ALTER TABLE task_runs ALTER COLUMN platform DROP NOT NULL;
ALTER TABLE task_runs ALTER COLUMN model_id DROP NOT NULL;
