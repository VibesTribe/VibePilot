package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

func setupPlanHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
) {
	selectDestination := func(agentID, planID, taskType string) string {
		result, err := connRouter.SelectDestination(ctx, runtime.RoutingRequest{
			AgentID:  agentID,
			TaskID:   planID,
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

	router.On(runtime.EventPlanCreated, func(event runtime.Event) {
		log.Printf("[EventPlanCreated] Handler invoked for event ID=%s", event.ID)
		startTime := time.Now()
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPlanCreated] Failed to unmarshal plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)
		currentStatus, _ := plan["status"].(string)
		log.Printf("[EventPlanCreated] Processing plan %s, status=%s", truncateID(planID), currentStatus)

		processingBy := fmt.Sprintf("plan_created:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventPlanCreated] Plan %s claim failed: %v", truncateID(planID), claimErr)
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPlanCreated] Plan %s already being processed", truncateID(planID))
			return
		}
		log.Printf("[EventPlanCreated] Plan %s claimed successfully", truncateID(planID))

		destID := selectDestination("planner", planID, "plan_creation")
		log.Printf("[EventPlanCreated] Selected destination: %s", destID)
		if destID == "" {
			log.Printf("[EventPlanCreated] No destination available for plan %s", truncateID(planID))
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "no_destination")
			return
		}

		session, err := factory.Create("planner")
		if err != nil {
			log.Printf("[EventPlanCreated] Failed to create session: %v", err)
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "session_creation_failed")
			return
		}
		log.Printf("[EventPlanCreated] Session created, submitting to pool...")

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "review", "plan_created")

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_created"})
			if err != nil {
				database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), false, map[string]any{"error": err.Error()})
				return err
			}

			log.Printf("[EventPlanCreated] Raw planner output: %s", truncateOutput(result.Output))

			log.Printf("[EventPlanCreated] Raw output (first 500 chars): %s", truncateOutput(result.Output))

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventPlanCreated] Failed to parse planner output: %v", parseErr)
				database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), false, map[string]any{"error": parseErr.Error()})
				return nil
			}

			log.Printf("[EventPlanCreated] Plan %s created, status: %s, tasks: %d", truncateID(planID), plannerOutput.Status, plannerOutput.TotalTasks)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
				if commitErr := git.CommitOutput(ctx, "main", output); commitErr != nil {
					log.Printf("[EventPlanCreated] Failed to commit plan to GitHub: %v", commitErr)
				} else {
					log.Printf("[EventPlanCreated] Plan file committed: %s", plannerOutput.PlanPath)
				}
			}

			newStatus := plannerOutput.Status
			if newStatus == "" {
				newStatus = "review"
			}

			_, updateErr := database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       newStatus,
				"p_plan_path":    plannerOutput.PlanPath,
				"p_review_notes": map[string]any{"plan_content": plannerOutput.PlanContent, "total_tasks": plannerOutput.TotalTasks},
			})
			if updateErr != nil {
				log.Printf("[EventPlanCreated] Failed to update plan status: %v", updateErr)
			}

			if newStatus == "review" {
				log.Printf("[EventPlanCreated] Triggering supervisor review for plan %s", truncateID(planID))
				supervisorSession, supErr := factory.Create("supervisor")
				if supErr != nil {
					log.Printf("[EventPlanCreated] Failed to create supervisor session: %v", supErr)
				} else {
					updatedPlan := map[string]any{
						"id":           planID,
						"prd_path":     plan["prd_path"],
						"plan_path":    plannerOutput.PlanPath,
						"status":       newStatus,
						"plan_content": plannerOutput.PlanContent,
					}
					supResult, supErr := supervisorSession.Run(ctx, map[string]any{"plan": updatedPlan, "event": "plan_review"})
					if supErr != nil {
						log.Printf("[EventPlanCreated] Supervisor review failed: %v", supErr)
					} else {
						log.Printf("[EventPlanCreated] Supervisor review completed: %s", truncateOutput(supResult.Output))
					}
				}
			}

			database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), true, nil)
			return nil
		})
		if err != nil {
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "pool_submit_failed")
			log.Printf("[EventPlanCreated] Failed to submit to pool: %v", err)
		}
		log.Printf("[EventPlanCreated] Submitted to pool successfully")
	})

	router.On(runtime.EventRevisionNeeded, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("planner_revision:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventRevisionNeeded] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventRevisionNeeded] Plan %s already being processed", truncateID(planID))
			return
		}

		maxRounds := cfg.GetMaxRevisionRounds()
		onMaxRounds := cfg.GetOnMaxRoundsAction()

		currentRound, _ := plan["revision_round"].(float64)
		if int(currentRound) >= maxRounds {
			log.Printf("[EventRevisionNeeded] Plan %s revision limit (%d) reached (current: %d), escalating", truncateID(planID), maxRounds, int(currentRound))
			_, err := database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  onMaxRounds,
				"p_review_notes": map[string]any{
					"error":         "revision_limit_reached",
					"max_rounds":    maxRounds,
					"current_round": int(currentRound),
				},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		limitReached, _ := database.RPC(ctx, "check_revision_limit", map[string]any{
			"p_plan_id":    planID,
			"p_max_rounds": maxRounds,
		})

		var limitReachedBool bool
		if limitReached != nil {
			if err := json.Unmarshal(limitReached, &limitReachedBool); err != nil {
				var result []bool
				if err := json.Unmarshal(limitReached, &result); err == nil && len(result) > 0 {
					limitReachedBool = result[0]
				}
			}
		}

		if limitReachedBool {
			log.Printf("[EventRevisionNeeded] Plan %s revision limit (%d) reached, escalating to human", truncateID(planID), maxRounds)
			_, err := database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  onMaxRounds,
				"p_review_notes": map[string]any{
					"error":      "revision_limit_reached",
					"max_rounds": maxRounds,
				},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		_, err = database.RPC(ctx, "increment_revision_round", map[string]any{
			"p_plan_id": planID,
		})
		if err != nil {
			log.Printf("[EventRevisionNeeded] Failed to increment revision round: %v", err)
		}

		revisionHistory, _ := plan["revision_history"].([]interface{})
		var latestFeedback map[string]any
		if len(revisionHistory) > 0 {
			if rh, ok := revisionHistory[len(revisionHistory)-1].(map[string]any); ok {
				latestFeedback = rh
			}
		}

		destID := selectDestination("planner", planID, "revision")
		if destID == "" {
			log.Printf("[EventRevisionNeeded] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "planner", "revision")
		if err != nil {
			log.Printf("[EventRevisionNeeded] Failed to create planner session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			result, err := session.Run(ctx, map[string]any{
				"plan":             plan,
				"event":            "revision_needed",
				"revision_history": revisionHistory,
				"latest_feedback":  latestFeedback,
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Planner session failed for %s: %v", truncateID(planID), err)
				return err
			}

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventRevisionNeeded] Failed to parse planner output: %v", parseErr)
				return nil
			}

			log.Printf("[EventRevisionNeeded] Plan %s revised, status: %s", truncateID(planID), plannerOutput.Status)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
				if err := git.CommitOutput(ctx, "main", output); err != nil {
					log.Printf("[EventRevisionNeeded] Failed to commit plan to GitHub: %v", err)
				}
			}

			newStatus := plannerOutput.Status
			if newStatus == "" {
				newStatus = "review"
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       newStatus,
				"p_plan_path":    plannerOutput.PlanPath,
				"p_review_notes": map[string]any{"plan_content": plannerOutput.PlanContent, "total_tasks": plannerOutput.TotalTasks, "revised": true},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventRevisionNeeded] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPlanApproved, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		log.Printf("[EventPlanApproved] Plan %s already approved (direct), tasks should exist", truncateID(planID))
	})

	router.On(runtime.EventPlanBlocked, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		log.Printf("[EventPlanBlocked] Plan %s blocked - requires human intervention", truncateID(planID))
	})

	router.On(runtime.EventPRDIncomplete, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		reviewNotes, _ := plan["review_notes"].(map[string]any)
		blockedReason, _ := reviewNotes["blocked_reason"].(string)

		log.Printf("[EventPRDIncomplete] Plan %s PRD incomplete: %s", truncateID(planID), blockedReason)

		_, err := database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "pending_human",
			"p_review_notes": map[string]any{
				"blocked_reason": blockedReason,
				"action_needed":  "Update PRD with missing information",
			},
		})
		if err != nil {
			log.Printf("[EventPRDIncomplete] Failed to update plan status: %v", err)
		}
	})

	router.On(runtime.EventPlanError, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		reviewNotes, _ := plan["review_notes"].(map[string]any)
		errorMsg, _ := reviewNotes["error"].(string)
		log.Printf("[EventPlanError] Plan %s in error state: %s", truncateID(planID), errorMsg)
	})

	router.On(runtime.EventPRDReady, func(event runtime.Event) {
		startTime := time.Now()
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPRDReady] Failed to parse plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)
		currentStatus, _ := plan["status"].(string)

		processingBy := fmt.Sprintf("planner:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventPRDReady] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPRDReady] Plan %s already being processed", truncateID(planID))
			return
		}

		destID := selectDestination("planner", planID, "planning")
		if destID == "" {
			log.Printf("[EventPRDReady] No destination available for plan %s", truncateID(planID))
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "no_destination")
			return
		}

		projectType := "general"
		if prdPath, ok := plan["prd_path"].(string); ok {
			if strings.Contains(strings.ToLower(prdPath), "dashboard") || strings.Contains(strings.ToLower(prdPath), "ui") {
				projectType = "frontend"
			} else if strings.Contains(strings.ToLower(prdPath), "api") {
				projectType = "backend"
			}
		}

		session, err := factory.CreateWithContext(ctx, "planner", projectType)
		if err != nil {
			log.Printf("[EventPRDReady] Failed to create planner session: %v", err)
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "session_creation_failed")
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", destID, func() error {
			defer database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "review", "planning_complete")

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "prd_ready"})
			if err != nil {
				log.Printf("[EventPRDReady] Planner session failed for %s: %v", truncateID(planID), err)
				database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), false, map[string]any{"error": err.Error()})
				return err
			}

			database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), true, nil)

			log.Printf("[EventPRDReady] Raw output for %s (len=%d): %s", truncateID(planID), len(result.Output), truncateOutput(result.Output))

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventPRDReady] Failed to parse planner output: %v", parseErr)
				return nil
			}

			if plannerOutput.Status == "" {
				plannerOutput.Status = "review"
			}

			log.Printf("[EventPRDReady] Plan %s created with %d tasks, status: %s", truncateID(planID), plannerOutput.TotalTasks, plannerOutput.Status)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
				if err := git.CommitOutput(ctx, "main", output); err != nil {
					log.Printf("[EventPRDReady] Failed to commit plan to GitHub: %v", err)
				}
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       plannerOutput.Status,
				"p_plan_path":    plannerOutput.PlanPath,
				"p_review_notes": map[string]any{"plan_content": plannerOutput.PlanContent, "total_tasks": plannerOutput.TotalTasks},
			})
			if err != nil {
				log.Printf("[EventPRDReady] Failed to update plan status: %v", err)
			}

			return nil
		})
		if err != nil {
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "pool_submit_failed")
			log.Printf("[EventPRDReady] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPlanReview, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPlanReview] Failed to parse plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventPlanReview] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPlanReview] Plan %s already being processed", truncateID(planID))
			return
		}

		destID := selectDestination("supervisor", planID, "plan_review")
		if destID == "" {
			log.Printf("[EventPlanReview] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			log.Printf("[EventPlanReview] Failed to create supervisor session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_review"})
			if err != nil {
				log.Printf("[EventPlanReview] Supervisor session failed for %s: %v", truncateID(planID), err)
				return err
			}

			review, parseErr := runtime.ParseInitialReview(result.Output)
			if parseErr != nil {
				log.Printf("[EventPlanReview] Failed to parse review: %v", parseErr)
				log.Printf("[EventPlanReview] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventPlanReview] Plan %s review: decision=%s complexity=%s", truncateID(planID), review.Decision, review.Complexity)

			var newStatus string
			var statusError error
			switch review.Decision {
			case "approved":
				if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
					var validationErr *ValidationFailedError
					if errors.As(err, &validationErr) {
						log.Printf("[EventPlanReview] Task validation failed for plan %s - sending back to planner", truncateID(planID))
						newStatus = "revision_needed"

						var concerns []string
						var taskNumbers []string
						for _, e := range validationErr.Errors {
							concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
							taskNumbers = append(taskNumbers, e.TaskNumber)
						}

						_, recordErr := database.RPC(ctx, "record_planner_revision", map[string]any{
							"p_plan_id":                planID,
							"p_concerns":               concerns,
							"p_tasks_needing_revision": taskNumbers,
						})
						if recordErr != nil {
							log.Printf("[EventPlanReview] Failed to record validation feedback: %v", recordErr)
						}

						_, recordErr = database.RPC(ctx, "record_supervisor_rule", map[string]any{
							"p_rule_text":  fmt.Sprintf("Plan passed review but failed task validation: %s", strings.Join(concerns, "; ")),
							"p_applies_to": "plan_review",
							"p_source":     "validation_safety_net",
						})
						if recordErr != nil {
							log.Printf("[EventPlanReview] Failed to record supervisor rule: %v", recordErr)
						}

						log.Printf("[EventPlanReview] Validation concerns: %v", concerns)
						statusError = err
					} else {
						log.Printf("[EventPlanReview] Failed to create tasks: %v", err)
						newStatus = "error"
						statusError = err
					}
				} else {
					newStatus = "approved"
				}
			case "needs_revision":
				newStatus = "revision_needed"
				_, err := database.RPC(ctx, "record_planner_revision", map[string]any{
					"p_plan_id":                planID,
					"p_concerns":               review.Concerns,
					"p_tasks_needing_revision": review.TasksNeedingRevision,
				})
				if err != nil {
					log.Printf("[EventPlanReview] Failed to record revision feedback: %v", err)
				}
				log.Printf("[EventPlanReview] Plan %s needs revision: %v", truncateID(planID), review.Concerns)
			case "council_review":
				newStatus = "council_review"
			default:
				newStatus = "revision_needed"
			}

			reviewNotes := map[string]any{
				"complexity": review.Complexity,
				"reasoning":  review.Reasoning,
				"concerns":   review.Concerns,
				"task_count": review.TaskCount,
			}
			if statusError != nil {
				reviewNotes["error"] = statusError.Error()
			}

			log.Printf("[EventPlanReview] Updating plan %s status to: %s", truncateID(planID), newStatus)
			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       newStatus,
				"p_review_notes": reviewNotes,
			})
			if err != nil {
				log.Printf("[EventPlanReview] Failed to update plan status: %v", err)
			} else {
				log.Printf("[EventPlanReview] Plan %s status updated to: %s", truncateID(planID), newStatus)
			}

			if statusError != nil {
				return statusError
			}
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventPlanReview] Failed to submit to pool: %v", err)
		}
	})
}
