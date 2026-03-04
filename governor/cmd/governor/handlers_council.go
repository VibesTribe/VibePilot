package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

func setupCouncilHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
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

	router.On(runtime.EventCouncilDone, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("council_done:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventCouncilDone] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventCouncilDone] Plan %s already being processed", truncateID(planID))
			return
		}

		councilReviews := extractCouncilReviews(plan)

		if len(councilReviews) == 0 {
			log.Printf("[EventCouncilDone] No council reviews for plan %s - direct supervisor approval, creating tasks", truncateID(planID))
			if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
				var validationErr *ValidationFailedError
				if errors.As(err, &validationErr) {
					log.Printf("[EventCouncilDone] Task validation failed for plan %s - sending back to planner", truncateID(planID))
					var concerns []string
					var taskNumbers []string
					for _, e := range validationErr.Errors {
						concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
						taskNumbers = append(taskNumbers, e.TaskNumber)
					}
					_, _ = database.RPC(ctx, "record_planner_revision", map[string]any{
						"p_plan_id":                planID,
						"p_concerns":               concerns,
						"p_tasks_needing_revision": taskNumbers,
					})
					_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
						"p_plan_id":      planID,
						"p_status":       "revision_needed",
						"p_review_notes": map[string]any{"validation_errors": concerns},
					})
				} else {
					log.Printf("[EventCouncilDone] Failed to create tasks: %v", err)
					_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
						"p_plan_id":      planID,
						"p_status":       "error",
						"p_review_notes": map[string]any{"error": err.Error()},
					})
				}
			} else {
				_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
					"p_plan_id": planID,
					"p_status":  "approved",
				})
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		destID := selectDestination("supervisor", planID, "council_done")
		if destID == "" {
			log.Printf("[EventCouncilDone] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			_, err := session.Run(ctx, map[string]any{"plan": plan, "event": "council_done"})
			if err != nil {
				return err
			}

			approved := 0
			revisionNeeded := 0
			blocked := 0

			for _, r := range councilReviews {
				vote, _ := r["vote"].(string)
				switch vote {
				case "APPROVED":
					approved++
				case "REVISION_NEEDED":
					revisionNeeded++
				case "BLOCKED":
					blocked++
				}
			}

			memberCount := cfg.GetCouncilMemberCount()
			consensusMethod := cfg.GetConsensusMethod()

			var consensus string
			if consensusMethod == "unanimous_approval" {
				if approved == memberCount {
					consensus = "approved"
				} else if blocked > 0 {
					consensus = "blocked"
				} else {
					consensus = "revision_needed"
				}
			} else {
				if approved == memberCount {
					consensus = "approved"
				} else if blocked > 0 {
					consensus = "blocked"
				} else {
					consensus = "revision_needed"
				}
			}

			log.Printf("[EventCouncilDone] Plan %s consensus: %s (approved=%d, revision=%d, blocked=%d, method=%s)", truncateID(planID), consensus, approved, revisionNeeded, blocked, consensusMethod)

			_, err = database.RPC(ctx, "set_council_consensus", map[string]any{
				"p_plan_id":   planID,
				"p_consensus": consensus,
			})
			if err != nil {
				log.Printf("[EventCouncilDone] Failed to set council consensus: %v", err)
			}

			switch consensus {
			case "approved":
				if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
					var validationErr *ValidationFailedError
					if errors.As(err, &validationErr) {
						log.Printf("[EventCouncilDone] Task validation failed for plan %s - sending back to planner", truncateID(planID))
						var concerns []string
						var taskNumbers []string
						for _, e := range validationErr.Errors {
							concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
							taskNumbers = append(taskNumbers, e.TaskNumber)
						}
						_, _ = database.RPC(ctx, "record_planner_revision", map[string]any{
							"p_plan_id":                planID,
							"p_concerns":               concerns,
							"p_tasks_needing_revision": taskNumbers,
						})
						_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
							"p_plan_id":      planID,
							"p_status":       "revision_needed",
							"p_review_notes": map[string]any{"validation_errors": concerns, "source": "council_approved_but_validation_failed"},
						})
					} else {
						log.Printf("[EventCouncilDone] Failed to create tasks: %v", err)
						_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
							"p_plan_id":      planID,
							"p_status":       "error",
							"p_review_notes": map[string]any{"error": err.Error()},
						})
					}
				}

			case "revision_needed", "blocked":
				for _, r := range councilReviews {
					concerns, _ := r["concerns"].([]interface{})
					for _, c := range concerns {
						if cm, ok := c.(map[string]interface{}); ok {
							description, _ := cm["description"].(string)
							if description != "" {
								_, err := database.RPC(ctx, "create_planner_rule", map[string]any{
									"p_applies_to": "*",
									"p_rule_type":  "council_feedback",
									"p_rule_text":  "Avoid: " + description,
									"p_source":     "council",
								})
								if err != nil {
									log.Printf("[EventCouncilDone] Failed to create planner rule: %v", err)
								}
							}
						}
					}
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventCouncilDone] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventCouncilReview, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("council_review:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventCouncilReview] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventCouncilReview] Plan %s already being processed", truncateID(planID))
			return
		}

		memberCount := cfg.GetCouncilMemberCount()
		lenses := cfg.GetCouncilLenses()
		includePRD := cfg.ShouldCouncilIncludePRD()

		var prdContent string
		if includePRD {
			if prdPath, ok := plan["prd_path"].(string); ok && prdPath != "" {
				fullPath := filepath.Join(cfg.GetRepoPath(), prdPath)
				if content, err := os.ReadFile(fullPath); err == nil {
					prdContent = string(content)
				}
			}
		}

		destID := selectDestination("council", planID, "council_review")
		if destID == "" {
			log.Printf("[EventCouncilReview] No destination available for plan %s", truncateID(planID))
			return
		}

		councilMode := "sequential_same_model_different_hats"
		councilModels := []map[string]any{}

		availableDests := connRouter.GetAvailableConnectors()
		internalDests := 0
		for _, d := range availableDests {
			category := cfg.GetConnectorCategory(d)
			if category == "internal" {
				internalDests++
			}
		}

		if internalDests >= memberCount {
			councilMode = "parallel_different_models"
		}

		log.Printf("[EventCouncilReview] Plan %s council starting (mode: %s, members: %d)", truncateID(planID), councilMode, memberCount)

		reviews := make([]map[string]any, memberCount)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < memberCount; i++ {
			wg.Add(1)
			go func(memberIndex int) {
				defer wg.Done()

				lens := lenses[memberIndex%len(lenses)]
				session, err := factory.CreateWithContext(ctx, "council", lens)
				if err != nil {
					log.Printf("[EventCouncilReview] Failed to create council session for member %d: %v", memberIndex+1, err)
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
					log.Printf("[EventCouncilReview] Council member %d failed: %v", memberIndex+1, err)
					return
				}

				vote, parseErr := runtime.ParseCouncilVote(result.Output)
				if parseErr != nil {
					log.Printf("[EventCouncilReview] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
					return
				}

				mu.Lock()
				reviews[memberIndex] = map[string]any{
					"member_number": memberIndex + 1,
					"lens":          lens,
					"vote":          vote.Vote,
					"concerns":      vote.Concerns,
					"reasoning":     vote.Reasoning,
					"destination":   destID,
				}
				councilModels = append(councilModels, map[string]any{
					"lens":        lens,
					"destination": destID,
				})
				mu.Unlock()

				log.Printf("[EventCouncilReview] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
			}(i)
		}
		wg.Wait()

		validReviews := make([]map[string]any, 0)
		for _, r := range reviews {
			if r != nil {
				validReviews = append(validReviews, r)
			}
		}

		if len(validReviews) == 0 {
			log.Printf("[EventCouncilReview] No valid votes for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		reviewsJSON, _ := json.Marshal(validReviews)
		modelsJSON, _ := json.Marshal(councilModels)
		_, storeErr := database.RPC(ctx, "store_council_reviews", map[string]any{
			"p_plan_id": planID,
			"p_reviews": reviewsJSON,
			"p_mode":    councilMode,
			"p_models":  modelsJSON,
		})
		if storeErr != nil {
			log.Printf("[EventCouncilReview] Failed to store council reviews: %v", storeErr)
		}

		approved := 0
		revisionNeeded := 0
		blocked := 0
		var allConcerns []string
		var tasksNeedingRevision []string

		for _, r := range validReviews {
			vote, _ := r["vote"].(string)
			switch vote {
			case "APPROVED":
				approved++
			case "REVISION_NEEDED":
				revisionNeeded++
			case "BLOCKED":
				blocked++
			}
			if concerns, ok := r["concerns"].([]interface{}); ok {
				for _, c := range concerns {
					if concern, ok := c.(string); ok {
						allConcerns = append(allConcerns, concern)
					}
				}
			}
		}

		consensusMethod := cfg.GetConsensusMethod()
		var consensus string
		if consensusMethod == "unanimous_approval" {
			if approved == memberCount {
				consensus = "approved"
			} else if blocked > 0 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		} else {
			if approved > memberCount/2 {
				consensus = "approved"
			} else if blocked > memberCount/2 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		}

		log.Printf("[EventCouncilReview] Plan %s consensus: %s (approved=%d, revision=%d, blocked=%d, method=%s)", truncateID(planID), consensus, approved, revisionNeeded, blocked, consensusMethod)

		var newStatus string
		switch consensus {
		case "approved":
			newStatus = "approved"
		case "blocked":
			newStatus = "blocked"
		case "revision_needed":
			newStatus = "revision_needed"
			_, feedbackErr := database.RPC(ctx, "record_revision_feedback", map[string]any{
				"p_plan_id":                planID,
				"p_source":                 "council",
				"p_feedback":               map[string]any{"concerns": allConcerns},
				"p_tasks_needing_revision": tasksNeedingRevision,
			})
			if feedbackErr != nil {
				log.Printf("[EventCouncilReview] Failed to record revision feedback: %v", feedbackErr)
			}
		}

		_, updateErr := database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  newStatus,
			"p_review_notes": map[string]any{
				"consensus":        consensus,
				"approved_count":   approved,
				"revision_count":   revisionNeeded,
				"blocked_count":    blocked,
				"council_mode":     councilMode,
				"consensus_method": consensusMethod,
			},
		})
		if updateErr != nil {
			log.Printf("[EventCouncilReview] Failed to update plan status: %v", updateErr)
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
	})
}
