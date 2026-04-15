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
}

// ============================================================================
// TASK EXECUTION: available → in_progress → review
// ============================================================================

func (h *TaskHandler) handleTaskAvailable(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskAvailable] Parse error: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	taskType := getString(task, "type")
	taskCategory := getString(task, "category")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// Get task packet
	taskPacket, err := h.database.GetTaskPacket(ctx, taskID)
	if err != nil {
		if result, ok := task["result"].(map[string]any); ok {
			if prompt, ok := result["prompt_packet"].(string); ok && prompt != "" {
				taskPacket = &db.TaskPacket{TaskID: taskID, Prompt: prompt}
			}
		}
		if taskPacket == nil || taskPacket.Prompt == "" {
			log.Printf("[TaskAvailable] No packet for %s", truncateID(taskID))
			return
		}
	}

	// Route to model
	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "internal_cli",
		TaskID:   taskID,
		TaskType: taskCategory,
	})
	if err != nil || routingResult == nil {
		routingResult, _ = h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
			AgentID:  "internal_cli",
			TaskID:   taskID,
			TaskType: taskType,
		})
	}
	if routingResult == nil {
		log.Printf("[TaskAvailable] No route for %s", truncateID(taskID))
		return
	}

	modelID := routingResult.ModelID
	connectorID := routingResult.DestinationID
	connConfig := h.cfg.GetConnector(connectorID)
	routingFlag := h.deriveRoutingFlag(connConfig)

	// Check pool capacity BEFORE claiming
	if !h.pool.HasCapacity(sliceID, connectorID) {
		log.Printf("[TaskAvailable] Task %s pending - no capacity (slice=%s, dest=%s)", truncateID(taskID), sliceID, connectorID)
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": "pending_resources",
		})
		return
	}

	// Atomically claim task
	workerID := fmt.Sprintf("executor:%s:%d", modelID, time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_task", map[string]any{
		"p_task_id":        taskID,
		"p_worker_id":      workerID,
		"p_model_id":       modelID,
		"p_routing_flag":   routingFlag,
		"p_routing_reason": fmt.Sprintf("Routed via %s", connectorID),
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskAvailable] Task %s claim failed: err=%v, result=%s", truncateID(taskID), err, string(claimed))
		return
	}

	log.Printf("[TaskAvailable] Task %s claimed by %s", truncateID(taskID), modelID)

	// Setup branch
	branchName := h.buildBranchName(sliceID, taskNumber, taskID)
	attempts := 0
	if v, ok := task["attempts"].(float64); ok {
		attempts = int(v)
	}
	if attempts > 0 {
		h.git.DeleteBranch(ctx, branchName)
	}
	h.git.CreateBranch(ctx, branchName)
	h.database.RPC(ctx, "update_task_branch", map[string]any{
		"p_task_id":     taskID,
		"p_branch_name": branchName,
	})

	h.saveCheckpoint(ctx, taskID, "execution_start", 0, "", nil)
	runStart := time.Now()

	// Execute
	err = h.pool.SubmitWithDestination(ctx, sliceID, connectorID, func() error {
		return h.executeTask(ctx, task, taskPacket, taskID, taskNumber, modelID, connectorID, branchName, taskCategory, runStart)
	})
	if err != nil {
		log.Printf("[TaskAvailable] Pool error for %s: %v", truncateID(taskID), err)
		h.failTask(ctx, taskID, modelID, branchName, "pool_submit_failed")
	}
}

