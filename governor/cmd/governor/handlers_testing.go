package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
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
}

// ============================================================================
// TESTING: testing → merged (approved) OR available (fail)
// ============================================================================

func (h *TestingHandler) handleTaskTesting(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[Testing] Parse error: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	taskType := getString(task, "type")
	sliceID := getStringOr(task, "slice_id", "testing")

	if taskID == "" {
		return
	}

	branchName := h.buildBranchName(taskNumber, taskID)

	// Claim for testing
	testerID := fmt.Sprintf("tester:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": testerID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[Testing] Task %s already being tested", truncateID(taskID))
		return
	}

	// Route to tester
	routingResult, _ := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "tester",
		TaskID:   taskID,
		TaskType: taskType,
	})
	if routingResult == nil {
		log.Printf("[Testing] No tester for %s", truncateID(taskID))
		return
	}

	session, err := h.factory.CreateWithContext(ctx, "tester", taskType)
	if err != nil {
		log.Printf("[Testing] Session error for %s: %v", truncateID(taskID), err)
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
			log.Printf("[Testing] Session failed for %s: %v", truncateID(taskID), sessionErr)
			return sessionErr
		}

		testOutput, parseErr := runtime.ParseTestResults(result.Output)
		if parseErr != nil {
			log.Printf("[Testing] Parse error for %s: %v", truncateID(taskID), parseErr)
			return nil
		}

		log.Printf("[Testing] Task %s outcome: %s", truncateID(taskID), testOutput.TestOutcome)

		switch testOutput.TestOutcome {
		case "approved":
			// Tests passed → merge to module
			targetBranch := h.getTargetBranch(sliceID)
			if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
				log.Printf("[Testing] Merge failed for %s: %v", branchName, err)
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "approval",
					"p_failure_reason": "merge_failed",
				})
				recordModelFailure(ctx, h.database, routingResult.ModelID, taskID, "merge_failed")
			} else {
				// Success!
				h.git.DeleteBranch(ctx, branchName)
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":    taskID,
					"p_new_status": "merged",
				})
				h.database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
					"p_completed_task_id": taskID,
				})
				recordModelSuccess(ctx, h.database, routingResult.ModelID, taskType, duration)
				log.Printf("[Testing] Task %s → merged to %s", truncateID(taskID), targetBranch)
			}

		case "fail":
			// Tests failed → back to available with notes
			failureReason := testOutput.NextAction
			if failureReason == "" {
				failureReason = "test_failed"
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "available",
				"p_failure_reason": "test_fail: " + failureReason,
			})
			recordModelFailure(ctx, h.database, routingResult.ModelID, taskID, "test_failed")
			log.Printf("[Testing] Task %s failed: %s → available", truncateID(taskID), failureReason)

		default:
			if testOutput.NextAction == "await_human_approval" {
				// UI/UX task needs human eyes
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":    taskID,
					"p_new_status": "awaiting_human",
				})
				log.Printf("[Testing] Task %s → awaiting_human", truncateID(taskID))
			} else {
				log.Printf("[Testing] Unknown outcome '%s' for %s", testOutput.TestOutcome, truncateID(taskID))
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("[Testing] Submit error: %v", err)
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

func (h *TestingHandler) getTargetBranch(sliceID string) string {
	if sliceID != "" && sliceID != "default" && sliceID != "testing" && sliceID != "review" {
		moduleBranch := "module/" + sliceID
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", moduleBranch)
		cmd.Dir = h.cfg.GetRepoPath()
		if output, err := cmd.Output(); err == nil && len(output) > 0 {
			return moduleBranch
		}
	}
	return h.cfg.GetDefaultMergeTarget()
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
