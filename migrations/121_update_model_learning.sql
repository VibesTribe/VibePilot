-- Migration 121: update_model_learning RPC
-- Builds institutional knowledge about model competency per task type.
-- Every task success/failure feeds into models.learned JSONB column.
-- Over time, the router uses best_for_task_types / avoid_for_task_types for intelligent routing.

CREATE OR REPLACE FUNCTION update_model_learning(
  p_model_id TEXT,
  p_task_type TEXT,
  p_outcome TEXT DEFAULT 'success',
  p_failure_class TEXT DEFAULT NULL,
  p_failure_category TEXT DEFAULT NULL,
  p_failure_detail TEXT DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
  v_learned JSONB;
  v_type_key TEXT;
  v_current_rate JSONB;
  v_new_rate NUMERIC;
  v_attempts INT;
  v_successes INT;
BEGIN
  -- Get current learned data
  SELECT learned INTO v_learned FROM models WHERE id = p_model_id;
  IF v_learned IS NULL THEN
    v_learned := '{}'::JSONB;
  END IF;

  -- Initialize failure_rate_by_type if missing
  IF v_learned->'failure_rate_by_type' IS NULL THEN
    v_learned := jsonb_set(v_learned, '{failure_rate_by_type}', '{}'::JSONB);
  END IF;

  v_type_key := COALESCE(p_task_type, 'unknown');

  -- Get current stats for this task type
  v_current_rate := v_learned->'failure_rate_by_type'->v_type_key;
  IF v_current_rate IS NULL THEN
    v_current_rate := '{"attempts": 0, "successes": 0, "failures": 0, "rate": 0}'::JSONB;
  END IF;

  v_attempts := COALESCE((v_current_rate->>'attempts')::INT, 0) + 1;

  IF p_outcome = 'success' THEN
    v_successes := COALESCE((v_current_rate->>'successes')::INT, 0) + 1;
    v_new_rate := ROUND((v_successes::NUMERIC / v_attempts::NUMERIC) * 100, 1);
    
    -- Update failure_rate_by_type
    v_learned := jsonb_set(v_learned, 
      ARRAY['failure_rate_by_type', v_type_key],
      jsonb_build_object(
        'attempts', v_attempts,
        'successes', v_successes,
        'failures', COALESCE((v_current_rate->>'failures')::INT, 0),
        'rate', v_new_rate
      )
    );

    -- If success rate > 80% with 3+ attempts, add to best_for
    IF v_new_rate >= 80 AND v_attempts >= 3 THEN
      IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(v_learned->'best_for_task_types') elem WHERE elem = v_type_key) THEN
        IF v_learned->'best_for_task_types' IS NULL THEN
          v_learned := jsonb_set(v_learned, '{best_for_task_types}', jsonb_build_array(v_type_key));
        ELSE
          v_learned := jsonb_set(v_learned, '{best_for_task_types}', (v_learned->'best_for_task_types') || to_jsonb(v_type_key));
        END IF;
      END IF;
      -- Remove from avoid if it was there
      IF v_learned->'avoid_for_task_types' IS NOT NULL THEN
        v_learned := jsonb_set(v_learned, '{avoid_for_task_types}',
          (SELECT jsonb_agg(elem) FROM jsonb_array_elements_text(v_learned->'avoid_for_task_types') elem WHERE elem != v_type_key)
        );
      END IF;
    END IF;

  ELSE
    -- Failure
    v_successes := COALESCE((v_current_rate->>'successes')::INT, 0);
    v_new_rate := ROUND((v_successes::NUMERIC / v_attempts::NUMERIC) * 100, 1);

    v_learned := jsonb_set(v_learned,
      ARRAY['failure_rate_by_type', v_type_key],
      jsonb_build_object(
        'attempts', v_attempts,
        'successes', v_successes,
        'failures', COALESCE((v_current_rate->>'failures')::INT, 0) + 1,
        'rate', v_new_rate,
        'last_failure_class', p_failure_class,
        'last_failure_detail', p_failure_detail
      )
    );

    -- If success rate < 40% with 3+ attempts, add to avoid list
    IF v_new_rate < 40 AND v_attempts >= 3 THEN
      IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(v_learned->'avoid_for_task_types') elem WHERE elem = v_type_key) THEN
        IF v_learned->'avoid_for_task_types' IS NULL THEN
          v_learned := jsonb_set(v_learned, '{avoid_for_task_types}', jsonb_build_array(v_type_key));
        ELSE
          v_learned := jsonb_set(v_learned, '{avoid_for_task_types}', (v_learned->'avoid_for_task_types') || to_jsonb(v_type_key));
        END IF;
      END IF;
      -- Remove from best_for if it was there
      IF v_learned->'best_for_task_types' IS NOT NULL THEN
        v_learned := jsonb_set(v_learned, '{best_for_task_types}',
          (SELECT jsonb_agg(elem) FROM jsonb_array_elements_text(v_learned->'best_for_task_types') elem WHERE elem != v_type_key)
        );
      END IF;
    END IF;
  END IF;

  -- Update the model row
  UPDATE models SET learned = v_learned, updated_at = NOW() WHERE id = p_model_id;
  RETURN FOUND;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO authenticated;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO anon;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO service_role;
