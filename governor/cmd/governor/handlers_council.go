package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

type CouncilHandler struct {
	database   *db.DB
	factory    *runtime.SessionFactory
	pool       *runtime.AgentPool
	connRouter *runtime.Router
	cfg        *runtime.Config
	git        *gitree.Gitree
}

func NewCouncilHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
	git *gitree.Gitree,
) *CouncilHandler {
	return &CouncilHandler{
		database:   database,
		factory:    factory,
		pool:       pool,
		connRouter: connRouter,
		cfg:        cfg,
		git:        git,
	}
}

func (h *CouncilHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventCouncilReview, h.handleCouncilReview)
	router.On(runtime.EventCouncilDone, h.handleCouncilDone)
}

func (h *CouncilHandler) handleCouncilReview(event runtime.Event) {
	ctx := context.Background()

	var plan map[string]any
	if err := json.Unmarshal(event.Record, &plan); err != nil {
		log.Printf("[CouncilReview] Failed to parse event: %v", err)
		return
	}

	planID := getString(plan, "id")
	if planID == "" {
		return
	}

	processingBy := fmt.Sprintf("council_review:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "plans",
		"p_id":            planID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[CouncilReview] Plan %s already being processed", truncateID(planID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "plans",
		"p_id":    planID,
	})

	memberCount := h.cfg.GetCouncilMemberCount()
	lenses := h.cfg.GetCouncilLenses()
	includePRD := h.cfg.ShouldCouncilIncludePRD()

	if memberCount <= 0 {
		memberCount = 3
	}
	if len(lenses) == 0 {
		lenses = []string{"user_alignment", "architecture", "feasibility"}
	}

	var prdContent string
	if includePRD {
		if prdPath := getString(plan, "prd_path"); prdPath != "" {
			fullPath := fmt.Sprintf("%s/%s", h.cfg.GetRepoPath(), prdPath)
			if content, err := os.ReadFile(fullPath); err == nil {
				prdContent = string(content)
			}
		}
	}

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "council",
		TaskID:   planID,
		TaskType: "council_review",
	})
	if err != nil || routingResult == nil {
		log.Printf("[CouncilReview] No destination for plan %s", truncateID(planID))
		_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id":      planID,
			"p_status":       "error",
			"p_review_notes": map[string]any{"error": "no_destination"},
		})
		return
	}

	councilMode := "sequential_same_model"
	availableDests := h.connRouter.GetAvailableConnectors()
	internalCount := 0
	for _, d := range availableDests {
		if h.cfg.GetConnectorCategory(d) == "internal" {
			internalCount++
		}
	}
	if internalCount >= memberCount {
		councilMode = "parallel_different_models"
	}

	log.Printf("[CouncilReview] Plan %s starting (mode: %s, members: %d)",
		truncateID(planID), councilMode, memberCount)

	reviews := make([]map[string]any, memberCount)
	councilModels := make([]map[string]any, 0, memberCount)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < memberCount; i++ {
		wg.Add(1)
		go func(memberIndex int) {
			defer wg.Done()

			lens := lenses[memberIndex%len(lenses)]
			session, err := h.factory.CreateWithContext(ctx, "council", lens)
			if err != nil {
				log.Printf("[CouncilReview] Failed to create session for member %d: %v", memberIndex+1, err)
				return
			}

			contextData := map[string]any{
				"plan":          plan,
				"lens":          lens,
				"member_number": memberIndex + 1,
			}
			if prdContent != "" {
				contextData["prd_content"] = prdContent
			}

			result, err := session.Run(ctx, contextData)
			if err != nil {
				log.Printf("[CouncilReview] Member %d failed: %v", memberIndex+1, err)
				return
			}

			vote, parseErr := runtime.ParseCouncilVote(result.Output)
			if parseErr != nil {
				log.Printf("[CouncilReview] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
				return
			}

			mu.Lock()
			reviews[memberIndex] = map[string]any{
				"member_number": memberIndex + 1,
				"lens":          lens,
				"vote":          vote.Vote,
				"concerns":      vote.Concerns,
				"reasoning":     vote.Reasoning,
				"model_id":      routingResult.ModelID,
			}
			councilModels = append(councilModels, map[string]any{
				"lens":  lens,
				"model": routingResult.ModelID,
			})
			mu.Unlock()

			log.Printf("[CouncilReview] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
		}(i)
	}
	wg.Wait()

	validReviews := make([]map[string]any, 0, len(reviews))
	for _, r := range reviews {
		if r != nil {
			validReviews = append(validReviews, r)
		}
	}

	if len(validReviews) == 0 {
		log.Printf("[CouncilReview] No valid votes for plan %s", truncateID(planID))
		return
	}

	consensus := h.determineConsensus(validReviews, memberCount)
	log.Printf("[CouncilReview] Plan %s consensus: %s (votes: %d/%d)",
		truncateID(planID), consensus, len(validReviews), memberCount)

	reviewsJSON, _ := json.Marshal(validReviews)
	modelsJSON, _ := json.Marshal(councilModels)
	_, _ = h.database.RPC(ctx, "store_council_reviews", map[string]any{
		"p_plan_id": planID,
		"p_reviews": reviewsJSON,
		"p_mode":    councilMode,
		"p_models":  modelsJSON,
	})

	_, _ = h.database.RPC(ctx, "set_council_consensus", map[string]any{
		"p_plan_id":   planID,
		"p_consensus": consensus,
	})

	switch consensus {
	case "approved":
		h.handleApprovedPlan(ctx, plan, planID)
	case "blocked":
		h.recordCouncilFeedback(ctx, planID, validReviews)
		_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "blocked",
			"p_review_notes": map[string]any{
				"consensus": consensus,
				"reviews":   validReviews,
			},
		})
	case "revision_needed":
		h.recordCouncilFeedback(ctx, planID, validReviews)
		h.updatePlanForRevision(ctx, planID, validReviews)
	}
}

