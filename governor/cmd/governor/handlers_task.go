package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
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
	usageTracker  *runtime.UsageTracker
	worktreeMgr   *gitree.WorktreeManager
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
	usageTracker *runtime.UsageTracker,
	worktreeMgr *gitree.WorktreeManager,
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
		usageTracker:  usageTracker,
		worktreeMgr:   worktreeMgr,
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
	taskCategory := getString(task, "category")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// DEPENDENCY GATE: Block tasks whose dependencies aren't complete
	if deps, ok := task["dependencies"].([]any); ok && len(deps) > 0 {
		allComplete := true
		for _, dep := range deps {
			depNum, _ := dep.(string)
			if depNum == "" {
				continue
			}
			depRows, err := h.database.Query(ctx, "tasks", map[string]any{
				"task_number": depNum,
				"select":      "id,status",
			})
			if err != nil || len(depRows) == 0 {
				log.Printf("[TaskAvailable] Task %s: dependency %s not found, blocking", taskNumber, depNum)
				allComplete = false
				break
			}
			var depTasks []map[string]any
			if json.Unmarshal(depRows, &depTasks) == nil && len(depTasks) > 0 {
				depStatus, _ := depTasks[0]["status"].(string)
				if depStatus != "merged" && depStatus != "complete" && depStatus != "completed" {
					log.Printf("[TaskAvailable] Task %s: dependency %s not complete (status=%s), reverting to pending", taskNumber, depNum, depStatus)
					allComplete = false
					break
				}
			}
		}
		if !allComplete {
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "pending",
			})
			return
		}
		log.Printf("[TaskAvailable] Task %s: all dependencies complete, proceeding", taskNumber)
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

	// On retry: append supervisor revision notes to prompt so executor fixes issues
	// failure_notes being non-empty means a previous supervisor review rejected this task
	if failureNotes, ok := task["failure_notes"].(string); ok && failureNotes != "" {
		taskPacket.Prompt += "\n\n## PREVIOUS ATTEMPT FEEDBACK (fix these issues)\n" + failureNotes
		log.Printf("[TaskAvailable] Task %s retry: appended revision notes (%d bytes)", truncateID(taskID), len(failureNotes))
	}

	// Route to model with cascade retry — same pattern as planner/supervisor
	var routingResult *runtime.RoutingResult
	var failedModels []string
	var modelID, connectorID, routingFlag string
	var connConfig *runtime.ConnectorConfig
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		var routeErr error
		if attempt > 0 {
			log.Printf("[TaskAvailable] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "task_runner",
			TaskType:      taskCategory,
			RoutingFlag:   "internal",
			ExcludeModels: failedModels,
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[TaskAvailable] No routing for task %s (attempt %d)", truncateID(taskID), attempt+1)
			// All models in cooldown or unavailable — stop, don't retry
			return
		}

		modelID = routingResult.ModelID
		connectorID = routingResult.ConnectorID
		connConfig = h.cfg.GetConnector(connectorID)
		routingFlag = h.deriveRoutingFlag(connConfig)

		// Check pool capacity
		if !h.pool.HasCapacity(sliceID, connectorID) {
			log.Printf("[TaskAvailable] Task %s pending - no capacity (slice=%s, dest=%s)", truncateID(taskID), sliceID, connectorID)
			failedModels = append(failedModels, modelID)
			continue
		}

		// Claim task
		workerID := fmt.Sprintf("executor:%s:%d", modelID, time.Now().UnixNano())
		claimed, err := h.database.RPC(ctx, "claim_task", map[string]any{
			"p_task_id":        taskID,
			"p_worker_id":      workerID,
			"p_model_id":       modelID,
			"p_routing_flag":   routingFlag,
			"p_routing_reason": fmt.Sprintf("Routed via %s (attempt %d)", connectorID, attempt+1),
		})
		if err != nil || !parseBool(claimed) {
			log.Printf("[TaskAvailable] Task %s claim failed (model=%s): err=%v", truncateID(taskID), modelID, err)
			failedModels = append(failedModels, modelID)
			continue
		}

		// Successfully claimed
		log.Printf("[TaskAvailable] Task %s claimed by %s via %s", truncateID(taskID), modelID, connectorID)
		break
	}

	if routingResult == nil {
		log.Printf("[TaskAvailable] No routing available for task %s after %d attempts", truncateID(taskID), maxRetries)
		return
	}

	// Setup branch
	branchName := h.buildBranchName(sliceID, taskNumber, taskID)
	attempts := 0
	if v, ok := task["attempts"].(float64); ok {
		attempts = int(v)
	}

	var worktreePath string

	if h.worktreeMgr != nil {
		// Worktree mode: isolated checkout per task
		existingPath := h.worktreeMgr.GetWorktreePath(taskID)
		if attempts > 0 && existingPath != "" {
			// Check if worktree directory still exists (preserved after test failure)
			if _, err := os.Stat(existingPath); err == nil {
				// Reuse existing worktree and branch — iterative fix, not fresh start
				worktreePath = existingPath
				log.Printf("[TaskAvailable] Task %s retry: reusing existing worktree at %s", truncateID(taskID), worktreePath)
			} else {
				// Worktree gone — create fresh
				wtInfo, err := h.worktreeMgr.CreateWorktree(ctx, taskID, branchName)
				if err != nil {
					log.Printf("[TaskAvailable] Worktree create failed for %s: %v, falling back to branch-only", truncateID(taskID), err)
					h.git.CreateBranch(ctx, branchName)
				} else {
					worktreePath = wtInfo.Path
					log.Printf("[TaskAvailable] Worktree created for %s at %s", truncateID(taskID), worktreePath)
				}
			}
		} else {
			// First attempt: create fresh worktree
			wtInfo, err := h.worktreeMgr.CreateWorktree(ctx, taskID, branchName)
			if err != nil {
				log.Printf("[TaskAvailable] Worktree create failed for %s: %v, falling back to branch-only", truncateID(taskID), err)
				h.git.CreateBranch(ctx, branchName)
			} else {
				worktreePath = wtInfo.Path
				log.Printf("[TaskAvailable] Worktree created for %s at %s", truncateID(taskID), worktreePath)
			}
		}
	} else {
		// Legacy mode: single directory, branch checkout
		if attempts > 0 {
			h.git.DeleteBranch(ctx, branchName)
		}
		h.git.CreateBranch(ctx, branchName)
	}

	h.database.RPC(ctx, "update_task_branch", map[string]any{
		"p_task_id":     taskID,
		"p_branch_name": branchName,
	})

	h.saveCheckpoint(ctx, taskID, "execution_start", 0, "", nil)
	runStart := time.Now()

	// Execute
	err = h.pool.SubmitWithDestination(ctx, sliceID, connectorID, func() error {
		return h.executeTask(ctx, task, taskPacket, taskID, taskNumber, modelID, connectorID, branchName, taskCategory, worktreePath, runStart)
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
	taskID, taskNumber, modelID, connectorID, branchName, taskCategory, worktreePath string,
	runStart time.Time,
) error {

	var contextData map[string]any
	if len(taskPacket.Context) > 0 {
		json.Unmarshal(taskPacket.Context, &contextData)
	}

	session, err := h.factory.CreateWithConnector(ctx, "task_runner", taskCategory, connectorID)
	if err != nil {
		log.Printf("[TaskHandler] Session create failed for task %s: %v", truncateID(taskID), err)
		h.failTask(ctx, taskID, modelID, branchName, "session_create_failed")
		return err
	}

	// Build session params -- include worktree path if available
	sessionParams := map[string]any{
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
	}
	if worktreePath != "" {
		sessionParams["worktree_path"] = worktreePath
		sessionParams["repo_path"] = worktreePath
	}

	// Execute with timeout — prevent hung workers from locking tasks forever
	execCtx, execCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer execCancel()

	result, err := session.Run(execCtx, sessionParams)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			log.Printf("[TaskHandler] TIMEOUT for task %s after 5m", truncateID(taskID))
			h.failTask(ctx, taskID, modelID, branchName, "execution_timeout")
			return fmt.Errorf("execution timeout")
		}
		// Check for rate limit (HTTP 429)
		if h.usageTracker != nil && modelID != "" {
			if isRateLimitError(err) {
				log.Printf("[TaskAvailable] Rate limit hit for model %s, recording cooldown", modelID)
				h.usageTracker.RecordRateLimit(ctx, modelID)
			}
			h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, time.Since(runStart).Seconds(), false)
		}
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

	// Record usage with tracker
	if h.usageTracker != nil {
		if err := h.usageTracker.RecordUsage(ctx, modelID, tokensIn, tokensOut); err != nil {
			log.Printf("[TaskAvailable] UsageTracker RecordUsage error for %s: %v", modelID, err)
		}
	}

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

	// Deduct cost from model's credit_remaining_usd (if model has credit tracking)
	if costs.Actual > 0 {
		h.database.RPC(ctx, "deduct_model_credit", map[string]any{
			"p_model_id": modelID,
			"p_cost_usd": costs.Actual,
		})
	}

	// Atomically transition to review
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "review",
	})

	h.recordSuccess(ctx, taskID, modelID, taskCategory, duration.Seconds(), totalTokens)

	// Record successful completion with tracker
	if h.usageTracker != nil {
		h.usageTracker.RecordCompletion(ctx, modelID, taskCategory, duration.Seconds(), true)
	}

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

	// Route to supervisor with cascade retry — same pattern as planner/executor
	var routingResult *runtime.RoutingResult
	var failedModels []string
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		var routeErr error
		if attempt > 0 {
			log.Printf("[TaskReview] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "supervisor",
			TaskType:      taskType,
			RoutingFlag:   "internal",
			ExcludeModels: failedModels,
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[TaskReview] No supervisor for task %s (attempt %d)", truncateID(taskID), attempt+1)
			// All models in cooldown — stop, don't retry
			return
		}
		break // routing found
	}

	if routingResult == nil {
		log.Printf("[TaskReview] No routing available for task %s after %d attempts", truncateID(taskID), maxRetries)
		return
	}

	session, err := h.factory.CreateWithConnector(ctx, "supervisor", taskType, routingResult.ConnectorID)
	if err != nil {
		log.Printf("[TaskReview] Session error for %s: %v", truncateID(taskID), err)
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.ConnectorID, func() error {
		// Get context for review
		taskPacket, _ := h.database.GetTaskPacket(ctx, taskID)
		taskRunData, _ := h.database.REST(ctx, "GET", fmt.Sprintf("task_runs?task_id=eq.%s&order=created_at.desc&limit=1", taskID), nil)
		var taskRuns []map[string]any
		var latestRun map[string]any
		if err := json.Unmarshal(taskRunData, &taskRuns); err == nil && len(taskRuns) > 0 {
			latestRun = taskRuns[0]
		}

		// Supervisor with timeout — prevent hung reviews from locking tasks
		reviewCtx, reviewCancel := context.WithTimeout(ctx, 2*time.Minute)
		defer reviewCancel()

		result, err := session.Run(reviewCtx, map[string]any{
			"task":        task,
			"event":       "task_review",
			"task_packet": taskPacket,
			"task_run":    latestRun,
		})
		if err != nil {
			if reviewCtx.Err() == context.DeadlineExceeded {
				log.Printf("[TaskReview] TIMEOUT reviewing task %s after 2m", truncateID(taskID))
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "review",
					"p_failure_reason": "supervisor_review_timeout",
				})
				return nil
			}
			log.Printf("[TaskReview] Session failed for %s: %v", truncateID(taskID), err)
			return err
		}

		// Compact session for context history
		h.factory.Compact(ctx, result, taskID)

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[TaskReview] Parse error for %s: %v, retrying...", truncateID(taskID), parseErr)

			// Retry with explicit JSON enforcement
			retrySession, retryErr := h.factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)
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
			// Failed → back to available with full failure context
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "unknown"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" && len(decision.Issues) > 0 {
				failureDetail = decision.Issues[0].Description
			}

			// Build full failure notes including ReturnFeedback
			failureNotes := fmt.Sprintf("[%s] %s", failureClass, failureDetail)
			if decision.ReturnFeedback.Summary != "" {
				failureNotes += "\n\n" + decision.ReturnFeedback.Summary
			}
			if len(decision.ReturnFeedback.SpecificIssues) > 0 {
				failureNotes += "\n\nIssues:"
				for _, issue := range decision.ReturnFeedback.SpecificIssues {
					failureNotes += "\n- " + issue
				}
			}

			h.recordFailure(ctx, modelID, taskID, failureClass)
			h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
			h.recordEvent(ctx, "failure", taskID, modelID, failureClass, map[string]any{
				"class": failureClass, "detail": failureDetail,
			})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": failureNotes,
			})
			log.Printf("[TaskReview] Task %s failed: %s (%s) → available", truncateID(taskID), failureClass, failureDetail)

		case "needs_revision":
			// Needs revision → back to available with FULL feedback for retry
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "needs_revision"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" && len(decision.Issues) > 0 {
				failureDetail = decision.Issues[0].Description
			}

			// Build structured revision feedback from supervisor's ReturnFeedback
			revisionNotes := fmt.Sprintf("[%s] %s", failureClass, failureDetail)
			if decision.ReturnFeedback.Summary != "" {
				revisionNotes += "\n\n" + decision.ReturnFeedback.Summary
			}
			if len(decision.ReturnFeedback.SpecificIssues) > 0 {
				revisionNotes += "\n\nIssues to fix:"
				for _, issue := range decision.ReturnFeedback.SpecificIssues {
					revisionNotes += "\n- " + issue
				}
			}
			if len(decision.ReturnFeedback.Suggestions) > 0 {
				revisionNotes += "\n\nSuggestions:"
				for _, s := range decision.ReturnFeedback.Suggestions {
					revisionNotes += "\n- " + s
				}
			}

			h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
			h.recordEvent(ctx, "revision_needed", taskID, modelID, failureClass, map[string]any{
				"class": failureClass, "detail": failureDetail, "revision_notes": revisionNotes,
			})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": revisionNotes,
			})
			log.Printf("[TaskReview] Task %s needs revision: %s (%s) → available", truncateID(taskID), failureClass, failureDetail)

		case "council_review":
			// Complex → escalate to council
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "council_review",
			})
			log.Printf("[TaskReview] Task %s → council_review", truncateID(taskID))

		case "reroute":
			// Reroute → back to available for different assignment
			failureClass := decision.FailureClass
			if failureClass == "" {
				failureClass = "model_limitation"
			}
			failureDetail := decision.FailureDetail
			if failureDetail == "" {
				failureDetail = "Supervisor recommends different model"
			}
			h.recordModelLearning(ctx, modelID, taskType, failureClass, failureDetail)
			h.recordEvent(ctx, "reroute", taskID, modelID, failureClass, map[string]any{
				"class": failureClass, "detail": failureDetail,
			})
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": fmt.Sprintf("[%s] %s", failureClass, failureDetail),
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
	// Clean up worktree if active
	if h.worktreeMgr != nil {
		h.worktreeMgr.RemoveWorktree(ctx, taskID)
	}
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
	// Feed success into model learning for competency tracking
	h.database.RPC(ctx, "update_model_learning", map[string]any{
		"p_model_id":       modelID,
		"p_task_type":      taskType,
		"p_outcome":        "success",
		"p_failure_class":  "",
		"p_failure_category": "",
		"p_failure_detail": "",
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

// recordModelLearning writes structured failure data to models.learned JSONB column
// This builds institutional knowledge: which models excel at what, struggle with what.
// Over time the router uses best_for_task_types / avoid_for_task_types for intelligent routing.
func (h *TaskHandler) recordModelLearning(ctx context.Context, modelID, taskType, failureClass, failureDetail string) {
	if modelID == "" {
		return
	}
	category := runtime.CategorizeFailure(failureClass)

	// Update learned.failure_rate_by_type and learned.avoid_for_task_types
	// The RPC will merge into the existing JSONB
	h.database.RPC(ctx, "update_model_learning", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_outcome":          "failure",
		"p_failure_class":    failureClass,
		"p_failure_category": category,
		"p_failure_detail":   failureDetail,
	})
}

// recordEvent writes to orchestrator_events for the dashboard timeline.
// The dashboard reads: event_type (maps to type), reason (maps to reasonCode),
// details JSONB, task_id, model_id, created_at.
// event_type "failure" marks task quality as "fail" in deriveQualityMap.
func (h *TaskHandler) recordEvent(ctx context.Context, eventType, taskID, modelID, reason string, details map[string]any) {
	eventDetails := details
	if eventDetails == nil {
		eventDetails = map[string]any{}
	}
	eventDetails["model_id"] = modelID

	_, err := h.database.REST(ctx, "POST", "orchestrator_events", map[string]any{
		"event_type": eventType,
		"task_id":    taskID,
		"model_id":   modelID,
		"reason":     reason,
		"details":    eventDetails,
	})
	if err != nil {
		log.Printf("[recordEvent] Failed to write event: %v", err)
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
	result, err := h.database.RPC(ctx, "calc_run_costs", map[string]any{
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

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "quota exceeded")
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
	usageTracker *runtime.UsageTracker,
	worktreeMgr *gitree.WorktreeManager,
) {
	handler := NewTaskHandler(database, factory, pool, connRouter, git, checkpointMgr, leakDetector, cfg, usageTracker, worktreeMgr)
	handler.Register(router)
}