func (h *TaskHandler) executeTask(
	ctx context.Context,
	task map[string]any,
	taskPacket *db.TaskPacket,
	taskID, taskNumber, modelID, connectorID, branchName, taskCategory string,
	runStart time.Time,
) error {

	var contextData map[string]any
	if len(taskPacket.Context) > 0 {
		json.Unmarshal(taskPacket.Context, &contextData)
	}

	session, err := h.factory.CreateWithConnector(ctx, "internal_cli", taskCategory, connectorID)
	if err != nil {
		h.failTask(ctx, taskID, modelID, branchName, "session_create_failed")
		return err
	}

	result, err := session.Run(ctx, map[string]any{
		"task_id":         taskID,
		"task_number":     taskNumber,
		"title":           getString(task, "title"),
		"type":            getString(task, "type"),
		"category":        taskCategory,
		"prompt_packet":   taskPacket.Prompt,
		"expected_output": taskPacket.ExpectedOutput,
		"context":         contextData,
		"dependencies":    task["dependencies"],
		"event":           "task_available",
	})
	if err != nil {
		h.failTask(ctx, taskID, modelID, branchName, "execution_error")
		return err
	}

	// Compact session for context history
	h.factory.Compact(ctx, result, taskID)

	// Security scan
	cleanOutput, leaks := h.leakDetector.Scan(result.Output)
	if len(leaks) > 0 {
		log.Printf("[TaskAvailable] %d leaks redacted in %s", len(leaks), truncateID(taskID))
	}

	duration := time.Since(runStart)
	tokensIn := result.TokensIn
	tokensOut := result.TokensOut
	totalTokens := tokensIn + tokensOut

	// Parse output
	taskOutput, parseErr := runtime.ParseTaskRunnerOutput(result.Output)
	var files []runtime.File
	var summary string
	if parseErr != nil {
		summary = cleanOutput
	} else {
		files = taskOutput.Files
		summary = taskOutput.Summary
	}

	// Build execution result for supervisor review
	executionResult := map[string]any{
		"files": func() []map[string]any {
			result := make([]map[string]any, len(files))
			for i, f := range files {
				result[i] = map[string]any{
					"path":    f.Path,
					"content": f.Content,
				}
			}
			return result
		}(),
		"summary":    summary,
		"raw_output": cleanOutput,
		"status":     "complete",
	}

	// Commit output to branch
	h.commitOutput(ctx, branchName, files, cleanOutput, summary, modelID, taskID, duration.Seconds())

	// Record task run with execution result
	costs := h.calculateCosts(ctx, modelID, tokensIn, tokensOut)
	h.database.RPC(ctx, "create_task_run", map[string]any{
		"p_task_id":                       taskID,
		"p_model_id":                      modelID,
		"p_courier":                       connectorID,
		"p_platform":                      h.deriveRoutingFlag(h.cfg.GetConnector(connectorID)),
		"p_status":                        "success",
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
		"p_result":                         executionResult,
	})

	// Atomically transition to review
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "review",
	})

	h.recordSuccess(ctx, taskID, modelID, taskCategory, duration.Seconds(), totalTokens)
	h.deleteCheckpoint(ctx, taskID)

	log.Printf("[TaskAvailable] Task %s → review", truncateID(taskID))
	return nil
}

// ============================================================================
// SUPERVISOR REVIEW: review → testing (approved) OR available (fail)
// ============================================================================

