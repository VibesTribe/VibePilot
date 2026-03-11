package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

type MaintenanceHandler struct {
	database   *db.DB
	factory    *runtime.SessionFactory
	pool       *runtime.AgentPool
	connRouter *runtime.Router
	cfg        *runtime.Config
	git        *gitree.Gitree
}

func NewMaintenanceHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
	git *gitree.Gitree,
) *MaintenanceHandler {
	return &MaintenanceHandler{
		database:   database,
		factory:    factory,
		pool:       pool,
		connRouter: connRouter,
		cfg:        cfg,
		git:        git,
	}
}

func (h *MaintenanceHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventMaintenanceCmd, h.handleMaintenanceCommand)
	router.On(runtime.EventTaskApproval, h.handleTaskApproved)
	router.On(runtime.EventTaskMergePending, h.handleTaskMergePending)
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

func (h *MaintenanceHandler) handleTaskApproved(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskApproved] Failed to parse event: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// Claim for merge
	mergeID := fmt.Sprintf("merge:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": mergeID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskApproved] Task %s already being processed", truncateID(taskID))
		return
	}

	branchName := h.buildBranchName(taskNumber, taskID)
	targetBranch := h.getTargetBranch(sliceID)

	log.Printf("[TaskApproved] Merging %s -> %s for task %s", branchName, targetBranch, truncateID(taskID))

	if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
		log.Printf("[TaskApproved] Merge failed for %s: %v", truncateID(taskID), err)
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "approval",
			"p_failure_reason": "merge_failed",
		})
		return
	}

	h.git.DeleteBranch(ctx, branchName)

	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "merged",
	})
	h.database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
		"p_completed_task_id": taskID,
	})
	log.Printf("[TaskApproved] Task %s merged to %s", truncateID(taskID), targetBranch)
}

func (h *MaintenanceHandler) handleTaskMergePending(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskMergePending] Failed to parse event: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// Claim for merge retry
	mergeID := fmt.Sprintf("merge_retry:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": mergeID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskMergePending] Task %s already being processed", truncateID(taskID))
		return
	}

	log.Printf("[TaskMergePending] Creating maintenance command for task %s", truncateID(taskID))

	_, err = h.database.RPC(ctx, "create_maintenance_command", map[string]any{
		"p_command_type": "merge_conflict",
		"p_payload": map[string]any{
			"task_id":       taskID,
			"task_number":   taskNumber,
			"slice_id":      sliceID,
			"branch_name":   h.buildBranchName(taskNumber, taskID),
			"target_branch": h.getTargetBranch(sliceID),
		},
		"p_status": "pending",
	})
	if err != nil {
		log.Printf("[TaskMergePending] Failed to create maintenance command: %v", err)
	}
}

func (h *MaintenanceHandler) buildBranchName(taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}
	if taskNumber != "" {
		return prefix + taskNumber
	}
	return prefix + truncateID(taskID)
}

func (h *MaintenanceHandler) getTargetBranch(sliceID string) string {
	if sliceID != "" && sliceID != "default" && sliceID != "testing" && sliceID != "review" {
		return "module/" + sliceID
	}
	return h.cfg.GetDefaultMergeTarget()
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
	git *gitree.Gitree,
) {
	handler := NewMaintenanceHandler(database, factory, pool, connRouter, cfg, git)
	handler.Register(router)
}
