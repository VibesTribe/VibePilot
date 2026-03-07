package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/security"
)

type TaskHandler struct {
	database      *db.DB
	factory       *runtime.SessionFactory
	pool          *runtime.AgentPool
	connRouter    *runtime.Router
	git           *gitree.Gitree
	checkpointMgr *core.CheckpointManager
	leakDetector  *security.LeakDetector
	cfg           *runtime.Config
}

func NewTaskHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	checkpointMgr *core.CheckpointManager,
	leakDetector *security.LeakDetector,
	cfg *runtime.Config,
) *TaskHandler {
	return &TaskHandler{
		database:      database,
		factory:       factory,
		pool:          pool,
		connRouter:    connRouter,
		git:           git,
		checkpointMgr: checkpointMgr,
		leakDetector:  leakDetector,
		cfg:           cfg,
	}
}

func (h *TaskHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventTaskAvailable, h.handleTaskAvailable)
	router.On(runtime.EventTaskReview, h.handleTaskReview)
	router.On(runtime.EventTaskCompleted, h.handleTaskCompleted)
}

func (h *TaskHandler) handleTaskAvailable(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskAvailable] Failed to parse event: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	taskType := getString(task, "type")
	taskCategory := getString(task, "category")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		log.Printf("[TaskAvailable] Task has no ID, skipping")
		return
	}

	processingBy := fmt.Sprintf("task_runner:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskAvailable] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	taskPacket, err := h.database.GetTaskPacket(ctx, taskID)
	if err != nil {
		log.Printf("[TaskAvailable] Failed to get task packet for %s: %v", truncateID(taskID), err)
		h.handleTaskError(ctx, taskID, "", "packet_fetch_failed")
		return
	}

	if taskPacket.Prompt == "" {
		log.Printf("[TaskAvailable] Task %s has empty prompt", truncateID(taskID))
		h.handleTaskError(ctx, taskID, "", "empty_prompt")
		return
	}

	branchName := h.buildBranchName(taskNumber, taskID)
	if err := h.git.CreateBranch(ctx, branchName); err != nil {
		log.Printf("[TaskAvailable] Warning: branch creation failed for %s: %v", branchName, err)
	}

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "task_runner",
		TaskID:   taskID,
		TaskType: taskCategory,
	})
	if err != nil || routingResult == nil {
		routingResult, _ = h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
			AgentID:  "task_runner",
			TaskID:   taskID,
			TaskType: taskType,
		})
	}
	if routingResult == nil {
		log.Printf("[TaskAvailable] No destination for task %s", truncateID(taskID))
		h.handleTaskError(ctx, taskID, "", "no_destination")
		return
	}

	modelID := routingResult.ModelID
	connectorID := routingResult.DestinationID
	connConfig := h.cfg.GetConnector(connectorID)
	routingFlag := h.deriveRoutingFlag(connConfig)

	_, err = h.database.RPC(ctx, "update_task_assignment", map[string]any{
		"p_task_id":             taskID,
		"p_status":              "in_progress",
		"p_assigned_to":         modelID,
		"p_routing_flag":        routingFlag,
		"p_routing_flag_reason": fmt.Sprintf("Routed via %s", connectorID),
	})
	if err != nil {
		log.Printf("[TaskAvailable] Failed to update assignment for %s: %v", truncateID(taskID), err)
		h.handleTaskError(ctx, taskID, modelID, "assignment_failed")
		return
	}

	log.Printf("[TaskAvailable] Task %s assigned to %s via %s (flag=%s)",
		truncateID(taskID), modelID, connectorID, routingFlag)

	h.saveCheckpoint(ctx, taskID, "execution_start", 0, "", nil)

	runStart := time.Now()

	err = h.pool.SubmitWithDestination(ctx, sliceID, connectorID, func() error {
		return h.executeTask(ctx, task, taskPacket, taskID, taskNumber, modelID, connectorID, branchName, taskCategory, runStart)
	})
	if err != nil {
		log.Printf("[TaskAvailable] Failed to submit task %s: %v", truncateID(taskID), err)
		h.handleTaskError(ctx, taskID, modelID, "pool_submit_failed")
	}
}

