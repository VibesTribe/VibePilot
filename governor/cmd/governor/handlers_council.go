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
	database     db.Database
	factory      *runtime.SessionFactory
	pool         *runtime.AgentPool
	connRouter   *runtime.Router
	cfg          *runtime.Config
	git          *gitree.Gitree
	usageTracker *runtime.UsageTracker
}

func NewCouncilHandler(
	database db.Database,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
) *CouncilHandler {
	return &CouncilHandler{
		database:     database,
		factory:      factory,
		pool:         pool,
		connRouter:   connRouter,
		cfg:          cfg,
		git:          git,
		usageTracker: usageTracker,
	}
}

func (h *CouncilHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventCouncilReview, h.handleCouncilReview)
}

func (h *CouncilHandler) handleCouncilReview(event runtime.Event) {
	ctx := context.Background()

	plan, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[CouncilReview] Failed to get plan record: %v", err)
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
			repoPath := os.Getenv("REPO_PATH")
			if content, err := fetchContent(ctx, repoPath, prdPath); err == nil {
				prdContent = string(content)
			}
		}
	}

	// Count available models to decide parallel vs sequential
	availableModels := h.connRouter.GetAvailableModelCount()
	councilMode := "sequential_same_model"
	if availableModels >= memberCount {
		councilMode = "parallel_different_models"
	}

	log.Printf("[CouncilReview] Plan %s starting (mode: %s, members: %d, available_models: %d)",
		truncateID(planID), councilMode, memberCount, availableModels)

	reviews := make([]map[string]any, memberCount)
	councilModels := make([]map[string]any, 0, memberCount)
	var failedMembers []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < memberCount; i++ {
		lens := lenses[i%len(lenses)]

		// Route each member independently through the cascade
		memberRouting, routeErr := h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "council",
			TaskType:      "council_review",
			RoutingFlag:   "internal",
			ExcludeModels: failedMembers,
		})
		if routeErr != nil || memberRouting == nil {
			log.Printf("[CouncilReview] No routing for member %d, skipping", i+1)
			continue
		}

		session, err := h.factory.CreateWithConnector(ctx, "council", lens, memberRouting.ConnectorID)
		if err != nil {
			log.Printf("[CouncilReview] Failed to create session for member %d: %v", i+1, err)
			failedMembers = append(failedMembers, memberRouting.ModelID)
			continue
		}

		contextData := map[string]any{
			"plan":          plan,
			"lens":          lens,
			"member_number": i + 1,
		}
		if prdContent != "" {
			contextData["prd_content"] = prdContent
		}

		if councilMode == "parallel_different_models" {
			wg.Add(1)
			go func(memberIndex int, sess *runtime.Session, routing *runtime.RoutingResult, memberLens string) {
				defer wg.Done()
				memberStart := time.Now()
				result, err := sess.Run(ctx, contextData)
				memberDuration := time.Since(memberStart).Seconds()
				if err != nil {
					log.Printf("[CouncilReview] Member %d failed: %v", memberIndex+1, err)
					mu.Lock()
					failedMembers = append(failedMembers, routing.ModelID)
					mu.Unlock()
					// Record failure for the model
					if h.usageTracker != nil {
						h.usageTracker.RecordCompletion(ctx, routing.ModelID, "council_review", memberDuration, false)
					}
					return
				}

				h.factory.Compact(ctx, result, planID)

				// Record token usage and completion for the council member model
				if h.usageTracker != nil {
					h.usageTracker.RecordCompletion(ctx, routing.ModelID, "council_review", memberDuration, true)
				}

				vote, parseErr := runtime.ParseCouncilVote(result.Output)
				if parseErr != nil {
					log.Printf("[CouncilReview] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
					return
				}

				mu.Lock()
				reviews[memberIndex] = map[string]any{
					"member_number": memberIndex + 1,
					"lens":          memberLens,
					"vote":          vote.Vote,
					"concerns":      vote.Concerns,
					"reasoning":     vote.Reasoning,
					"model_id":      routing.ModelID,
				}
				councilModels = append(councilModels, map[string]any{
					"lens":  memberLens,
					"model": routing.ModelID,
				})
				mu.Unlock()

				log.Printf("[CouncilReview] Member %d (%s, model=%s) voted: %s", memberIndex+1, memberLens, routing.ModelID, vote.Vote)
			}(i, session, memberRouting, lens)
		} else {
			// Sequential mode - run one at a time, reuse models if needed
			memberStart := time.Now()
			result, err := session.Run(ctx, contextData)
			memberDuration := time.Since(memberStart).Seconds()
			if err != nil {
				log.Printf("[CouncilReview] Member %d failed: %v", i+1, err)
				if h.usageTracker != nil {
					h.usageTracker.RecordCompletion(ctx, memberRouting.ModelID, "council_review", memberDuration, false)
				}
				continue
			}

			h.factory.Compact(ctx, result, planID)

			// Record token usage and completion for the council member model
			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, memberRouting.ModelID, "council_review", memberDuration, true)
			}

			vote, parseErr := runtime.ParseCouncilVote(result.Output)
			if parseErr != nil {
				log.Printf("[CouncilReview] Failed to parse vote from member %d: %v", i+1, parseErr)
				continue
			}

			reviews[i] = map[string]any{
				"member_number": i + 1,
				"lens":          lens,
				"vote":          vote.Vote,
				"concerns":      vote.Concerns,
				"reasoning":     vote.Reasoning,
				"model_id":      memberRouting.ModelID,
			}
			councilModels = append(councilModels, map[string]any{
				"lens":  lens,
				"model": memberRouting.ModelID,
			})
			log.Printf("[CouncilReview] Member %d (%s, model=%s) voted: %s", i+1, lens, memberRouting.ModelID, vote.Vote)
		}
	}
	if councilMode == "parallel_different_models" {
		wg.Wait()
	}

	validReviews := make([]map[string]any, 0, len(reviews))
	for _, r := range reviews {
		if r != nil {
			validReviews = append(validReviews, r)
		}
	}

	if len(validReviews) == 0 {
		log.Printf("[CouncilReview] No valid votes for plan %s", truncateID(planID))
		_, _ = h.database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id":      planID,
			"p_status":       "error",
			"p_review_notes": map[string]any{"error": "no_routing_available"},
		})
		return
	}

	consensus := h.determineConsensus(validReviews, memberCount)
	log.Printf("[CouncilReview] Plan %s consensus: %s (votes: %d/%d)",
		truncateID(planID), consensus, len(validReviews), memberCount)

	// Record per-model learning based on vote alignment with consensus.
	// Models whose vote aligned with the final consensus get success signal.
	// Models that voted against consensus get a gentle failure signal.
	// This teaches the router which models make good council members.
	if h.usageTracker != nil {
		for _, r := range validReviews {
			modelID, _ := r["model_id"].(string)
			vote, _ := r["vote"].(string)
			if modelID == "" {
				continue
			}
			voteAligned := false
			switch consensus {
			case "approved":
				voteAligned = vote == "APPROVED"
			case "revision_needed":
				voteAligned = vote == "REVISION_NEEDED" || vote == "BLOCKED"
			}
			if voteAligned {
				h.usageTracker.RecordCompletion(ctx, modelID, "council_vote", 0, true)
			} else {
				h.usageTracker.RecordCompletion(ctx, modelID, "council_vote", 0, false)
			}
		}
	}

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
		// Record council approval for timeline
		recordPipelineEvent(ctx, h.database, "council_approved", planID, "", "",
			map[string]any{
				"plan_id":       planID,
				"member_count":  len(validReviews),
				"consensus":     consensus,
			})
	case "revision_needed":
		h.recordCouncilFeedback(ctx, planID, validReviews)
		h.updatePlanForRevision(ctx, planID, validReviews)
		// Record council feedback for timeline
		recordPipelineEvent(ctx, h.database, "council_feedback", planID, "", "revision_needed",
			map[string]any{
				"plan_id":       planID,
				"member_count":  len(validReviews),
				"consensus":     consensus,
			})
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
	for _, r := range reviews {
		vote := getString(r, "vote")
		switch vote {
		case "APPROVED", "approved":
			approved++
		}
	}

	consensusMethod := h.cfg.GetConsensusMethod()
	if consensusMethod == "unanimous_approval" {
		if approved == memberCount {
			return "approved"
		}
		// Any concerns → revision_needed (nothing is ever blocked)
		return "revision_needed"
	}

	if approved > memberCount/2 {
		return "approved"
	}
	// Majority concerns → revision_needed with strong feedback
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
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	usageTracker *runtime.UsageTracker,
) {
	handler := NewCouncilHandler(database, factory, pool, connRouter, cfg, git, usageTracker)
	handler.Register(router)
}
