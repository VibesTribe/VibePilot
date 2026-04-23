package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

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
	// Try direct bool first (scalar RPC return)
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		return b
	}
	// Try rowsToJSON format: [{"function_name": true}]
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		for _, v := range rows[0] {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return false
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

// accumulateFailedModel appends a model ID to the routing_flag_reason field,
// tracking which models have failed this task. Format: "exec_failed_by:m1,m2" or "test_failed_by:m1,m2"
// Returns the unique set of failed models extracted from the updated field.
// When 2+ DIFFERENT models have failed the same task, flags the prompt as suspect.
func accumulateFailedModel(ctx context.Context, database db.Database, taskID string, prefix string, modelID string) []string {
	if modelID == "" {
		return nil
	}

	// Read current task to get existing routing_flag_reason
	data, err := database.Query(ctx, "tasks", map[string]any{
		"id":     "eq." + taskID,
		"select": "routing_flag_reason",
	})
	if err != nil {
		log.Printf("[Routing] Failed to read routing_flag_reason for task %s: %v", truncateID(taskID), err)
		return []string{modelID}
	}

	var tasks []map[string]any
	existing := ""
	if json.Unmarshal(data, &tasks) == nil && len(tasks) > 0 {
		existing = getString(tasks[0], "routing_flag_reason")
	}

	// Parse existing failed models from the matching prefix
	var models []string
	fullPrefix := prefix + ":"
	if after, ok := strings.CutPrefix(existing, fullPrefix); ok && after != "" {
		for _, m := range strings.Split(after, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				models = append(models, m)
			}
		}
	}

	// Add the new model if not already tracked
	alreadyTracked := false
	for _, m := range models {
		if m == modelID {
			alreadyTracked = true
			break
		}
	}
	if !alreadyTracked {
		models = append(models, modelID)
	}

	// Build the updated value
	newValue := fullPrefix + strings.Join(models, ",")

	// If 2+ different models have failed, flag the prompt as suspect
	if len(models) >= 2 {
		newValue += "|prompt_suspect"
		log.Printf("[Routing] WARNING: Task %s failed on %d different models (%v) — likely a prompt problem, not a model problem",
			truncateID(taskID), len(models), models)
	}

	database.Update(ctx, "tasks", taskID, map[string]any{
		"routing_flag_reason": newValue,
	})

	return models
}

// parseFailedModels extracts all failed model IDs from routing_flag_reason.
// Handles both single and accumulated formats:
//   "exec_failed_by:model_a,model_b" → ["model_a", "model_b"]
//   "test_failed_by:m1,m2|prompt_suspect" → ["m1", "m2"]
//   Legacy: "exec_failed_by:single" → ["single"]
func parseFailedModels(flagReason string) []string {
	var models []string
	seen := make(map[string]bool)

	// Split off any suffixes like |prompt_suspect
	mainPart := flagReason
	if idx := strings.Index(mainPart, "|"); idx >= 0 {
		mainPart = mainPart[:idx]
	}

	// Try exec_failed_by: prefix
	for _, prefix := range []string{"exec_failed_by:", "test_failed_by:"} {
		if after, ok := strings.CutPrefix(mainPart, prefix); ok && after != "" {
			for _, m := range strings.Split(after, ",") {
				m = strings.TrimSpace(m)
				if m != "" && !seen[m] {
					seen[m] = true
					models = append(models, m)
				}
			}
		}
	}

	return models
}

// isPromptSuspect checks if the routing_flag_reason indicates a prompt problem.
func isPromptSuspect(flagReason string) bool {
	return strings.Contains(flagReason, "|prompt_suspect")
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