func (h *TaskHandler) executeTask(
	ctx context.Context,
	task map[string]any,
	taskPacket *db.TaskPacket,
	taskID, taskNumber, modelID, connectorID, branchName, taskCategory string,
	runStart time.Time,
) error {
	taskType := getString(task, "type")

	var contextData map[string]any
	if len(taskPacket.Context) > 0 {
		json.Unmarshal(taskPacket.Context, &contextData)
	}

	session, err := h.factory.CreateWithContext(ctx, "task_runner", taskCategory)
	if err != nil {
		h.handleTaskError(ctx, taskID, modelID, "session_create_failed")
		return err
	}

	result, err := session.Run(ctx, map[string]any{
		"task_id":         taskID,
		"task_number":     taskNumber,
		"title":           getString(task, "title"),
		"type":            taskType,
		"category":        taskCategory,
		"prompt_packet":   taskPacket.Prompt,
		"expected_output": taskPacket.ExpectedOutput,
		"context":         contextData,
		"dependencies":    task["dependencies"],
		"event":           "task_available",
	})
	if err != nil {
		log.Printf("[TaskAvailable] Execution failed for %s: %v", truncateID(taskID), err)
		h.handleTaskError(ctx, taskID, modelID, "execution_error")
		return err
	}

	cleanOutput, leaks := h.leakDetector.Scan(result.Output)
	if len(leaks) > 0 {
		log.Printf("[TaskAvailable] SECURITY: %d leak(s) redacted in %s", len(leaks), truncateID(taskID))
	}

	duration := time.Since(runStart)
	tokensIn := result.TokensIn
	tokensOut := result.TokensOut
	totalTokens := tokensIn + tokensOut

	taskOutput, parseErr := runtime.ParseTaskRunnerOutput(result.Output)
	var files []runtime.File
	var summary, status string
	if parseErr != nil {
		log.Printf("[TaskAvailable] Failed to parse output for %s: %v", truncateID(taskID), parseErr)
		summary = cleanOutput
		status = "success"
	} else {
		files = taskOutput.Files
		summary = taskOutput.Summary
		status = taskOutput.Status
		if status == "" {
			status = "success"
		}
	}

	commitErr := h.commitOutput(ctx, branchName, files, cleanOutput, summary, modelID, taskID, duration.Seconds())
	if commitErr != nil {
		log.Printf("[TaskAvailable] Commit failed for %s: %v", branchName, commitErr)
	}

	costs := h.calculateCosts(ctx, modelID, tokensIn, tokensOut)

	_, err = h.database.RPC(ctx, "create_task_run", map[string]any{
		"p_task_id":                       taskID,
		"p_model_id":                      modelID,
		"p_courier":                       connectorID,
		"p_platform":                      h.deriveRoutingFlag(h.cfg.GetConnector(connectorID)),
		"p_status":                        status,
		"p_tokens_in":                     tokensIn,
		"p_tokens_out":                    tokensOut,
		"p_tokens_used":                   totalTokens,
		"p_courier_model_id":              nil,
		"p_courier_tokens":                0,
		"p_courier_cost_usd":              0,
		"p_platform_theoretical_cost_usd": costs.Theoretical,
		"p_total_actual_cost_usd":         costs.Actual,
		"p_total_savings_usd":             costs.Savings,
		"p_started_at":                    runStart,
		"p_completed_at":                  time.Now(),
	})
	if err != nil {
		log.Printf("[TaskAvailable] Warning: task_run creation failed for %s: %v", truncateID(taskID), err)
	} else {
		log.Printf("[TaskAvailable] task_run created for %s: tokens=%d, savings=$%.4f",
			truncateID(taskID), totalTokens, costs.Savings)
	}

	if status == "success" && commitErr == nil {
		_, err = h.database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "review",
		})
		h.recordSuccess(ctx, modelID, taskCategory, duration.Seconds(), totalTokens)
		log.Printf("[TaskAvailable] Task %s complete, status=review", truncateID(taskID))
	} else {
		_, err = h.database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "available",
		})
		h.recordFailure(ctx, modelID, taskID, "commit_or_parse_failed")
		log.Printf("[TaskAvailable] Task %s failed, retrying", truncateID(taskID))
	}

	h.deleteCheckpoint(ctx, taskID)

	return nil
}

