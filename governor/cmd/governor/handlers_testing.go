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

type TestingHandler struct {
	database   *db.DB
	factory    *runtime.SessionFactory
	pool       *runtime.AgentPool
	connRouter *runtime.Router
	git        *gitree.Gitree
	cfg        *runtime.Config
}

func NewTestingHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	cfg *runtime.Config,
) *TestingHandler {
	return &TestingHandler{
		database:   database,
		factory:    factory,
		pool:       pool,
		connRouter: connRouter,
		git:        git,
		cfg:        cfg,
	}
}

func (h *TestingHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventTaskTesting, h.handleTaskTesting)
	router.On(runtime.EventTestResults, h.handleTestResults)
}

func (h *TestingHandler) handleTaskTesting(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[TaskTesting] Failed to parse event: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	taskType := getString(task, "type")
	modelID := getString(task, "assigned_to")
	sliceID := getStringOr(task, "slice_id", "testing")

	if taskID == "" {
		return
	}

	processingBy := fmt.Sprintf("tester:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskTesting] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "tester",
		TaskID:   taskID,
		TaskType: taskType,
	})
	if err != nil || routingResult == nil {
		log.Printf("[TaskTesting] No destination for task %s", truncateID(taskID))
		h.recordFailure(ctx, modelID, taskID, "no_destination")
		return
	}

	branchName := h.buildBranchName(taskNumber, taskID)

	session, err := h.factory.CreateWithContext(ctx, "tester", taskType)
	if err != nil {
		log.Printf("[TaskTesting] Failed to create session for %s: %v", truncateID(taskID), err)
		h.recordFailure(ctx, modelID, taskID, "session_create_failed")
		return
	}

	err = h.pool.SubmitWithDestination(ctx, sliceID, routingResult.DestinationID, func() error {
		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{
			"task":        task,
			"branch_name": branchName,
			"repo_path":   h.cfg.GetRepoPath(),
			"event":       "task_testing",
		})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[TaskTesting] Session failed for %s: %v", truncateID(taskID), sessionErr)
			h.recordFailure(ctx, modelID, taskID, "session_error")
			h.recordFailureNotes(ctx, taskID, fmt.Sprintf("session_error: %v", sessionErr))
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			return sessionErr
		}

		testOutput, parseErr := runtime.ParseTestResults(result.Output)
		if parseErr != nil {
			log.Printf("[TaskTesting] Failed to parse test output for %s: %v", truncateID(taskID), parseErr)
			h.recordFailure(ctx, modelID, taskID, "parse_error")
			h.recordFailureNotes(ctx, taskID, fmt.Sprintf("parse_error: %v", parseErr))
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			return nil
		}

		log.Printf("[TaskTesting] Task %s test outcome: %s, next: %s",
			truncateID(taskID), testOutput.TestOutcome, testOutput.NextAction)

		switch testOutput.TestOutcome {
		case "pass", "passed", "success":
			h.recordSuccess(ctx, routingResult.ModelID, taskType, duration, result.TokensIn+result.TokensOut)
			_, err := h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "approved",
			})
			if err != nil {
				log.Printf("[TaskTesting] Failed to update status: %v", err)
			}

		case "fail", "failed":
			failureDetail := testOutput.NextAction
			h.recordFailure(ctx, routingResult.ModelID, taskID, "test_failed")
			h.recordFailureNotes(ctx, taskID, fmt.Sprintf("test_failed: %s", failureDetail))

			// Create learning rule from test failure
			_, _ = h.database.RPC(ctx, "create_tester_rule", map[string]any{
				"p_applies_to":      taskType,
				"p_test_type":       "automated",
				"p_test_command":    "watch_for_pattern",
				"p_source":          "test_failure",
				"p_trigger_pattern": failureDetail,
				"p_priority":        5,
				"p_source_task_id":  taskID,
				"p_source_details":  map[string]any{"test_outcome": testOutput.TestOutcome},
			})
			log.Printf("[TaskTesting] Created tester rule for task %s", truncateID(taskID))

			_, err := h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			if err != nil {
				log.Printf("[TaskTesting] Failed to reset task: %v", err)
			}

		default:
			if testOutput.NextAction == "return_for_fix" {
				h.recordFailure(ctx, routingResult.ModelID, taskID, "test_needs_fix")
				h.recordFailureNotes(ctx, taskID, fmt.Sprintf("test_needs_fix: %s", testOutput.NextAction))

				// Create learning rule from test failure
				_, _ = h.database.RPC(ctx, "create_tester_rule", map[string]any{
					"p_applies_to":      taskType,
					"p_test_type":       "automated",
					"p_test_command":    "watch_for_pattern",
					"p_source":          "test_needs_fix",
					"p_trigger_pattern": testOutput.NextAction,
					"p_priority":        5,
					"p_source_task_id":  taskID,
					"p_source_details":  map[string]any{"test_outcome": testOutput.TestOutcome},
				})
				log.Printf("[TaskTesting] Created tester rule for task %s (needs fix)", truncateID(taskID))

				_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
			} else if testOutput.NextAction == "await_human_approval" {
				_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "approval",
				})
			} else {
				h.recordFailure(ctx, routingResult.ModelID, taskID, "unknown_test_outcome")
				h.recordFailureNotes(ctx, taskID, fmt.Sprintf("unknown_test_outcome: %s, next: %s", testOutput.TestOutcome, testOutput.NextAction))
				_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("[TaskTesting] Failed to submit: %v", err)
	}
}

