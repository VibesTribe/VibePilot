package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

func setupResearchHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
) {
	selectDestination := func(agentID, suggestionID, taskType string) string {
		result, err := connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
			AgentID:  agentID,
			TaskID:   suggestionID,
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

	router.On(runtime.EventResearchReady, func(event runtime.Event) {
		var suggestion map[string]any
		if err := json.Unmarshal(event.Record, &suggestion); err != nil {
			log.Printf("[EventResearchReady] Failed to parse suggestion: %v", err)
			return
		}

		suggestionID, _ := suggestion["id"].(string)
		suggestionType, _ := suggestion["type"].(string)
		complexity, _ := suggestion["complexity"].(string)

		processingBy := fmt.Sprintf("research_ready:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "research_suggestions",
			"p_id":            suggestionID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventResearchReady] Suggestion %s already being processed or claim failed", truncateID(suggestionID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventResearchReady] Suggestion %s already being processed", truncateID(suggestionID))
			return
		}

		log.Printf("[EventResearchReady] Processing research suggestion %s (type: %s, complexity: %s)", truncateID(suggestionID), suggestionType, complexity)

		switch complexity {
		case "human":
			log.Printf("[EventResearchReady] Human review required for %s", truncateID(suggestionID))
			_, err := database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":           suggestionID,
				"p_status":       "pending_human",
				"p_review_notes": map[string]any{"reason": "complexity=human, requires human decision"},
			})
			if err != nil {
				log.Printf("[EventResearchReady] Failed to update status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return

		case "complex":
			log.Printf("[EventResearchReady] Complex item %s - routing to council", truncateID(suggestionID))
			_, err := database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":           suggestionID,
				"p_status":       "council_review",
				"p_review_notes": map[string]any{"source": "research", "type": suggestionType},
			})
			if err != nil {
				log.Printf("[EventResearchReady] Failed to update status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		destID := selectDestination("supervisor", suggestionID, "research_review")
		if destID == "" {
			log.Printf("[EventResearchReady] No destination available")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "research", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})

			result, err := session.Run(ctx, map[string]any{
				"event":      "research_review",
				"suggestion": suggestion,
			})
			if err != nil {
				return err
			}

			review, parseErr := runtime.ParseResearchReview(result.Output)
			if parseErr != nil {
				log.Printf("[EventResearchReady] Failed to parse review: %v", parseErr)
				log.Printf("[EventResearchReady] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventResearchReady] Suggestion %s review: decision=%s", truncateID(suggestionID), review.Decision)

			switch review.Decision {
			case "approved":
				if review.MaintenanceCommand != nil {
					cmdJSON, _ := json.Marshal(review.MaintenanceCommand.Details)
					_, err := database.RPC(ctx, "create_maintenance_command", map[string]any{
						"p_command_type": review.MaintenanceCommand.Action,
						"p_payload":      json.RawMessage(cmdJSON),
						"p_source":       "research_review",
						"p_approved_by":  "supervisor",
					})
					if err != nil {
						log.Printf("[EventResearchReady] Failed to create maintenance command: %v", err)
					} else {
						log.Printf("[EventResearchReady] Created maintenance command: %s", review.MaintenanceCommand.Action)
					}
				}
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "approved",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"notes":     review.Notes,
					},
				})

			case "rejected":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "rejected",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
					},
				})

			case "council_review":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "council_review",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"source":    "research",
					},
				})

			case "human_review":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "pending_human",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"urgency":   review.Urgency,
					},
				})
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			log.Printf("[EventResearchReady] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventResearchCouncil, func(event runtime.Event) {
		var suggestion map[string]any
		if err := json.Unmarshal(event.Record, &suggestion); err != nil {
			log.Printf("[EventResearchCouncil] Failed to parse suggestion: %v", err)
			return
		}

		suggestionID, _ := suggestion["id"].(string)
		title, _ := suggestion["title"].(string)

		processingBy := fmt.Sprintf("research_council:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "research_suggestions",
			"p_id":            suggestionID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventResearchCouncil] Suggestion %s already being processed or claim failed", truncateID(suggestionID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventResearchCouncil] Suggestion %s already being processed", truncateID(suggestionID))
			return
		}

		log.Printf("[EventResearchCouncil] Starting council review for %s: %s", truncateID(suggestionID), title)

		memberCount := cfg.GetCouncilMemberCount()
		lenses := cfg.GetCouncilLenses()
		if len(lenses) == 0 {
			lenses = []string{"user_alignment", "architecture", "feasibility"}
		}

		councilMode := "sequential_same_model"
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

		log.Printf("[EventResearchCouncil] Council starting (mode: %s, members: %d)", councilMode, memberCount)

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
					log.Printf("[EventResearchCouncil] Failed to create council session for member %d: %v", memberIndex+1, err)
					return
				}

				contextData := map[string]any{
					"research":      suggestion,
					"lens":          lens,
					"member_number": memberIndex + 1,
					"review_type":   "research",
				}

				result, err := session.Run(ctx, contextData)
				if err != nil {
					log.Printf("[EventResearchCouncil] Council member %d failed: %v", memberIndex+1, err)
					return
				}

				vote, parseErr := runtime.ParseCouncilVote(result.Output)
				if parseErr != nil {
					log.Printf("[EventResearchCouncil] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
					return
				}

				mu.Lock()
				reviews[memberIndex] = map[string]any{
					"member_number": memberIndex + 1,
					"lens":          lens,
					"vote":          vote.Vote,
					"concerns":      vote.Concerns,
					"reasoning":     vote.Reasoning,
				}
				mu.Unlock()

				log.Printf("[EventResearchCouncil] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
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
			log.Printf("[EventResearchCouncil] No valid votes for suggestion %s", truncateID(suggestionID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		approved := 0
		revisionNeeded := 0
		blocked := 0
		var allConcerns []string

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
					if cm, ok := c.(map[string]interface{}); ok {
						if desc, ok := cm["description"].(string); ok && desc != "" {
							allConcerns = append(allConcerns, desc)
						} else if issue, ok := cm["issue"].(string); ok && issue != "" {
							allConcerns = append(allConcerns, issue)
						}
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

		log.Printf("[EventResearchCouncil] Consensus: %s (approved=%d, revision=%d, blocked=%d)", consensus, approved, revisionNeeded, blocked)

		switch consensus {
		case "approved":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "approved",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"reviews":           validReviews,
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s approved by council", truncateID(suggestionID))

		case "blocked":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "rejected",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"concerns":          allConcerns,
					"reviews":           validReviews,
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s blocked by council", truncateID(suggestionID))

		case "revision_needed":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "pending_human",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"concerns":          allConcerns,
					"reviews":           validReviews,
					"note":              "Council could not reach consensus, escalating to human",
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s needs human review", truncateID(suggestionID))
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
	})
}
