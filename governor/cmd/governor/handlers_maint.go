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

type MaintenanceHandler struct {
	database   *db.DB
	factory    *runtime.SessionFactory
	pool       *runtime.AgentPool
	connRouter *runtime.Router
	cfg        *runtime.Config
}

func NewMaintenanceHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
) *MaintenanceHandler {
	return &MaintenanceHandler{
		database:   database,
		factory:    factory,
		pool:       pool,
		connRouter: connRouter,
		cfg:        cfg,
	}
}

func (h *MaintenanceHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventMaintenanceCmd, h.handleMaintenanceCommand)
}

func (h *MaintenanceHandler) handleMaintenanceCommand(event runtime.Event) {
	ctx := context.Background()

	var cmd map[string]any
	if err := json.Unmarshal(event.Record, &cmd); err != nil {
		log.Printf("[MaintenanceCmd] Failed to parse event: %v", err)
		return
	}

	cmdID := getString(cmd, "id")
	cmdType := getString(cmd, "command_type")
	payload := cmd["payload"]

	if cmdID == "" {
		return
	}

	processingBy := fmt.Sprintf("maintenance_cmd:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "maintenance_commands",
		"p_id":            cmdID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[MaintenanceCmd] Command %s already being processed", truncateID(cmdID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "maintenance_commands",
		"p_id":    cmdID,
	})

	log.Printf("[MaintenanceCmd] Processing command %s (type: %s)", truncateID(cmdID), cmdType)

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "maintenance",
		TaskID:   cmdID,
		TaskType: cmdType,
	})
	if err != nil || routingResult == nil {
		log.Printf("[MaintenanceCmd] No destination for command %s", truncateID(cmdID))
		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":           cmdID,
			"p_status":       "failed",
			"p_result_notes": map[string]any{"error": "no_destination"},
		})
		return
	}

	session, err := h.factory.CreateWithContext(ctx, "maintenance", cmdType)
	if err != nil {
		log.Printf("[MaintenanceCmd] Failed to create session for %s: %v", truncateID(cmdID), err)
		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":           cmdID,
			"p_status":       "failed",
			"p_result_notes": map[string]any{"error": err.Error()},
		})
		return
	}

	err = h.pool.SubmitWithDestination(ctx, "maintenance", routingResult.DestinationID, func() error {
		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{
			"command":      cmd,
			"command_type": cmdType,
			"payload":      payload,
			"event":        "maintenance_command",
		})
		duration := time.Since(start)

		if sessionErr != nil {
			log.Printf("[MaintenanceCmd] Execution failed for %s: %v", truncateID(cmdID), sessionErr)
			_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
				"p_id":           cmdID,
				"p_status":       "failed",
				"p_result_notes": map[string]any{"error": sessionErr.Error()},
			})
			return sessionErr
		}

		log.Printf("[MaintenanceCmd] Command %s executed via %s in %v", truncateID(cmdID), routingResult.DestinationID, duration)

		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":     cmdID,
			"p_status": "completed",
			"p_result_notes": map[string]any{
				"output":       result.Output,
				"duration_ms":  duration.Milliseconds(),
				"tokens_in":    result.TokensIn,
				"tokens_out":   result.TokensOut,
				"connector_id": routingResult.DestinationID,
				"model_id":     routingResult.ModelID,
			},
		})

		h.recordSuccess(ctx, routingResult.ModelID, cmdType, duration.Seconds(), result.TokensIn+result.TokensOut)

		return nil
	})
	if err != nil {
		log.Printf("[MaintenanceCmd] Failed to submit: %v", err)
	}
}

func (h *MaintenanceHandler) recordSuccess(ctx context.Context, modelID, taskType string, durationSeconds float64, tokensUsed int) {
	if modelID == "" {
		return
	}
	_, err := h.database.RPC(ctx, "record_model_success", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_duration_seconds": durationSeconds,
		"p_tokens_used":      tokensUsed,
	})
	if err != nil {
		log.Printf("[Learning] Failed to record success: %v", err)
	}
}

func setupMaintenanceHandler(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
) {
	handler := NewMaintenanceHandler(database, factory, pool, connRouter, cfg)
	handler.Register(router)
}