func (h *CouncilHandler) handleCouncilDone(event runtime.Event) {
	ctx := context.Background()

	var plan map[string]any
	if err := json.Unmarshal(event.Record, &plan); err != nil {
		log.Printf("[CouncilDone] Failed to parse event: %v", err)
		return
	}

	planID := getString(plan, "id")
	if planID == "" {
		return
	}

	processingBy := fmt.Sprintf("council_done:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "plans",
		"p_id":            planID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[CouncilDone] Plan %s already being processed", truncateID(planID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "plans",
		"p_id":    planID,
	})

	councilReviews := h.extractCouncilReviews(plan)
	consensus := getString(plan, "council_consensus")
	if consensus == "" {
		consensus = h.determineConsensus(councilReviews, h.cfg.GetCouncilMemberCount())
	}

	log.Printf("[CouncilDone] Plan %s consensus: %s", truncateID(planID), consensus)

	switch consensus {
	case "approved":
		h.handleApprovedPlan(ctx, plan, planID)
	case "blocked":
		h.recordCouncilFeedback(ctx, planID, councilReviews)
		_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "blocked",
		})
	case "revision_needed":
		h.recordCouncilFeedback(ctx, planID, councilReviews)
		h.updatePlanForRevision(ctx, planID, councilReviews)
	}
}

func (h *CouncilHandler) handleApprovedPlan(ctx context.Context, plan map[string]any, planID string) {
	if err := createTasksFromApprovedPlan(ctx, h.database, plan, h.cfg.GetValidationConfig(), h.cfg.GetRepoPath(), h.git); err != nil {
		var validationErr *ValidationFailedError
		if errors.As(err, &validationErr) {
			log.Printf("[Council] Task validation failed for %s", truncateID(planID))
			var concerns []string
			var taskNumbers []string
			for _, e := range validationErr.Errors {
				concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
				taskNumbers = append(taskNumbers, e.TaskNumber)
			}
			_, _ = h.database.RPC(ctx, "record_planner_revision", map[string]any{
				"p_plan_id":                planID,
				"p_concerns":               concerns,
				"p_tasks_needing_revision": taskNumbers,
			})
			_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  "revision_needed",
				"p_review_notes": map[string]any{
					"validation_errors": concerns,
					"source":            "council_approved_but_validation_failed",
				},
			})
			return
		}
		log.Printf("[Council] Failed to create tasks for %s: %v", truncateID(planID), err)
		_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "error",
			"p_review_notes": map[string]any{
				"error": err.Error(),
			},
		})
		return
	}

	_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
		"p_plan_id": planID,
		"p_status":  "approved",
	})
	log.Printf("[Council] Plan %s approved, tasks created", truncateID(planID))
}