func (h *TaskHandler) handleTaskReview(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		return
	}

	taskID := getString(task, "id")
	taskType := getString(task, "type")
	modelID := getString(task, "assigned_to")
	sliceID := getStringOr(task, "slice_id", "review")

	if taskID == "" {
		return
	}

	processingBy := fmt.Sprintf("supervisor_review:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskReview] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "supervisor",
		TaskID:   taskID,
		TaskType: taskType,
	})
	if err != nil || routingResult == nil {
		log.Printf("[TaskReview] No destination for task %s", truncateID(taskID))
		return
	}

	session, err := h.factory.CreateWithContext(ctx, "supervisor", taskType)
	if err != nil {
		log.Printf("[TaskReview] Failed to create session for %s: %v", truncateID(taskID), err)
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.DestinationID, func() error {
		result, err := session.Run(ctx, map[string]any{
			"task":  task,
			"event": "task_review",
		})
		if err != nil {
			log.Printf("[TaskReview] Session failed for %s: %v", truncateID(taskID), err)
			return err
		}

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[TaskReview] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
			return nil
		}

		log.Printf("[TaskReview] Task %s decision: %s, next: %s",
			truncateID(taskID), decision.Decision, decision.NextAction)

		switch decision.Decision {
		case "pass":
			_, err = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "testing",
			})
			if err != nil {
				log.Printf("[TaskReview] Failed to update status: %v", err)
			}

		case "fail":
			h.recordIssues(ctx, taskID, modelID, taskType, decision.Issues)
			h.recordFailure(ctx, modelID, taskID, "supervisor_reject")

			nextStatus := "available"
			if decision.NextAction == "split_task" || decision.NextAction == "escalate" {
				nextStatus = "escalated"
			}
			_, err = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  nextStatus,
			})

		case "reroute":
			_, err = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
		}

		return nil
	})
	if err != nil {
		log.Printf("[TaskReview] Failed to submit: %v", err)
	}
}

func (h *TaskHandler) handleTaskCompleted(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		return
	}

	taskID := getString(task, "id")
	taskType := getString(task, "type")
	taskNumber := getString(task, "task_number")
	modelID := getString(task, "assigned_to")
	sliceID := getStringOr(task, "slice_id", "complete")

	if taskID == "" {
		return
	}

	processingBy := fmt.Sprintf("supervisor_completed:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskCompleted] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	branchName := h.buildBranchName(taskNumber, taskID)

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "supervisor",
		TaskID:   taskID,
		TaskType: taskType,
	})
	if err != nil || routingResult == nil {
		log.Printf("[TaskCompleted] No destination for task %s", truncateID(taskID))
		h.recordFailure(ctx, modelID, taskID, "no_destination")
		return
	}

	session, err := h.factory.CreateWithContext(ctx, "supervisor", taskType)
	if err != nil {
		log.Printf("[TaskCompleted] Failed to create session for %s: %v", truncateID(taskID), err)
		h.recordFailure(ctx, modelID, taskID, "session_create_failed")
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.DestinationID, func() error {
		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{
			"task":  task,
			"event": "task_completed",
		})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[TaskCompleted] Session failed for %s: %v", truncateID(taskID), sessionErr)
			h.recordFailure(ctx, modelID, taskID, "session_error")
			return sessionErr
		}

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[TaskCompleted] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
			return nil
		}

		log.Printf("[TaskCompleted] Task %s decision: %s, next: %s",
			truncateID(taskID), decision.Decision, decision.NextAction)

		output := map[string]any{
			"output":     result.Output,
			"model_id":   modelID,
			"task_id":    taskID,
			"duration":   duration,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
			"decision":   decision.Decision,
		}

		if err := h.git.CommitOutput(ctx, branchName, output); err != nil {
			log.Printf("[TaskCompleted] Commit failed for %s: %v", branchName, err)
		}

		switch decision.Decision {
		case "pass":
			if decision.NextAction == "final_merge" {
				targetBranch := h.cfg.GetDefaultMergeTarget()
				if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
					log.Printf("[TaskCompleted] Merge failed for %s: %v", branchName, err)
					_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "escalated",
					})
				} else {
					log.Printf("[TaskCompleted] Merged %s to %s", branchName, targetBranch)
					_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "merged",
					})
					h.git.DeleteBranch(ctx, branchName)
				}
			} else {
				_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "approval",
				})
			}
			h.recordSuccess(ctx, modelID, taskType, duration, result.TokensIn+result.TokensOut)

		case "fail":
			h.recordIssues(ctx, taskID, modelID, taskType, decision.Issues)
			h.recordFailure(ctx, modelID, taskID, decision.NextAction)

			nextStatus := "available"
			if decision.NextAction != "return_to_runner" {
				nextStatus = "escalated"
			}
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  nextStatus,
			})

		default:
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
		}

		return nil
	})
	if err != nil {
		log.Printf("[TaskCompleted] Failed to submit: %v", err)
	}
}

func (h *TaskHandler) handleTaskError(ctx context.Context, taskID, modelID, failureType string) {
	if modelID != "" {
		h.recordFailure(ctx, modelID, taskID, failureType)
	}
	_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
		"p_task_id": taskID,
		"p_status":  "available",
	})
}

