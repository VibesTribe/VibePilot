package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RecordStateTransition records a state transition for recovery and debugging
func (d *DB) RecordStateTransition(ctx context.Context, entityType string, entityID string, fromState string, toState string, reason string, metadata map[string]any) error {
	_, err := d.RPC(ctx, "record_state_transition", map[string]any{
		"p_entity_type": entityType,
		"p_entity_id":   entityID,
		"p_from_state":  fromState,
		"p_to_state":    toState,
		"p_reason":      reason,
		"p_metadata":    metadata,
	})
	if err != nil {
		return fmt.Errorf("record state transition: %w", err)
	}
	return nil
}

// RecordPerformanceMetric records a performance metric for optimization
func (d *DB) RecordPerformanceMetric(ctx context.Context, metricType string, entityID string, duration time.Duration, success bool, metadata map[string]any) error {
	_, err := d.RPC(ctx, "record_performance_metric", map[string]any{
		"p_metric_type":      metricType,
		"p_entity_id":        entityID,
		"p_duration_seconds": duration.Seconds(),
		"p_success":          success,
		"p_metadata":         metadata,
	})
	if err != nil {
		return fmt.Errorf("record performance metric: %w", err)
	}
	return nil
}

// GetLatestState gets the latest state for an entity
func (d *DB) GetLatestState(ctx context.Context, entityType string, entityID string) (toState string, reason string, createdAt time.Time, err error) {
	result, err := d.RPC(ctx, "get_latest_state", map[string]any{
		"p_entity_type": entityType,
		"p_entity_id":   entityID,
	})
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("get latest state: %w", err)
	}

	var states []struct {
		ToState          string    `json:"to_state"`
		TransitionReason string    `json:"transition_reason"`
		CreatedAt        time.Time `json:"created_at"`
	}

	if err := json.Unmarshal(result, &states); err != nil {
		return "", "", time.Time{}, fmt.Errorf("parse latest state: %w", err)
	}

	if len(states) == 0 {
		return "", "", time.Time{}, nil
	}

	return states[0].ToState, states[0].TransitionReason, states[0].CreatedAt, nil
}

// ClearProcessingAndRecordTransition clears processing and records state transition
func (d *DB) ClearProcessingAndRecordTransition(ctx context.Context, table string, id string, fromState string, toState string, reason string) error {
	// Clear processing
	_, err := d.RPC(ctx, "clear_processing", map[string]any{
		"p_table": table,
		"p_id":    id,
	})
	if err != nil {
		return fmt.Errorf("clear processing: %w", err)
	}

	// Record transition
	if err := d.RecordStateTransition(ctx, table, id, fromState, toState, reason, nil); err != nil {
		// Log but don't fail - transition recording is not critical
		return nil
	}

	return nil
}