func (h *TaskHandler) handleTaskReview(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		return
	}

	taskID := getString(task, "id")
	taskType := getString(task, "type")
	taskNumber := getString(task, "task_number")
	modelID := getString(task, "assigned_to")
	sliceID := getStringOr(task, "slice_id", "review")

	if taskID == "" {
		return
	}

	branchName := h.buildBranchName(sliceID, taskNumber, taskID)

	// Claim for review
	reviewerID := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": reviewerID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskReview] Task %s already being reviewed", truncateID(taskID))
		return
	}

	// Route to supervisor
	routingResult, _ := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "supervisor",
		TaskID:   taskID,
		TaskType: taskType,
	})
	if routingResult == nil {
		log.Printf("[TaskReview] No supervisor for %s", truncateID(taskID))
		return
	}

	session, err := h.factory.CreateWithConnector(ctx, "supervisor", taskType, routingResult.DestinationID)
	if err != nil {
		log.Printf("[TaskReview] Session error for %s: %v", truncateID(taskID), err)
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.DestinationID, func() error {
		// Get context for review
		taskPacket, _ := h.database.GetTaskPacket(ctx, taskID)
		taskRunData, _ := h.database.REST(ctx, "GET", fmt.Sprintf("task_runs?task_id=eq.%s&order=created_at.desc&limit=1", taskID), nil)
		var taskRuns []map[string]any
		var latestRun map[string]any
		if err := json.Unmarshal(taskRunData, &taskRuns); err == nil && len(taskRuns) > 0 {
			latestRun = taskRuns[0]
		}

		result, err := session.Run(ctx, map[string]any{
			"task":        task,
			"event":       "task_review",
			"task_packet": taskPacket,
			"task_run":    latestRun,
		})
		if err != nil {
			log.Printf("[TaskReview] Session failed for %s: %v", truncateID(taskID), err)
			return err
		}

		// Compact session for context history
		h.factory.Compact(ctx, result, taskID)

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[TaskReview] Parse error for %s: %v, retrying...", truncateID(taskID), parseErr)

			// Retry with explicit JSON enforcement
			retrySession, retryErr := h.factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.DestinationID)
			if retryErr == nil {
				retryResult, retryRunErr := retrySession.Run(ctx, map[string]any{
					"previous_output": result.Output,
					"parse_error":     parseErr.Error(),
					"instruction":     "Your previous response was not valid JSON. Parse the previous output and respond with ONLY the JSON object. No markdown. No explanations.",
				})
				if retryRunErr == nil {
					decision, parseErr = runtime.ParseSupervisorDecision(retryResult.Output)
				}
			}

			if parseErr != nil {
				log.Printf("[TaskReview] Retry also failed to parse for %s: %v", truncateID(taskID), parseErr)
				// Set to failed status instead of leaving in limbo
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "failed",
					"p_failure_reason": fmt.Sprintf("JSON parse failed after retry: %v", parseErr),
				})
				return nil
			}
			log.Printf("[TaskReview] Retry succeeded for %s", truncateID(taskID))
		}

		log.Printf("[TaskReview] Task %s decision: %s", truncateID(taskID), decision.Decision)

		switch decision.Decision {
		case "approved", "pass":
			// Approved → testing
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "testing",
			})
			log.Printf("[TaskReview] Task %s → testing", truncateID(taskID))

		case "fail", "failed":
			// Failed → back to available with notes
			failureReason := "supervisor_reject"
			if len(decision.Issues) > 0 {
				failureReason = decision.Issues[0].Description
			}
			h.recordIssues(ctx, taskID, modelID, taskType, decision.Issues)
			h.recordFailure(ctx, modelID, taskID, "supervisor_fail")
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": failureReason,
			})
			log.Printf("[TaskReview] Task %s failed: %s → available", truncateID(taskID), failureReason)

		case "needs_revision":
			// Needs revision → back to available for re-execution
			h.recordIssues(ctx, taskID, modelID, taskType, decision.Issues)
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "available",
			})
			log.Printf("[TaskReview] Task %s needs revision → available", truncateID(taskID))

		case "council_review":
			// Complex → escalate to council
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "council_review",
			})
			log.Printf("[TaskReview] Task %s → council_review", truncateID(taskID))

		case "reroute":
			// Reroute → back to available for different assignment
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "available",
			})
			log.Printf("[TaskReview] Task %s reroute → available", truncateID(taskID))

		default:
			// Unknown decision → human review
			log.Printf("[TaskReview] Unknown decision '%s' for %s → awaiting_human", decision.Decision, truncateID(taskID))
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "awaiting_human",
			})
		}

		return nil
	})
	if err != nil {
		log.Printf("[TaskReview] Submit error: %v", err)
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func (h *TaskHandler) failTask(ctx context.Context, taskID, modelID, branchName, reason string) {
	h.recordFailure(ctx, modelID, taskID, reason)
	h.git.DeleteBranch(ctx, branchName)
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":        taskID,
		"p_new_status":     "available",
		"p_failure_reason": reason,
	})
	log.Printf("[TaskHandler] Task %s failed: %s → available", truncateID(taskID), reason)
}

