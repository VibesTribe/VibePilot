package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
) {
	router.On(runtime.EventPlanCreated, func(event runtime.Event) {
		handlePlanCreated(ctx, factory, pool, database, cfg, connRouter, git, usageTracker, event)
	})
	router.On(runtime.EventPlanReview, func(event runtime.Event) {
		handlePlanReview(ctx, factory, pool, database, cfg, connRouter, git, usageTracker, event)
	})
}

func handlePlanCreated(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
	event runtime.Event,
) {
	startTime := time.Now()
	plan, err := fetchRecord(ctx, database, event)
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to get plan record: %v", err)
		return
	}

	planID, _ := plan["id"].(string)
	prdPath, _ := plan["prd_path"].(string)
	currentStatus, _ := plan["status"].(string)

	log.Printf("[EventPlanCreated] Processing plan %s, status=%s", truncateID(planID), currentStatus)

	// Only process plans still in draft. Prevents infinite loop:
	// our status update fires pg_notify which re-triggers this handler.
	if currentStatus != "draft" {
		log.Printf("[EventPlanCreated] Plan %s no longer draft (status=%s), skipping", truncateID(planID), currentStatus)
		return
	}

	processingBy := fmt.Sprintf("planner:%d", time.Now().UnixNano())
	claimed, err := database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "plans",
		"p_id":            planID,
		"p_processing_by": processingBy,
	})
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to claim plan %s: %v", truncateID(planID), err)
		return
	}
	// RPC returns bool via parseBool (handles both scalar and rowsToJSON formats)
	if err != nil || !parseBool(claimed) {
		log.Printf("[EventPlanCreated] Plan %s already being processed", truncateID(planID))
		return
	}

	clearProcessingLock := func() {
		database.RPC(ctx, "clear_processing", map[string]any{
			"p_table": "plans",
			"p_id":    planID,
		})
	}

	repoPath := cfg.GetRepoPath()

	// Fetch PRD content
	prdContent, err := fetchContent(ctx, repoPath, prdPath)
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to fetch PRD: %v", err)
		setPlanError(ctx, database, planID, "prd_fetch_failed")
		clearProcessingLock()
		return
	}

	// Try running planner with cascade retry on transient failures (429, 503, timeout)
	var result *runtime.SessionResult
	var routingResult *runtime.RoutingResult
	var failedModels []string
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		var routeErr error
		if attempt > 0 && routingResult != nil {
			failedModels = append(failedModels, routingResult.ModelID)
			log.Printf("[EventPlanCreated] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		routingResult, routeErr = connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:            "planner",
			TaskType:        "planning",
			RoutingFlag:     "internal",
			ExcludeModels:   failedModels,
			EstimatedTokens: runtime.EstimateTokens(string(prdContent), "planner"),
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[EventPlanCreated] No routing available for planner (attempt %d)", attempt+1)
			setPlanError(ctx, database, planID, "no_routing")
			clearProcessingLock()
			return
		}

		session, err := factory.CreateWithConnector(ctx, "planner", "planning", routingResult.ConnectorID)
		if err != nil {
			log.Printf("[EventPlanCreated] Failed to create planner session: %v", err)
			setPlanError(ctx, database, planID, "session_failed")
			clearProcessingLock()
			return
		}

		result, err = session.Run(ctx, map[string]any{
			"prd_content": string(prdContent),
			"plan_id":     planID,
		})
		if err != nil {
			log.Printf("[EventPlanCreated] Planner attempt %d failed (model=%s, connector=%s): %v", attempt+1, routingResult.ModelID, routingResult.ConnectorID, err)
			// Track rate limit and completion in usage tracker
			if usageTracker != nil {
				if isRateLimitError(err) {
					usageTracker.RecordRateLimit(ctx, routingResult.ModelID)
				}
				usageTracker.RecordCompletion(ctx, routingResult.ModelID, "", time.Since(startTime).Seconds(), false)
			}
			database.RPC(ctx, "record_model_failure", map[string]any{
				"p_model_id":         routingResult.ModelID,
				"p_task_id":          planID,
				"p_failure_type":     "execution_error",
				"p_failure_category": "model_issue",
			})
			if attempt < maxRetries-1 {
				continue // try next model
			}
			setPlanError(ctx, database, planID, "execution_failed")
			clearProcessingLock()
			return
		}
		// Track successful planner usage
		if usageTracker != nil {
			usageTracker.RecordUsage(ctx, routingResult.ModelID, result.TokensIn, result.TokensOut)
			usageTracker.RecordCompletion(ctx, routingResult.ModelID, "planning", time.Since(startTime).Seconds(), true)
		}
		break // success
	}

	plannerOutput, err := runtime.ParsePlannerOutput(result.Output)
	if err != nil {
		// Log first 500 chars of raw output for debugging
		raw := result.Output
		if len(raw) > 500 {
			raw = raw[:500]
		}
		log.Printf("[EventPlanCreated] Failed to parse planner output: %v\nRaw output (first 500): %s", err, raw)
		// Also dump full output to file for analysis
		os.WriteFile("/tmp/planner_output_debug.json", []byte(result.Output), 0644)
		setPlanError(ctx, database, planID, "parse_failed")
		clearProcessingLock()
		return
	}

	if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
		planFilePath := filepath.Join(repoPath, plannerOutput.PlanPath)
		planDir := filepath.Dir(planFilePath)
		if err := os.MkdirAll(planDir, 0755); err != nil {
			log.Printf("[EventPlanCreated] Failed to create plan directory: %v", err)
		} else if err := os.WriteFile(planFilePath, []byte(plannerOutput.PlanContent), 0644); err != nil {
			log.Printf("[EventPlanCreated] Failed to write plan file: %v", err)
		} else {
			log.Printf("[EventPlanCreated] Plan file written: %s", plannerOutput.PlanPath)
			if err := git.CommitAndPush(ctx, plannerOutput.PlanPath, fmt.Sprintf("docs: add plan %s", truncateID(planID))); err != nil {
				log.Printf("[EventPlanCreated] Failed to commit/push plan file: %v", err)
			} else {
				log.Printf("[EventPlanCreated] Plan file committed and pushed: %s", plannerOutput.PlanPath)
			}
		}
	}

	plan["plan_path"] = plannerOutput.PlanPath

	// Update plan status BEFORE clearing the processing lock.
	// This prevents a race where another handler sees draft + no lock and starts a duplicate planner run.
	_, err = database.RPC(ctx, "update_plan_status", map[string]any{
		"p_plan_id":   planID,
		"p_status":    "review",
		"p_plan_path": plannerOutput.PlanPath,
	})
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to update plan status: %v", err)
		clearProcessingLock()
		return
	}

	clearProcessingLock()

	database.RPC(ctx, "record_performance_metric", map[string]any{
		"p_metric_type": "prd_to_plan",
		"p_entity_id":   planID,
		"p_duration_ms": time.Since(startTime).Milliseconds(),
		"p_success":     true,
	})

	log.Printf("[EventPlanCreated] Plan %s created successfully in %dms", truncateID(planID), time.Since(startTime).Milliseconds())

	// Note: We do NOT call runPlanReview directly here.
	// The update_plan_status RPC (line 147) triggers a realtime UPDATE event,
	// which will be handled by handlePlanReview -> runPlanReview.
	// This avoids race conditions with processing locks.
}

