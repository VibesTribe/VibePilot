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
	selectRouting := func(agentID, taskID, taskType string) *runtime.RoutingResult {
		result, err := connRouter.SelectDestination(ctx, runtime.RoutingRequest{
			AgentID:  agentID,
			TaskID:   taskID,
			TaskType: taskType,
		})
		if err != nil || result == nil {
			log.Printf("[Router] No destination available for agent %s, using fallback", agentID)
			dests := connRouter.GetAvailableConnectors()
			if len(dests) > 0 {
				return &runtime.RoutingResult{DestinationID: dests[0]}
			}
			return nil
		}
		return result
	}

	router.On(runtime.EventTaskAvailable, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			log.Printf("[EventTaskAvailable] Failed to parse task: %v", err)
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		taskNumber, _ := task["task_number"].(string)
		taskCategory, _ := task["category"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "default"
		}

		processingBy := fmt.Sprintf("task_runner:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskAvailable] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskAvailable] Task %s already being processed", truncateID(taskID))
			return
		}

		taskPacket, err := database.GetTaskPacket(ctx, taskID)
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to fetch task packet for %s: %v", truncateID(taskID), err)
			_, _ = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "error",
			})
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		if taskPacket.Prompt == "" {
			log.Printf("[EventTaskAvailable] Task %s has empty prompt packet - cannot execute", truncateID(taskID))
			_, _ = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "error",
			})
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		log.Printf("[EventTaskAvailable] Task %s packet loaded: prompt_len=%d category=%s", truncateID(taskID), len(taskPacket.Prompt), taskCategory)

		taskPrefix := cfg.GetTaskBranchPrefix()
		if taskPrefix == "" {
			taskPrefix = "task/"
		}
		branchName := fmt.Sprintf("%s%s", taskPrefix, taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("%s%s", taskPrefix, truncateID(taskID))
		}

		if err := git.CreateBranch(ctx, branchName); err != nil {
			log.Printf("[EventTaskAvailable] Failed to create branch %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskAvailable] Created branch %s for task %s", branchName, truncateID(taskID))
		}

		routingResult := selectRouting("task_runner", taskID, taskCategory)
		if routingResult == nil {
			routingResult = selectRouting("task_runner", taskID, taskType)
		}
		if routingResult == nil {
			log.Printf("[EventTaskAvailable] No destination available for task %s (category=%s, type=%s)", truncateID(taskID), taskCategory, taskType)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		destID := routingResult.DestinationID
		modelID := routingResult.ModelID
		if modelID == "" {
			modelID = "unknown"
		}

		connConfig := cfg.GetConnector(destID)
		connectorType := "cli"
		if connConfig != nil && connConfig.Type != "" {
			connectorType = connConfig.Type
		}

		_, err = database.RPC(ctx, "update_task_assignment", map[string]any{
			"p_task_id":     taskID,
			"p_status":      "in_progress",
			"p_assigned_to": modelID,
		})
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to update status to in_progress: %v", err)
		} else {
			log.Printf("[EventTaskAvailable] Task %s assigned to model %s via %s", truncateID(taskID), modelID, destID)
		}

		session, err := factory.CreateWithContext(ctx, "task_runner", taskCategory)
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to create task_runner session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		if cfg.GetCoreConfig().IsCheckpointEnabled() {
			_, err := database.RPC(ctx, "save_checkpoint", map[string]any{
				"p_task_id":  taskID,
				"p_step":     "execution",
				"p_progress": 0,
				"p_output":   "",
				"p_files":    []string{},
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Warning: Failed to save initial checkpoint: %v", err)
			} else {
				log.Printf("[EventTaskAvailable] Checkpoint saved for task %s", truncateID(taskID))
			}
		}

		runStartTime := time.Now()
		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})

			var contextData map[string]any
			if len(taskPacket.Context) > 0 {
				json.Unmarshal(taskPacket.Context, &contextData)
			}

			result, err := session.Run(ctx, map[string]any{
				"task_id":         taskID,
				"task_number":     taskNumber,
				"title":           task["title"],
				"type":            taskType,
				"category":        taskCategory,
				"prompt_packet":   taskPacket.Prompt,
				"expected_output": taskPacket.ExpectedOutput,
				"context":         contextData,
				"dependencies":    task["dependencies"],
				"event":           "task_available",
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Task runner failed for %s: %v", truncateID(taskID), err)
				return err
			}

			cleanOutput, leaks := leakDetector.Scan(result.Output)
			if len(leaks) > 0 {
				log.Printf("[EventTaskAvailable] SECURITY: %d leak(s) detected and redacted in task %s output", len(leaks), truncateID(taskID))
			}

			tokensIn := result.TokensIn
			tokensOut := result.TokensOut
			totalTokens := tokensIn + tokensOut

			runnerOutput := map[string]any{
				"raw_output": cleanOutput,
				"model_id":   modelID,
				"task_id":    taskID,
				"duration":   result.Duration.Seconds(),
				"tokens_in":  tokensIn,
				"tokens_out": tokensOut,
			}

			taskOutput, parseErr := runtime.ParseTaskRunnerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventTaskAvailable] Failed to parse runner output: %v", parseErr)
				runnerOutput["parse_error"] = parseErr.Error()
			} else {
				runnerOutput["files"] = taskOutput.Files
				runnerOutput["summary"] = taskOutput.Summary
				runnerOutput["status"] = taskOutput.Status
			}

			if err := git.CommitOutput(ctx, branchName, runnerOutput); err != nil {
				log.Printf("[EventTaskAvailable] Failed to commit output to %s: %v", branchName, err)
			} else {
				log.Printf("[EventTaskAvailable] Committed output to %s", branchName)
			}

			_, err = database.Insert(ctx, "task_runs", map[string]any{
				"task_id":      taskID,
				"model_id":     modelID,
				"courier":      destID,
				"platform":     connectorType,
				"tokens_in":    tokensIn,
				"tokens_out":   tokensOut,
				"tokens_used":  totalTokens,
				"status":       "success",
				"started_at":   runStartTime,
				"completed_at": time.Now(),
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Warning: Failed to create task_run record: %v", err)
			} else {
				log.Printf("[EventTaskAvailable] Created task_run record for task %s: model=%s, tokens=%d", truncateID(taskID), modelID, totalTokens)
			}

			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "review",
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Failed to update status to review: %v", err)
			}

			if cfg.GetCoreConfig().IsCheckpointEnabled() {
				_, delErr := database.RPC(ctx, "delete_checkpoint", map[string]any{
					"p_task_id": taskID,
				})
				if delErr != nil {
					log.Printf("[EventTaskAvailable] Warning: Failed to delete checkpoint: %v", delErr)
				}
			}

			log.Printf("[EventTaskAvailable] Task %s output committed, status=review", truncateID(taskID))
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			log.Printf("[EventTaskAvailable] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskReview, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "review"
		}

		processingBy := fmt.Sprintf("supervisor_review:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskReview] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskReview] Task %s already being processed", truncateID(taskID))
			return
		}

		routingResult := selectRouting("supervisor", taskID, taskType)
		if routingResult == nil {
			log.Printf("[EventTaskReview] No destination available for task %s", truncateID(taskID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}
		destID := routingResult.DestinationID

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
		if err != nil {
			log.Printf("[EventTaskReview] Failed to create supervisor session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})

			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_review"})
			if err != nil {
				return err
			}

			decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
			if parseErr != nil {
				log.Printf("[EventTaskReview] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
				log.Printf("[EventTaskReview] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventTaskReview] Task %s decision: %s, next: %s", truncateID(taskID), decision.Decision, decision.NextAction)

			switch decision.Decision {
			case "pass":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "testing",
				})
				if err != nil {
					log.Printf("[EventTaskReview] Failed to update task status to testing: %v", err)
				}

			case "fail":
				for _, issue := range decision.Issues {
					failureCategory := runtime.CategorizeFailure(issue.Type)
					_, err := database.RPC(ctx, "record_failure", map[string]any{
						"p_task_id":          taskID,
						"p_failure_type":     issue.Type,
						"p_failure_category": failureCategory,
						"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
						"p_model_id":         modelID,
						"p_task_type":        taskType,
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to record failure: %v", err)
					}
				}

				switch decision.NextAction {
				case "return_to_runner":
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "available",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to reset task to available: %v", err)
					}

				case "split_task", "escalate":
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "escalated",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to escalate task: %v", err)
					}

				default:
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "available",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to reset task: %v", err)
					}
				}

			case "reroute":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
				if err != nil {
					log.Printf("[EventTaskReview] Failed to reroute task: %v", err)
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			log.Printf("[EventTaskReview] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskCompleted, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		taskNumber, _ := task["task_number"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "complete"
		}

		processingBy := fmt.Sprintf("supervisor_completed:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskCompleted] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskCompleted] Task %s already being processed", truncateID(taskID))
			return
		}

		branchName := fmt.Sprintf("task/%s", taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("task/%s", truncateID(taskID))
		}

		routingResult := selectRouting("supervisor", taskID, taskType)
		if routingResult == nil {
			log.Printf("[EventTaskCompleted] No destination available for task %s", truncateID(taskID))
			recordModelFailure(ctx, database, modelID, taskID, "no_destination")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
		if err != nil {
			log.Printf("[EventTaskCompleted] Failed to create supervisor session: %v", err)
			recordModelFailure(ctx, database, modelID, taskID, "session_create_failed")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{"task": task, "event": "task_completed"})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[EventTaskCompleted] Supervisor session failed for %s: %v", truncateID(taskID), sessionErr)
			recordModelFailure(ctx, database, modelID, taskID, "session_error")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[EventTaskCompleted] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
			log.Printf("[EventTaskCompleted] Raw output: %s", truncateOutput(result.Output))
			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
			if err != nil {
				log.Printf("[EventTaskCompleted] Failed to escalate task: %v", err)
			}
			return
		}

		log.Printf("[EventTaskCompleted] Task %s decision: %s, next: %s", truncateID(taskID), decision.Decision, decision.NextAction)

		output := map[string]any{
			"output":     result.Output,
			"model_id":   modelID,
			"task_id":    taskID,
			"duration":   duration,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
			"decision":   decision.Decision,
		}

		if err := git.CommitOutput(ctx, branchName, output); err != nil {
			log.Printf("[EventTaskCompleted] Failed to commit output to %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskCompleted] Committed output to %s", branchName)
		}

		switch decision.Decision {
		case "pass":
			if decision.NextAction == "final_merge" {
				targetBranch := cfg.GetDefaultMergeTarget()
				if err := git.MergeBranch(ctx, branchName, targetBranch); err != nil {
					log.Printf("[EventTaskCompleted] Failed to merge %s to %s: %v", branchName, targetBranch, err)
					_, err = database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "escalated",
					})
				} else {
					log.Printf("[EventTaskCompleted] Merged %s to %s", branchName, targetBranch)
					_, err = database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "merged",
					})
					git.DeleteBranch(ctx, branchName)
				}
			} else {
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "approval",
				})
			}
			recordModelSuccess(ctx, database, modelID, taskType, duration)

		case "fail":
			for _, issue := range decision.Issues {
				failureCategory := runtime.CategorizeFailure(issue.Type)
				_, err := database.RPC(ctx, "record_failure", map[string]any{
					"p_task_id":          taskID,
					"p_failure_type":     issue.Type,
					"p_failure_category": failureCategory,
					"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
					"p_model_id":         modelID,
					"p_task_type":        taskType,
				})
				if err != nil {
					log.Printf("[EventTaskCompleted] Failed to record failure: %v", err)
				}
			}

			switch decision.NextAction {
			case "return_to_runner":
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
			default:
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "escalated",
				})
			}
			recordModelFailure(ctx, database, modelID, taskID, decision.NextAction)

		default:
			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
		}

		if err != nil {
			log.Printf("[EventTaskCompleted] Failed to update task status: %v", err)
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
	})
}
