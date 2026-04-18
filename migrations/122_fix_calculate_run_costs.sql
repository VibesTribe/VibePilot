-- Migration 122 (v6): Give up fighting overloads, use a clean new name
-- The old calculate_run_costs has stuck overloads that won't drop.
-- Go code will be updated to call calc_run_costs instead.

-- Kill everything we can
DROP FUNCTION IF EXISTS calculate_run_costs(UUID) CASCADE;
DROP FUNCTION IF EXISTS calculate_run_costs(TEXT, INT, INT, NUMERIC) CASCADE;
DROP FUNCTION IF EXISTS calculate_run_costs(TEXT, INT, INT, DOUBLE PRECISION) CASCADE;

-- Create with clean name - no overloads possible
CREATE OR REPLACE FUNCTION calc_run_costs(
    p_model_id TEXT,
    p_tokens_in INT DEFAULT 0,
    p_tokens_out INT DEFAULT 0,
    p_courier_cost_usd DOUBLE PRECISION DEFAULT 0
)
RETURNS JSONB AS $$
DECLARE
    v_input_cost DOUBLE PRECISION;
    v_output_cost DOUBLE PRECISION;
    v_theoretical DOUBLE PRECISION;
    v_actual DOUBLE PRECISION;
BEGIN
    SELECT 
        COALESCE(cost_input_per_1k_usd, 0),
        COALESCE(cost_output_per_1k_usd, 0)
    INTO v_input_cost, v_output_cost
    FROM models
    WHERE id = p_model_id
    LIMIT 1;

    v_theoretical := (p_tokens_in::DOUBLE PRECISION / 1000.0) * v_input_cost
                   + (p_tokens_out::DOUBLE PRECISION / 1000.0) * v_output_cost;

    v_actual := p_courier_cost_usd;

    RETURN jsonb_build_object(
        'model_id', p_model_id,
        'tokens_in', p_tokens_in,
        'tokens_out', p_tokens_out,
        'theoretical', v_theoretical,
        'actual', v_actual,
        'savings', GREATEST(v_theoretical - v_actual, 0)
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
