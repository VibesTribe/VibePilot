-- Migration 125: deduct_model_credit RPC
-- Deducts cost from a model's credit_remaining_usd and checks for low credit alert
-- Safe when credit_remaining_usd is NULL (free/subscription models)

CREATE OR REPLACE FUNCTION public.deduct_model_credit(
    p_model_id TEXT,
    p_cost_usd DOUBLE PRECISION
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_current_credit DOUBLE PRECISION;
    v_alert_threshold DOUBLE PRECISION;
    v_new_credit DOUBLE PRECISION;
BEGIN
    -- Get current credit and threshold
    SELECT credit_remaining_usd, COALESCE(credit_alert_threshold, 0)
    INTO v_current_credit, v_alert_threshold
    FROM public.models WHERE id = p_model_id;

    -- If no credit tracking (NULL), skip silently
    IF v_current_credit IS NULL THEN
        RETURN jsonb_build_object(
            'deducted', false,
            'reason', 'no_credit_tracking'
        );
    END IF;

    -- Deduct
    v_new_credit := GREATEST(v_current_credit - p_cost_usd, 0);

    UPDATE public.models
    SET credit_remaining_usd = v_new_credit,
        updated_at = now()
    WHERE id = p_model_id;

    -- Check for low credit alert
    IF v_new_credit <= v_alert_threshold AND v_current_credit > v_alert_threshold THEN
        RETURN jsonb_build_object(
            'deducted', true,
            'previous_credit', v_current_credit,
            'new_credit', v_new_credit,
            'deducted_amount', p_cost_usd,
            'low_credit_alert', true,
            'threshold', v_alert_threshold
        );
    END IF;

    RETURN jsonb_build_object(
        'deducted', true,
        'previous_credit', v_current_credit,
        'new_credit', v_new_credit,
        'deducted_amount', p_cost_usd,
        'low_credit_alert', false
    );
END;
$$;