func (h *TaskHandler) buildBranchName(sliceID, taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}

	// Use slice-based naming: task/{slice_id}/{task_number}
	// Example: task/general/T001, task/auth/T002
	if sliceID != "" && taskNumber != "" {
		return prefix + sliceID + "/" + taskNumber
	}

	// Fallback to task number only (for backwards compatibility)
	if taskNumber != "" {
		return prefix + taskNumber
	}

	return prefix + truncateID(taskID)
}

func (h *TaskHandler) getTargetBranch(sliceID string) string {
	if sliceID == "" || sliceID == "default" || sliceID == "review" || sliceID == "testing" {
		sliceID = "general"
	}
	return "TEST_MODULES/" + sliceID
}

func (h *TaskHandler) deriveRoutingFlag(conn *runtime.ConnectorConfig) string {
	if conn == nil {
		return "internal"
	}
	switch conn.Type {
	case "mcp":
		return "mcp"
	case "web":
		return "web"
	default:
		return "internal"
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
		fileMaps := make([]any, len(files))
		for i, f := range files {
			fileMaps[i] = map[string]any{"path": f.Path, "content": f.Content}
		}
		outputMap["files"] = fileMaps
	}
	return h.git.CommitOutput(ctx, branchName, outputMap)
}

func (h *TaskHandler) recordSuccess(ctx context.Context, taskID, modelID, taskType string, durationSeconds float64, tokensUsed int) {
	if modelID == "" {
		return
	}
	h.database.RPC(ctx, "record_model_success", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_duration_seconds": durationSeconds,
		"p_tokens_used":      tokensUsed,
	})
}

func (h *TaskHandler) recordFailure(ctx context.Context, modelID, taskID, failureType string) {
	if modelID == "" {
		return
	}
	h.database.RPC(ctx, "record_model_failure", map[string]any{
		"p_model_id":         modelID,
		"p_task_id":          taskID,
		"p_failure_type":     failureType,
		"p_failure_category": runtime.CategorizeFailure(failureType),
	})
}

func (h *TaskHandler) recordIssues(ctx context.Context, taskID, modelID, taskType string, issues []runtime.Issue) {
	for _, issue := range issues {
		h.database.RPC(ctx, "record_failure", map[string]any{
			"p_task_id":          taskID,
			"p_failure_type":     issue.Type,
			"p_failure_category": runtime.CategorizeFailure(issue.Type),
			"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
			"p_model_id":         modelID,
			"p_task_type":        taskType,
		})
	}
}

func (h *TaskHandler) saveCheckpoint(ctx context.Context, taskID, step string, progress int, output string, files []string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	h.database.RPC(ctx, "save_checkpoint", map[string]any{
		"p_task_id": taskID, "p_step": step, "p_progress": progress, "p_output": output, "p_files": files,
	})
}

func (h *TaskHandler) deleteCheckpoint(ctx context.Context, taskID string) {
	if !h.cfg.GetCoreConfig().IsCheckpointEnabled() {
		return
	}
	h.database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
}

type costResult struct{ Theoretical, Actual, Savings float64 }

func (h *TaskHandler) calculateCosts(ctx context.Context, modelID string, tokensIn, tokensOut int) costResult {
	result, err := h.database.RPC(ctx, "calculate_run_costs", map[string]any{
		"p_model_id": modelID, "p_tokens_in": tokensIn, "p_tokens_out": tokensOut, "p_courier_cost_usd": 0,
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
	return costResult{Theoretical: costs.TheoreticalCostUsd, Actual: costs.ActualCostUsd, Savings: costs.SavingsUsd}
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
