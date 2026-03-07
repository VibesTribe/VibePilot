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
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
) {
	router.On(runtime.EventPlanCreated, func(event runtime.Event) {
		handlePlanCreated(ctx, factory, pool, database, cfg, connRouter, git, event)
	})
	router.On(runtime.EventPlanReview, func(event runtime.Event) {
		handlePlanReview(ctx, factory, pool, database, cfg, connRouter, event)
	})
}

func handlePlanCreated(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	event runtime.Event,
) {
	startTime := time.Now()
	var plan map[string]any
	if err := json.Unmarshal(event.Record, &plan); err != nil {
		log.Printf("[EventPlanCreated] Failed to parse plan: %v", err)
		return
	}

	planID, _ := plan["id"].(string)
	prdPath, _ := plan["prd_path"].(string)
	currentStatus, _ := plan["status"].(string)

	log.Printf("[EventPlanCreated] Processing plan %s, status=%s", truncateID(planID), currentStatus)

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
	var claimSuccess bool
	if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
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
	prdContent, err := os.ReadFile(filepath.Join(repoPath, prdPath))
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to read PRD: %v", err)
		setPlanError(ctx, database, planID, "prd_read_failed")
		clearProcessingLock()
		return
	}

	routingResult, err := connRouter.SelectRouting(ctx, runtime.RoutingRequest{
		Role:        "planner",
		TaskType:    "planning",
		RoutingFlag: "internal",
	})
	if err != nil || routingResult == nil {
		log.Printf("[EventPlanCreated] No routing available for planner")
		setPlanError(ctx, database, planID, "no_routing")
		clearProcessingLock()
		return
	}

	session, err := factory.CreateWithContext(ctx, "planner", "planning")
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to create planner session: %v", err)
		setPlanError(ctx, database, planID, "session_failed")
		clearProcessingLock()
		return
	}

	result, err := session.Run(ctx, map[string]any{
		"prd_content": string(prdContent),
		"plan_id":     planID,
	})
	if err != nil {
		log.Printf("[EventPlanCreated] Planner execution failed: %v", err)
		database.RPC(ctx, "record_model_failure", map[string]any{
			"p_model_id":         routingResult.ModelID,
			"p_task_id":          planID,
			"p_failure_type":     "execution_error",
			"p_failure_category": "model_issue",
		})
		setPlanError(ctx, database, planID, "execution_failed")
		clearProcessingLock()
		return
	}

	plannerOutput, err := runtime.ParsePlannerOutput(result.Output)
	if err != nil {
		log.Printf("[EventPlanCreated] Failed to parse planner output: %v", err)
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
		}
	}

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

	database.RPC(ctx, "record_performance_metric", map[string]any{
		"p_metric_type": "prd_to_plan",
		"p_entity_id":   planID,
		"p_duration_ms": time.Since(startTime).Milliseconds(),
		"p_success":     true,
	})

	log.Printf("[EventPlanCreated] Plan %s created successfully in %dms", truncateID(planID), time.Since(startTime).Milliseconds())

	plan["plan_path"] = plannerOutput.PlanPath

	clearProcessingLock()

	runPlanReview(ctx, factory, pool, database, cfg, connRouter, plan)
}

