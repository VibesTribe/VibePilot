package main

import (
	"context"
	"log"

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

	_, err := database.Insert(ctx, "orchestrator_events", map[string]any{
		"event_type": eventType,
		"task_id":    taskID,
		"model_id":   modelID,
		"reason":     reason,
		"details":    eventDetails,
	})
	if err != nil {
		log.Printf("[recordPipelineEvent] Failed to write %s event: %v", eventType, err)
	}
}