func (h *CouncilHandler) determineConsensus(reviews []map[string]any, memberCount int) string {
	if len(reviews) == 0 {
		return "revision_needed"
	}

	approved := 0
	blocked := 0

	for _, r := range reviews {
		vote := getString(r, "vote")
		switch vote {
		case "APPROVED", "approved":
			approved++
		case "BLOCKED", "blocked":
			blocked++
		}
	}

	consensusMethod := h.cfg.GetConsensusMethod()
	if consensusMethod == "unanimous_approval" {
		if approved == memberCount {
			return "approved"
		}
		if blocked > 0 {
			return "blocked"
		}
		return "revision_needed"
	}

	if approved > memberCount/2 {
		return "approved"
	}
	if blocked > memberCount/2 {
		return "blocked"
	}
	return "revision_needed"
}

func (h *CouncilHandler) extractCouncilReviews(plan map[string]any) []map[string]any {
	reviews := plan["council_reviews"]
	if reviews == nil {
		return nil
	}

	switch v := reviews.(type) {
	case []interface{}:
		result := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, mapStrAny(m))
			}
		}
		return result
	case []map[string]interface{}:
		result := make([]map[string]any, 0, len(v))
		for _, m := range v {
			result = append(result, mapStrAny(m))
		}
		return result
	case string:
		if v == "" || v == "[]" || v == "null" {
			return nil
		}
		var parsed []map[string]any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}

func mapStrAny(m map[string]interface{}) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func (h *CouncilHandler) recordCouncilFeedback(ctx context.Context, planID string, reviews []map[string]any) {
	for _, r := range reviews {
		concerns, _ := r["concerns"].([]interface{})
		for _, c := range concerns {
			var description string
			switch cm := c.(type) {
			case map[string]interface{}:
				description = getString(mapStrAny(cm), "description")
				if description == "" {
					description = getString(mapStrAny(cm), "issue")
				}
			case string:
				description = cm
			}
			if description != "" {
				_, _ = h.database.RPC(ctx, "create_planner_rule", map[string]any{
					"p_applies_to": "*",
					"p_rule_type":  "council_feedback",
					"p_rule_text":  "Avoid: " + description,
					"p_source":     "council",
				})
			}
		}
	}
}

func (h *CouncilHandler) updatePlanForRevision(ctx context.Context, planID string, reviews []map[string]any) {
	var allConcerns []string
	var tasksNeedingRevision []string

	for _, r := range reviews {
		if concerns, ok := r["concerns"].([]interface{}); ok {
			for _, c := range concerns {
				switch cm := c.(type) {
				case map[string]interface{}:
					if desc := getString(mapStrAny(cm), "description"); desc != "" {
						allConcerns = append(allConcerns, desc)
					}
					if taskID := getString(mapStrAny(cm), "task_id"); taskID != "" {
						tasksNeedingRevision = append(tasksNeedingRevision, taskID)
					}
				case string:
					allConcerns = append(allConcerns, cm)
				}
			}
		}
	}

	_, _ = h.database.RPC(ctx, "record_revision_feedback", map[string]any{
		"p_plan_id":                planID,
		"p_source":                 "council",
		"p_feedback":               map[string]any{"concerns": allConcerns},
		"p_tasks_needing_revision": tasksNeedingRevision,
	})

	_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
		"p_plan_id": planID,
		"p_status":  "revision_needed",
		"p_review_notes": map[string]any{
			"consensus":              "revision_needed",
			"concerns":               allConcerns,
			"tasks_needing_revision": tasksNeedingRevision,
		},
	})
}

func setupCouncilHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
) {
	handler := NewCouncilHandler(database, factory, pool, connRouter, cfg, git)
	handler.Register(router)
}
