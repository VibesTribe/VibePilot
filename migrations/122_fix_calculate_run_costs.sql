-- Migration 122 (v4): Fix calculate_run_costs - kill duplicates
-- Two copies exist (NUMERIC and DOUBLE PRECISION variants), PostgREST can't choose

DROP FUNCTION IF EXISTS calculate_run_costs(UUID) CASCADE;
DROP FUNCTION IF EXISTS calculate_run_costs(TEXT, INT, INT, NUMERIC) CASCADE;
DROP FUNCTION IF EXISTS calculate_run_costs(TEXT, INT, INT, DOUBLE PRECISION) CASCADE;

CREATE OR REPLACE FUNCTION calculate_run_costs(
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
