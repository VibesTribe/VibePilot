package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/vibepilot/governor/internal/db"
)

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringOr(m map[string]any, key, def string) string {
	if v := getString(m, key); v != "" {
		return v
	}
	return def
}

func parseBool(data []byte) bool {
	if data == nil {
		return false
	}
	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		return false
	}
	return b
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncateOutput(output string) string {
	if len(output) > 5000 {
		return output[:5000] + "..."
	}
	return output
}

func extractCouncilReviews(plan map[string]any) []map[string]any {
	reviews := plan["council_reviews"]
	if reviews == nil {
		return nil
	}

	switch v := reviews.(type) {
	case []interface{}:
		var result []map[string]any
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	case []map[string]interface{}:
		return v
	default:
		return nil
	}
}

func recordModelSuccess(ctx context.Context, database db.Database, modelID, taskType string, durationSeconds float64) {
	if modelID == "" {
		return
	}
	_, err := database.RPC(ctx, "record_model_success", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_duration_seconds": durationSeconds,
	})
	if err != nil {
		log.Printf("[Learning] Failed to record model success: %v", err)
	}
}

func recordModelFailure(ctx context.Context, database db.Database, modelID, taskID, failureType string) {
	if modelID == "" {
		return
	}
	_, err := database.RPC(ctx, "record_model_failure", map[string]any{
		"p_model_id":     modelID,
		"p_failure_type": failureType,
		"p_task_id":      taskID,
	})
	if err != nil {
		log.Printf("[Learning] Failed to record model failure: %v", err)
	}
}
