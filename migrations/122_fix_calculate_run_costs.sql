-- Migration 122 (v3): Fix calculate_run_costs RPC
-- Step 1: Kill the old UUID-based version that returns wrong data
-- Step 2: Create the pricing calculator version
-- Reads cost_input_per_1k_usd and cost_output_per_1k_usd from models table

DROP FUNCTION IF EXISTS calculate_run_costs(UUID) CASCADE;

CREATE OR REPLACE FUNCTION calculate_run_costs(
    p_model_id TEXT,
    p_tokens_in INT DEFAULT 0,
    p_tokens_out INT DEFAULT 0,
    p_courier_cost_usd NUMERIC DEFAULT 0
)
RETURNS JSONB AS $$
DECLARE
    v_input_cost NUMERIC;
    v_output_cost NUMERIC;
    v_theoretical NUMERIC;
    v_actual NUMERIC;
BEGIN
    SELECT 
        COALESCE(cost_input_per_1k_usd, 0),
        COALESCE(cost_output_per_1k_usd, 0)
    INTO v_input_cost, v_output_cost
    FROM models
    WHERE id = p_model_id
    LIMIT 1;

    v_theoretical := (p_tokens_in::NUMERIC / 1000.0) * v_input_cost
                   + (p_tokens_out::NUMERIC / 1000.0) * v_output_cost;

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