func runPlanReview(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
	plan map[string]any,
) {
	startTime := time.Now()
	planID, _ := plan["id"].(string)
	prdPath, _ := plan["prd_path"].(string)
	planPath, _ := plan["plan_path"].(string)

	log.Printf("[PlanReview] Starting review for plan %s", truncateID(planID))

	processingBy := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
	// Retry claiming: planner may still hold the lock briefly after transitioning
	// plan to "review" (race between clear_processing and update_plan_status).
	var claimSuccess bool
	for claimAttempt := 0; claimAttempt < 3; claimAttempt++ {
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil {
			log.Printf("[PlanReview] Failed to claim plan %s: %v", truncateID(planID), err)
			return
		}
		if parseBool(claimed) {
			claimSuccess = true
		}
		if claimSuccess {
			break
		}
		if claimAttempt < 2 {
			log.Printf("[PlanReview] Plan %s locked, retrying in 3s (attempt %d/3)", truncateID(planID), claimAttempt+1)
			time.Sleep(3 * time.Second)
		}
	}
	if !claimSuccess {
		log.Printf("[PlanReview] Plan %s still locked after 3 attempts, will retry via recovery", truncateID(planID))
		return
	}

	defer func() {
		database.RPC(ctx, "clear_processing", map[string]any{
			"p_table": "plans",
			"p_id":    planID,
		})
	}()

	// Re-fetch plan from DB to confirm it's still in review status.
	// Prevents re-processing if a previous attempt already moved it to error/approved.
	currentPlanResult, err := database.Query(ctx, "plans", map[string]any{
		"id":    planID,
		"limit": 1,
	})
	if err == nil {
		var currentRows []map[string]any
		if json.Unmarshal(currentPlanResult, &currentRows) == nil && len(currentRows) > 0 {
			if currentStatus, ok := currentRows[0]["status"].(string); ok && currentStatus != "review" {
				log.Printf("[PlanReview] Plan %s no longer review (status=%s), skipping", truncateID(planID), currentStatus)
				return
			}
		}
	}

	// Fetch PRD and plan content
	repoPath := cfg.GetRepoPath()

	prdContent, err := fetchContent(ctx, repoPath, prdPath)
	if err != nil {
		log.Printf("[PlanReview] Failed to fetch PRD: %v", err)
		setPlanError(ctx, database, planID, "prd_fetch_failed")
		return
	}

	planContent, err := fetchContent(ctx, repoPath, planPath)
	if err != nil {
		log.Printf("[PlanReview] Failed to fetch plan: %v", err)
		setPlanError(ctx, database, planID, "plan_fetch_failed")
		return
	}

	// Try running supervisor with cascade retry on transient failures (429, 503, timeout)
	var result *runtime.SessionResult
	var routingResult *runtime.RoutingResult
	var failedModels []string
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 && routingResult != nil {
			failedModels = append(failedModels, routingResult.ModelID)
			log.Printf("[PlanReview] Retry %d/%d: failed models %v", attempt+1, maxRetries, failedModels)
		}
		var routeErr error
		routingResult, routeErr = connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:            "supervisor",
			TaskType:        "review",
			RoutingFlag:     "internal",
			ExcludeModels:   failedModels,
			EstimatedTokens: runtime.EstimateTokens(string(prdContent)+string(planContent), "supervisor"),
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[PlanReview] No routing available for supervisor (attempt %d)", attempt+1)
			setPlanError(ctx, database, planID, "no_routing")
			return
		}

		session, err := factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)
		if err != nil {
			log.Printf("[PlanReview] Failed to create supervisor session: %v", err)
			setPlanError(ctx, database, planID, "session_failed")
			return
		}

		result, err = session.Run(ctx, map[string]any{
			"prd_content":  string(prdContent),
			"plan_content": string(planContent),
			"plan_id":      planID,
		})
		if err != nil {
			log.Printf("[PlanReview] Supervisor attempt %d failed (model=%s, connector=%s): %v", attempt+1, routingResult.ModelID, routingResult.ConnectorID, err)
			database.RPC(ctx, "record_model_failure", map[string]any{
				"p_model_id":         routingResult.ModelID,
				"p_task_id":          planID,
				"p_failure_type":     "execution_error",
				"p_failure_category": "model_issue",
			})
			if attempt < maxRetries-1 {
				continue // try next model
			}
			setPlanError(ctx, database, planID, "execution_failed")
			return
		}
		break // success
	}

	review, err := runtime.ParseInitialReview(result.Output)
	if err != nil {
		log.Printf("[PlanReview] Failed to parse supervisor output: %v, retrying...", err)

		// Retry with explicit JSON enforcement
		retrySession, retryErr := factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)
		if retryErr == nil {
			retryResult, retryRunErr := retrySession.Run(ctx, map[string]any{
				"previous_output": result.Output,
				"parse_error":     err.Error(),
				"instruction":     "Your previous response was not valid JSON. Parse the previous output and respond with ONLY the JSON object. No markdown. No explanations.",
			})
			if retryRunErr == nil {
				review, err = runtime.ParseInitialReview(retryResult.Output)
			}
		}

		if err != nil {
			log.Printf("[PlanReview] Retry also failed to parse: %v", err)
			// Record supervisor model failure for bad output format
			database.RPC(ctx, "record_model_failure", map[string]any{
				"p_model_id":       routingResult.ModelID,
				"p_task_type":      "plan_review",
				"p_failure_class":  "json_parse_error",
				"p_failure_detail": fmt.Sprintf("Failed to produce valid JSON after retry: %v", err),
			})
			setPlanError(ctx, database, planID, "parse_failed")
			return
		}
		log.Printf("[PlanReview] Retry succeeded")
	}

	log.Printf("[PlanReview] Supervisor decision: %s", review.Decision)

	switch review.Decision {
	case "approved":
		if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), repoPath, git); err != nil {
			log.Printf("[PlanReview] Failed to create tasks: %v", err)
			setPlanError(ctx, database, planID, "task_creation_failed")
			return
		}

		_, err = database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "approved",
			"p_review_notes": map[string]any{
				"decision":   review.Decision,
				"complexity": review.Complexity,
				"reasoning":  review.Reasoning,
			},
		})
		if err != nil {
			log.Printf("[PlanReview] Failed to update plan status: %v", err)
			return
		}

		// Reinforce learned rules — plan approved after incorporating context
		// that included learned rules. Increment effectiveness of active rules
		// that apply to this project type, so the system learns which rules help.
		rulesResult, rulesErr := database.RPC(ctx, "get_planner_rules", map[string]any{
			"p_applies_to": "*",
			"p_limit":      50,
		})
		if rulesErr == nil && rulesResult != nil {
			var rulesList []map[string]any
			if json.Unmarshal(rulesResult, &rulesList) == nil {
				for _, rule := range rulesList {
					if ruleID, ok := rule["id"].(string); ok && ruleID != "" {
						if active, _ := rule["active"].(bool); active {
							_, _ = database.RPC(ctx, "record_planner_rule_prevented_issue", map[string]any{
								"p_rule_id": ruleID,
							})
						}
					}
				}
			}
		}

		database.RPC(ctx, "record_model_success", map[string]any{
			"p_model_id":         routingResult.ModelID,
			"p_task_type":        "review",
			"p_duration_seconds": int(time.Since(startTime).Seconds()),
		})
		if usageTracker != nil {
			usageTracker.RecordCompletion(ctx, routingResult.ModelID, "plan_review", time.Since(startTime).Seconds(), true)
		}

		log.Printf("[PlanReview] Plan %s approved and tasks created in %dms", truncateID(planID), time.Since(startTime).Milliseconds())

	case "needs_revision":
		failureClass := review.FailureClass
		if failureClass == "" {
			failureClass = "plan_quality"
		}
		failureDetail := review.FailureDetail
		if failureDetail == "" {
			failureDetail = review.Reasoning
		}
		_, err = database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "revision_needed",
			"p_review_notes": map[string]any{
				"decision":       review.Decision,
				"failure_class":  failureClass,
				"failure_detail": failureDetail,
				"reasoning":      review.Reasoning,
				"concerns":       review.Concerns,
			},
		})
		if err != nil {
			log.Printf("[PlanReview] Failed to update plan status: %v", err)
			return
		}

		// Learn from supervisor rejection — create planner rules so the
		// planner learns what to avoid, same pattern as council feedback.
		if failureDetail != "" {
			_, _ = database.RPC(ctx, "create_planner_rule", map[string]any{
				"p_applies_to": "*",
				"p_rule_type":  "supervisor_rejection",
				"p_rule_text":  "Avoid: " + failureDetail,
				"p_source":     "supervisor",
			})
		}
		if review.Reasoning != "" && review.Reasoning != failureDetail {
			_, _ = database.RPC(ctx, "create_planner_rule", map[string]any{
				"p_applies_to": "*",
				"p_rule_type":  "supervisor_rejection",
				"p_rule_text":  "Context: " + review.Reasoning,
				"p_source":     "supervisor",
			})
		}
		for _, concern := range review.Concerns {
			if concern != "" {
				_, _ = database.RPC(ctx, "create_planner_rule", map[string]any{
					"p_applies_to": "*",
					"p_rule_type":  "supervisor_rejection",
					"p_rule_text":  "Avoid: " + concern,
					"p_source":     "supervisor",
				})
			}
		}

		// Track the revision for analytics
		_, _ = database.RPC(ctx, "record_planner_revision", map[string]any{
			"p_plan_id":        planID,
			"p_source":         "supervisor",
			"p_failure_class":  failureClass,
			"p_failure_detail": failureDetail,
		})
		// Record supervisor model success (correctly identified plan quality issues)
		if usageTracker != nil {
			usageTracker.RecordCompletion(ctx, routingResult.ModelID, "plan_review", time.Since(startTime).Seconds(), true)
		}

		log.Printf("[PlanReview] Plan %s needs revision: %s (%s)", truncateID(planID), failureClass, failureDetail)

	case "council_review":
		_, err = database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "council_review",
			"p_review_notes": map[string]any{
				"decision":  review.Decision,
				"reasoning": review.Reasoning,
			},
		})
		if err != nil {
			log.Printf("[PlanReview] Failed to update plan status: %v", err)
			return
		}
		// Record supervisor model success (correctly identified need for council review)
		if usageTracker != nil {
			usageTracker.RecordCompletion(ctx, routingResult.ModelID, "plan_review", time.Since(startTime).Seconds(), true)
		}

		log.Printf("[PlanReview] Plan %s sent to council review (supervisor=%s)", truncateID(planID), routingResult.ModelID)

	default:
		log.Printf("[PlanReview] Unknown decision: %s", review.Decision)
		setPlanError(ctx, database, planID, "unknown_decision")
	}
}

