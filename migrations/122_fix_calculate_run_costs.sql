-- Migration 122 (v5): Nuclear option - drop ALL calculate_run_costs variants
-- The overloading is stuck, must use specific signatures

-- Kill every possible signature
DO $$
DECLARE
    sig TEXT;
BEGIN
    FOR sig IN 
        SELECT pg_get_function_identity_arguments(oid) 
        FROM pg_proc 
        WHERE proname = 'calculate_run_costs' 
        AND pronamespace = 'public'::regnamespace
    LOOP
        EXECUTE 'DROP FUNCTION IF EXISTS calculate_run_costs(' || sig || ') CASCADE';
        RAISE NOTICE 'Dropped calculate_run_costs(%)', sig;
    END LOOP;
END;
$$;

-- Now create the single clean version
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