func runPlanReview(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	plan map[string]any,
) {
	startTime := time.Now()
	planID, _ := plan["id"].(string)
	prdPath, _ := plan["prd_path"].(string)
	planPath, _ := plan["plan_path"].(string)

	log.Printf("[PlanReview] Starting review for plan %s", truncateID(planID))

	processingBy := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
	claimed, err := database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "plans",
		"p_id":            planID,
		"p_processing_by": processingBy,
	})
	if err != nil {
		log.Printf("[PlanReview] Failed to claim plan %s: %v", truncateID(planID), err)
		return
	}
	var claimSuccess bool
	if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
		log.Printf("[PlanReview] Plan %s already being processed", truncateID(planID))
		return
	}

	defer func() {
		database.RPC(ctx, "clear_processing", map[string]any{
			"p_table": "plans",
			"p_id":    planID,
		})
	}()

	repoPath := cfg.GetRepoPath()
	prdContent, err := os.ReadFile(filepath.Join(repoPath, prdPath))
	if err != nil {
		log.Printf("[PlanReview] Failed to read PRD: %v", err)
		setPlanError(ctx, database, planID, "prd_read_failed")
		return
	}

	planContent, err := os.ReadFile(filepath.Join(repoPath, planPath))
	if err != nil {
		log.Printf("[PlanReview] Failed to read plan: %v", err)
		setPlanError(ctx, database, planID, "plan_read_failed")
		return
	}

	routingResult, err := connRouter.SelectRouting(ctx, runtime.RoutingRequest{
		Role:        "supervisor",
		TaskType:    "review",
		RoutingFlag: "internal",
	})
	if err != nil || routingResult == nil {
		log.Printf("[PlanReview] No routing available for supervisor")
		setPlanError(ctx, database, planID, "no_routing")
		return
	}

	session, err := factory.CreateWithContext(ctx, "supervisor", "review")
	if err != nil {
		log.Printf("[PlanReview] Failed to create supervisor session: %v", err)
		setPlanError(ctx, database, planID, "session_failed")
		return
	}

	result, err := session.Run(ctx, map[string]any{
		"prd_content":  string(prdContent),
		"plan_content": string(planContent),
		"plan_id":      planID,
	})
	if err != nil {
		log.Printf("[PlanReview] Supervisor execution failed: %v", err)
		database.RPC(ctx, "record_model_failure", map[string]any{
			"p_model_id":         routingResult.ModelID,
			"p_task_id":          planID,
			"p_failure_type":     "execution_error",
			"p_failure_category": "model_issue",
		})
		setPlanError(ctx, database, planID, "execution_failed")
		return
	}

	review, err := runtime.ParseInitialReview(result.Output)
	if err != nil {
		log.Printf("[PlanReview] Failed to parse supervisor output: %v", err)
		setPlanError(ctx, database, planID, "parse_failed")
		return
	}

	log.Printf("[PlanReview] Supervisor decision: %s", review.Decision)

	switch review.Decision {
	case "approved":
		if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), repoPath); err != nil {
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

		database.RPC(ctx, "record_model_success", map[string]any{
			"p_model_id":         routingResult.ModelID,
			"p_task_type":        "review",
			"p_duration_seconds": int(time.Since(startTime).Seconds()),
		})

		log.Printf("[PlanReview] Plan %s approved and tasks created in %dms", truncateID(planID), time.Since(startTime).Milliseconds())

	case "needs_revision":
		_, err = database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "revision_needed",
			"p_review_notes": map[string]any{
				"decision":  review.Decision,
				"reasoning": review.Reasoning,
				"concerns":  review.Concerns,
			},
		})
		if err != nil {
			log.Printf("[PlanReview] Failed to update plan status: %v", err)
			return
		}

		log.Printf("[PlanReview] Plan %s needs revision", truncateID(planID))

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

		log.Printf("[PlanReview] Plan %s sent to council review", truncateID(planID))

	default:
		log.Printf("[PlanReview] Unknown decision: %s", review.Decision)
		setPlanError(ctx, database, planID, "unknown_decision")
	}
}

func handlePlanReview(
	ctx context.Context,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	event runtime.Event,
) {
	var plan map[string]any
	if err := json.Unmarshal(event.Record, &plan); err != nil {
		log.Printf("[EventPlanReview] Failed to parse plan: %v", err)
		return
	}

	planPath, _ := plan["plan_path"].(string)
	if planPath == "" {
		log.Printf("[EventPlanReview] Plan has no plan_path, skipping")
		return
	}

	runPlanReview(ctx, factory, pool, database, cfg, connRouter, plan)
}

func setPlanError(ctx context.Context, database *db.DB, planID string, reason string) {
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