func (h *TestingHandler) handleTestResults(event runtime.Event) {
	ctx := context.Background()

	var testResult map[string]any
	if err := json.Unmarshal(event.Record, &testResult); err != nil {
		log.Printf("[TestResults] Failed to parse event: %v", err)
		return
	}

	resultID := getString(testResult, "id")
	taskID := getString(testResult, "task_id")
	taskNumber := getString(testResult, "task_number")
	testOutcome := getString(testResult, "test_outcome")
	nextAction := getString(testResult, "next_action")

	if resultID == "" {
		return
	}

	processingBy := fmt.Sprintf("test_results:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "test_results",
		"p_id":            resultID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TestResults] Result %s already being processed", truncateID(resultID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "test_results",
		"p_id":    resultID,
	})

	log.Printf("[TestResults] Task %s outcome: %s, next: %s",
		truncateID(taskID), testOutcome, nextAction)

	switch nextAction {
	case "final_merge":
		branchName := fmt.Sprintf("task/%s", taskNumber)
		sliceID := getStringOr(testResult, "slice_id", "default")
		targetBranch := fmt.Sprintf("module/%s", sliceID)

		if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
			log.Printf("[TestResults] Merge failed %s -> %s: %v", branchName, targetBranch, err)
			_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "error",
			})
			return
		}

		h.git.DeleteBranch(ctx, branchName)

		_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "merged",
		})
		_, _ = h.database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
			"p_completed_task_id": taskID,
		})
		log.Printf("[TestResults] Task %s merged to %s", truncateID(taskID), targetBranch)

	case "return_for_fix":
		_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "available",
		})

	case "await_human_approval":
		_, _ = h.database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "approval",
		})
	}
}

func (h *TestingHandler) buildBranchName(taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}
	if taskNumber != "" {
		return prefix + taskNumber
	}
	return prefix + truncateID(taskID)
}

func (h *TestingHandler) recordSuccess(ctx context.Context, modelID, taskType string, durationSeconds float64, tokensUsed int) {
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

func (h *TestingHandler) recordFailure(ctx context.Context, modelID, taskID, failureType string) {
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

func (h *TestingHandler) recordFailureNotes(ctx context.Context, taskID, reason string) {
	if reason == "" {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = h.database.RPC(ctx, "append_failure_notes", map[string]any{
		"p_task_id": taskID,
		"p_notes":   fmt.Sprintf("%s (%s)", reason, timestamp),
	})
}

func setupTestingHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
) {
	handler := NewTestingHandler(database, factory, pool, connRouter, git, cfg)
	handler.Register(router)
}
