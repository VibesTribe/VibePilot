package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/vibepilot/governor/internal/db"
)

// recordPipelineEvent writes a human-readable lifecycle event to orchestrator_events
// for the dashboard timeline popup. The dashboard's getEventMeta() maps event_type
// strings to labels, icons, and tones. Keep event types in sync with:
//   - vibeflow/apps/dashboard/components/modals/MissionModals.tsx (getEventMeta)
//   - vibeflow/apps/dashboard/hooks/useMissionData.ts (pipeline event builder)
//
// Standard event types the dashboard recognizes:
//   PLANNING: prd_committed, planner_called, plan_created, supervisor_called,
//   plan_approved, plan_rejected, council_approved, council_rejected
//   EXECUTION: task_dispatched, output_received, run_completed, run_failed,
//   revision_needed, reroute
//   TESTING: test_passed, test_failed
//   MERGE: task_merged_to_module, merge_conflict_detected,
//   module_merged_to_testing, module_merge_failed,
//   integration_merge_failed, plan_complete
func recordPipelineEvent(ctx context.Context, database db.Database, eventType, taskID, modelID, reason string, details map[string]any) {
	eventDetails := details
	if eventDetails == nil {
		eventDetails = map[string]any{}
	}
	eventDetails["model_id"] = modelID
	eventDetails["source"] = "pipeline"

	// Resolve project_id from task (empty taskID = plan-level event = vibepilot default)
	projectID := ""
	if taskID != "" && !strings.HasPrefix(taskID, "plan-") {
		taskData, qErr := database.Query(ctx, "tasks", map[string]any{
			"select": "project_id",
			"id":     fmt.Sprintf("eq.%s", taskID),
			"limit":  1,
		})
		if qErr == nil {
			var taskRows []map[string]any
			if json.Unmarshal(taskData, &taskRows) == nil && len(taskRows) > 0 {
				if pid, ok := taskRows[0]["project_id"].(string); ok {
					projectID = pid
				}
			}
		}
	}

	eventData := map[string]any{
		"event_type": eventType,
		"task_id":    taskID,
		"model_id":   modelID,
		"reason":     reason,
		"details":    eventDetails,
	}
	if projectID != "" {
		eventData["project_id"] = projectID
	}

	_, err := database.Insert(ctx, "orchestrator_events", eventData)
	if err != nil {
		log.Printf("[recordPipelineEvent] Failed to write %s event: %v", eventType, err)
	}
}