func (h *TaskHandler) recordSuccess(ctx context.Context, modelID, taskType string, durationSeconds float64, tokensUsed int) {
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

func (h *TaskHandler) recordFailure(ctx context.Context, modelID, taskID, failureType string) {
	if modelID == "" {
		return
	}
	_, err := h.database.RPC(ctx, "record_model_failure", map[string]any{
		"p_model_id":         modelID,
		"p_task_id":          taskID,
		"p_failure_type":     failureType,
		"p_failure_category": runtime.CategorizeFailure(failureType),
	})
	if err != nil {
		log.Printf("[Learning] Failed to record failure: %v", err)
	}
}

func (h *TaskHandler) recordIssues(ctx context.Context, taskID, modelID, taskType string, issues []struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}) {
	for _, issue := range issues {
		_, _ = h.database.RPC(ctx, "record_failure", map[string]any{
			"p_task_id":          taskID,
			"p_failure_type":     issue.Type,
			"p_failure_category": runtime.CategorizeFailure(issue.Type),
			"p_failure_details": map[string]any{
				"description": issue.Description,
				"severity":    issue.Severity,
			},
			"p_model_id":  modelID,
			"p_task_type": taskType,
		})
	}
}

type costResult struct {
	Theoretical float64
	Actual      float64
	Savings     float64
}

func (h *TaskHandler) calculateCosts(ctx context.Context, modelID string, tokensIn, tokensOut int) costResult {
	result, err := h.database.RPC(ctx, "calculate_run_costs", map[string]any{
		"p_model_id":         modelID,
		"p_tokens_in":        tokensIn,
		"p_tokens_out":       tokensOut,
		"p_courier_cost_usd": 0,
	})
	if err != nil {
		return costResult{}
	}

	var costs struct {
		TheoreticalCostUsd float64 `json:"theoretical_cost_usd"`
		ActualCostUsd      float64 `json:"actual_cost_usd"`
		SavingsUsd         float64 `json:"savings_usd"`
	}
	if result != nil {
		json.Unmarshal(result, &costs)
	}

	return costResult{
		Theoretical: costs.TheoreticalCostUsd,
		Actual:      costs.ActualCostUsd,
		Savings:     costs.SavingsUsd,
	}
}

func (h *TaskHandler) commitOutput(ctx context.Context, branchName string, files []runtime.File, rawOutput, summary, modelID, taskID string, duration float64) error {
	outputMap := map[string]any{
		"raw_output": rawOutput,
		"model_id":   modelID,
		"task_id":    taskID,
		"duration":   duration,
		"summary":    summary,
	}

	if len(files) > 0 {
		var fileMaps []map[string]any
		for _, f := range files {
			fileMaps = append(fileMaps, map[string]any{
				"path":    f.Path,
				"content": f.Content,
			})
		}
		outputMap["files"] = fileMaps
	}

	return h.git.CommitOutput(ctx, branchName, outputMap)
}

func (h *TaskHandler) saveCheckpoint(ctx context.Context, taskID, step string, progress int, output string, files []string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	_, err := h.database.RPC(ctx, "save_checkpoint", map[string]any{
		"p_task_id":  taskID,
		"p_step":     step,
		"p_progress": progress,
		"p_output":   output,
		"p_files":    files,
	})
	if err != nil {
		log.Printf("[Checkpoint] Warning: save failed for %s: %v", truncateID(taskID), err)
	}
}

func (h *TaskHandler) deleteCheckpoint(ctx context.Context, taskID string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	_, err := h.database.RPC(ctx, "delete_checkpoint", map[string]any{
		"p_task_id": taskID,
	})
	if err != nil {
		log.Printf("[Checkpoint] Warning: delete failed for %s: %v", truncateID(taskID), err)
	}
}

func (h *TaskHandler) buildBranchName(taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}
	if taskNumber != "" {
		return prefix + taskNumber
	}
	return prefix + truncateID(taskID)
}

func (h *TaskHandler) deriveRoutingFlag(conn *runtime.ConnectorConfig) string {
	if conn == nil {
		return "internal"
	}
	switch conn.Type {
	case "cli", "api":
		return "internal"
	case "mcp":
		return "mcp"
	case "web":
		return "web"
	default:
		return "internal"
	}
}

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

func setupTaskHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	checkpointMgr *core.CheckpointManager,
	leakDetector *security.LeakDetector,
) {
	handler := NewTaskHandler(database, factory, pool, connRouter, git, checkpointMgr, leakDetector, cfg)
	handler.Register(router)
}
