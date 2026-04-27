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
//   prd_committed, plan_created, plan_approved, plan_rejected,
//   council_approved, council_rejected, task_started, task_dispatched,
//   run_completed, run_failed, task_completed, task_failed,
//   test_passed, test_failed, failure_detected,
//   revision_needed, reroute, approved, maintenance_started/completed/failed
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