func handlePlanReview(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
	event runtime.Event,
) {
	plan, err := fetchRecord(ctx, database, event)
	if err != nil {
		log.Printf("[EventPlanReview] Failed to get plan record: %v", err)
		return
	}

	planID, _ := plan["id"].(string)

	// Status guard: only process plans still in "review" status.
	// Prevents duplicate execution when pgnotify fires twice for the same
	// status transition (the first handler already moved the plan forward).
	currentStatus, _ := plan["status"].(string)
	if currentStatus != "review" {
		log.Printf("[EventPlanReview] Plan %s no longer in review (status=%s), skipping", truncateID(planID), currentStatus)
		return
	}

	planPath, _ := plan["plan_path"].(string)
	if planPath == "" && planID != "" {
		// Realtime event may not include plan_path (only changed columns).
		// fetchRecord already got full row, but check again.
		raw, err := database.Query(ctx, "plans", map[string]any{"id": planID})
		if err == nil && raw != nil {
			var plans []map[string]any
			if json.Unmarshal(raw, &plans) == nil && len(plans) > 0 {
				fullPlan := plans[0]
				if fp, ok := fullPlan["plan_path"].(string); ok {
					planPath = fp
					plan["plan_path"] = fp
				}
				// Merge other missing fields
				for k, v := range fullPlan {
					if _, exists := plan[k]; !exists {
						plan[k] = v
					}
				}
			}
		}
	}
	if planPath == "" {
		log.Printf("[EventPlanReview] Plan %s has no plan_path, skipping", truncateID(planID))
		return
	}

	runPlanReview(ctx, factory, pool, database, cfg, connRouter, git, usageTracker, plan)
}

func setPlanError(ctx context.Context, database db.Database, planID string, reason string) {
	_, err := database.RPC(ctx, "update_plan_status", map[string]any{
		"p_plan_id": planID,
		"p_status":  "error",
		"p_review_notes": map[string]any{
			"error": reason,
			"at":    time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[setPlanError] Failed to set plan error: %v", err)
	}
}
