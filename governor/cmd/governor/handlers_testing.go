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
	selectDestination := func(agentID, taskID, taskType string) string {
		result, err := connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
			AgentID:  agentID,
			TaskID:   taskID,
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

	router.On(runtime.EventTestResults, func(event runtime.Event) {
		var testResult map[string]any
		if err := json.Unmarshal(event.Record, &testResult); err != nil {
			return
		}

		resultID, _ := testResult["id"].(string)
		taskID, _ := testResult["task_id"].(string)
		taskNumber, _ := testResult["task_number"].(string)

		processingBy := fmt.Sprintf("test_results:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "test_results",
			"p_id":            resultID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventTestResults] Test result %s already being processed or claim failed", truncateID(resultID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTestResults] Test result %s already being processed", truncateID(resultID))
			return
		}

		destID := selectDestination("supervisor", taskID, "test_review")
		if destID == "" {
			log.Printf("[EventTestResults] No destination available for task %s", truncateID(taskID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "testing", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})

			result, err := session.Run(ctx, map[string]any{"test_result": testResult, "event": "test_results"})
			if err != nil {
				return err
			}

			testOutput, parseErr := runtime.ParseTestResults(result.Output)
			if parseErr != nil {
				log.Printf("[EventTestResults] Failed to parse test output: %v", parseErr)
				log.Printf("[EventTestResults] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventTestResults] Task %s test outcome: %s, next: %s", truncateID(taskID), testOutput.TestOutcome, testOutput.NextAction)

			switch testOutput.NextAction {
			case "final_merge":
				branchName := fmt.Sprintf("task/%s", taskNumber)
				sliceID, _ := testResult["slice_id"].(string)
				if sliceID == "" {
					sliceID = "default"
				}
				targetBranch := fmt.Sprintf("module/%s", sliceID)

				if err := git.MergeBranch(ctx, branchName, targetBranch); err != nil {
					log.Printf("[EventTestResults] Failed to merge %s to %s: %v", branchName, targetBranch, err)
					return nil
				}

				if err := git.DeleteBranch(ctx, branchName); err != nil {
					log.Printf("[EventTestResults] Failed to delete branch %s: %v", branchName, err)
				}

				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "complete",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to update task status to complete: %v", err)
				}

				_, err = database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
					"p_completed_task_id": taskID,
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to unlock dependents: %v", err)
				}

			case "return_for_fix":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to reset task for fix: %v", err)
				}

			case "await_human_approval":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "awaiting_human",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to set task awaiting_human: %v", err)
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			log.Printf("[EventTestResults] Failed to submit to pool: %v", err)
		}
	})
}
