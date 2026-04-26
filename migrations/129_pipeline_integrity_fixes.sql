-- Migration 129: Pipeline integrity fixes
-- 1. Unique constraint to prevent duplicate tasks per plan
ALTER TABLE tasks ADD CONSTRAINT tasks_plan_task_unique UNIQUE (plan_id, task_number);
