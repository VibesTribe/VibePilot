package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

func setupMaintenanceHandler(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
) {
	selectDestination := func(agentID, cmdID, taskType string) string {
		result, err := connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
			AgentID:  agentID,
			TaskID:   cmdID,
			TaskType: taskType,
		})
		if err != nil || result == nil {
			log.Printf("[Router] No destination available for agent %s, using fallback", agentID)
			dests := connRouter.GetAvailableConnectors()
			if len(dests) > 0 {
				return dests[0]
			}
			return ""
		}
		return result.DestinationID
	}

	router.On(runtime.EventMaintenanceCmd, func(event runtime.Event) {
		var cmd map[string]any
		if err := json.Unmarshal(event.Record, &cmd); err != nil {
			return
		}

		cmdID, _ := cmd["id"].(string)

		processingBy := fmt.Sprintf("maintenance_cmd:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "maintenance_commands",
			"p_id":            cmdID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventMaintenanceCmd] Command %s already being processed or claim failed", truncateID(cmdID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventMaintenanceCmd] Command %s already being processed", truncateID(cmdID))
			return
		}

		destID := selectDestination("maintenance", cmdID, "maintenance")
		if destID == "" {
			log.Printf("[EventMaintenanceCmd] No destination available for command %s", truncateID(cmdID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			return
		}

		session, err := factory.Create("maintenance")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "maintenance", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})

			result, err := session.Run(ctx, map[string]any{"command": cmd, "event": "maintenance_command"})
			if err != nil {
				return err
			}
			log.Printf("[EventMaintenanceCmd] Command %s executed via %s: %s", truncateID(cmdID), destID, truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			log.Printf("[EventMaintenanceCmd] Failed to submit to pool: %v", err)
		}
	})
}
