-- SAFETY PATCHES FOR VIBESPILOT CORE SCHEMA

-- 1. Prevent circular dependencies (no task can depend on itself)
ALTER TABLE tasks ADD CONSTRAINT no_self_dependency 
  CHECK (NOT (id = ANY(dependencies)));

-- 2. Reduce max attempts to 3 (stop error loops early)
ALTER TABLE tasks ALTER COLUMN max_attempts SET DEFAULT 3;

-- 3. Add 'escalated' status for tasks that need attention after 3 failures
ALTER TABLE tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'escalated'
  ));

-- 4. FUNCTION: Escalate task after max attempts
CREATE OR REPLACE FUNCTION check_task_escalation(p_task_id UUID)
RETURNS TEXT AS $$
DECLARE
  v_attempts INT;
  v_max_attempts INT;
BEGIN
  SELECT attempts, max_attempts INTO v_attempts, v_max_attempts
  FROM tasks WHERE id = p_task_id;
  
  IF v_attempts >= v_max_attempts THEN
    UPDATE tasks 
    SET status = 'escalated',
        updated_at = NOW()
    WHERE id = p_task_id;
    RETURN 'escalated';
  END IF;
  
  RETURN 'ok';
END;
$$ LANGUAGE plpgsql;

-- 5. FUNCTION: Deep circular dependency check
CREATE OR REPLACE FUNCTION check_circular_deps(p_task_id UUID, p_deps UUID[])
RETURNS BOOLEAN AS $$
DECLARE
  dep UUID;
  sub_deps UUID[];
BEGIN
  IF p_task_id = ANY(p_deps) THEN
    RETURN FALSE;
  END IF;
  
  FOR dep IN SELECT unnest(p_deps) LOOP
    SELECT dependencies INTO sub_deps FROM tasks WHERE id = dep;
    IF NOT check_circular_deps(p_task_id, sub_deps) THEN
      RETURN FALSE;
    END IF;
  END LOOP;
  
  RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- 6. Update existing tasks to max_attempts = 3
UPDATE tasks SET max_attempts = 3 WHERE max_attempts IS NULL OR max_attempts > 3;

-- 7. Add task_type constraint for future UI/UX flagging
ALTER TABLE tasks ADD CONSTRAINT task_type_check
  CHECK (type IN (
    'setup', 'feature', 'bugfix', 'test', 'docs', 
    'refactor', 'ui_ux', 'api', 'infrastructure'
  ));

SELECT 'Safety patches applied. Max attempts: 3. Escalation enabled.' AS status;
