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

type ResearchHandler struct {
	database   *db.DB
	factory    *runtime.SessionFactory
	pool       *runtime.AgentPool
	connRouter *runtime.Router
	cfg        *runtime.Config
}

func NewResearchHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
) *ResearchHandler {
	return &ResearchHandler{
		database:   database,
		factory:    factory,
		pool:       pool,
		connRouter: connRouter,
		cfg:        cfg,
	}
}

func (h *ResearchHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventResearchReady, h.handleResearchReady)
	router.On(runtime.EventResearchCouncil, h.handleResearchCouncil)
}

func (h *ResearchHandler) handleResearchReady(event runtime.Event) {
	ctx := context.Background()

	var suggestion map[string]any
	if err := json.Unmarshal(event.Record, &suggestion); err != nil {
		log.Printf("[ResearchReady] Failed to parse event: %v", err)
		return
	}

	suggestionID := getString(suggestion, "id")
	suggestionType := getString(suggestion, "type")
	complexity := getString(suggestion, "complexity")

	if suggestionID == "" {
		return
	}

	processingBy := fmt.Sprintf("research_ready:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "research_suggestions",
		"p_id":            suggestionID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[ResearchReady] Suggestion %s already being processed", truncateID(suggestionID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "research_suggestions",
		"p_id":    suggestionID,
	})

	log.Printf("[ResearchReady] Processing %s (type: %s, complexity: %s)", truncateID(suggestionID), suggestionType, complexity)

	switch complexity {
	case "human":
		log.Printf("[ResearchReady] Human review required for %s", truncateID(suggestionID))
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":           suggestionID,
			"p_status":       "pending_human",
			"p_review_notes": map[string]any{"reason": "complexity=human, requires human decision"},
		})
		return

	case "complex":
		log.Printf("[ResearchReady] Complex item %s - routing to council", truncateID(suggestionID))
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":           suggestionID,
			"p_status":       "council_review",
			"p_review_notes": map[string]any{"source": "research", "type": suggestionType},
		})
		return
	}

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "supervisor",
		TaskID:   suggestionID,
		TaskType: "research_review",
	})
	if err != nil || routingResult == nil {
		log.Printf("[ResearchReady] No destination for %s", truncateID(suggestionID))
		return
	}

	session, err := h.factory.CreateWithContext(ctx, "supervisor", "research_review")
	if err != nil {
		log.Printf("[ResearchReady] Failed to create session for %s: %v", truncateID(suggestionID), err)
		return
	}

	err = h.pool.SubmitWithDestination(ctx, "research", routingResult.DestinationID, func() error {
		result, err := session.Run(ctx, map[string]any{
			"event":      "research_review",
			"suggestion": suggestion,
		})
		if err != nil {
			return err
		}

		review, parseErr := runtime.ParseResearchReview(result.Output)
		if parseErr != nil {
			log.Printf("[ResearchReady] Failed to parse review: %v", parseErr)
			return nil
		}

		log.Printf("[ResearchReady] Suggestion %s review: decision=%s", truncateID(suggestionID), review.Decision)

		switch review.Decision {
		case "approved":
			if review.MaintenanceCommand != nil {
				cmdJSON, _ := json.Marshal(review.MaintenanceCommand.Details)
				_, err := h.database.RPC(ctx, "create_maintenance_command", map[string]any{
					"p_command_type": review.MaintenanceCommand.Action,
					"p_payload":      json.RawMessage(cmdJSON),
					"p_source":       "research_review",
					"p_approved_by":  "supervisor",
				})
				if err != nil {
					log.Printf("[ResearchReady] Failed to create maintenance command: %v", err)
				} else {
					log.Printf("[ResearchReady] Created maintenance command: %s", review.MaintenanceCommand.Action)
				}
			}
			_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "approved",
				"p_review_notes": map[string]any{
					"reasoning": review.Reasoning,
					"notes":     review.Notes,
				},
			})

		case "rejected":
			_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "rejected",
				"p_review_notes": map[string]any{
					"reasoning": review.Reasoning,
				},
			})

		case "council_review":
			_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "council_review",
				"p_review_notes": map[string]any{
					"reasoning": review.Reasoning,
					"source":    "research",
				},
			})

		case "human_review":
			_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
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
		log.Printf("[ResearchReady] Failed to submit: %v", err)
	}
}

func (h *ResearchHandler) handleResearchCouncil(event runtime.Event) {
	ctx := context.Background()

	var suggestion map[string]any
	if err := json.Unmarshal(event.Record, &suggestion); err != nil {
		log.Printf("[ResearchCouncil] Failed to parse event: %v", err)
		return
	}

	suggestionID := getString(suggestion, "id")

	if suggestionID == "" {
		return
	}

	processingBy := fmt.Sprintf("research_council:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "research_suggestions",
		"p_id":            suggestionID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[ResearchCouncil] Suggestion %s already being processed", truncateID(suggestionID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "research_suggestions",
		"p_id":    suggestionID,
	})

	log.Printf("[ResearchCouncil] Starting council review for %s", truncateID(suggestionID))

	memberCount := h.cfg.GetCouncilMemberCount()
	lenses := h.cfg.GetCouncilLenses()
	if len(lenses) == 0 {
		lenses = []string{"user_alignment", "architecture", "feasibility"}
	}
	if memberCount <= 0 {
		memberCount = 3
	}

	routingResult, err := h.connRouter.SelectDestination(ctx, runtime.LegacyRoutingRequest{
		AgentID:  "council",
		TaskID:   suggestionID,
		TaskType: "research_council",
	})
	if err != nil || routingResult == nil {
		log.Printf("[ResearchCouncil] No destination for %s", truncateID(suggestionID))
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":           suggestionID,
			"p_status":       "pending_human",
			"p_review_notes": map[string]any{"error": "no_destination"},
		})
		return
	}

	reviews := make([]map[string]any, memberCount)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < memberCount; i++ {
		wg.Add(1)
		go func(memberIndex int) {
			defer wg.Done()

			lens := lenses[memberIndex%len(lenses)]
			session, err := h.factory.CreateWithContext(ctx, "council", lens)
			if err != nil {
				log.Printf("[ResearchCouncil] Failed to create session for member %d: %v", memberIndex+1, err)
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
				log.Printf("[ResearchCouncil] Member %d failed: %v", memberIndex+1, err)
				return
			}

			vote, parseErr := runtime.ParseCouncilVote(result.Output)
			if parseErr != nil {
				log.Printf("[ResearchCouncil] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
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

			log.Printf("[ResearchCouncil] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
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
		log.Printf("[ResearchCouncil] No valid votes for suggestion %s", truncateID(suggestionID))
		return
	}

	approved := 0
	revisionNeeded := 0
	blocked := 0
	var allConcerns []string

	for _, r := range validReviews {
		vote := getString(r, "vote")
		switch vote {
		case "APPROVED", "approved":
			approved++
		case "REVISION_NEEDED", "revision_needed":
			revisionNeeded++
		case "BLOCKED", "blocked":
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
				} else if s, ok := c.(string); ok && s != "" {
					allConcerns = append(allConcerns, s)
				}
			}
		}
	}

	consensusMethod := h.cfg.GetConsensusMethod()
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

	log.Printf("[ResearchCouncil] Consensus: %s (approved=%d, revision=%d, blocked=%d)", consensus, approved, revisionNeeded, blocked)

	switch consensus {
	case "approved":
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":     suggestionID,
			"p_status": "approved",
			"p_review_notes": map[string]any{
				"council_consensus": consensus,
				"reviews":           validReviews,
			},
		})
		log.Printf("[ResearchCouncil] %s approved by council", truncateID(suggestionID))

	case "blocked":
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":     suggestionID,
			"p_status": "rejected",
			"p_review_notes": map[string]any{
				"council_consensus": consensus,
				"concerns":          allConcerns,
				"reviews":           validReviews,
			},
		})
		log.Printf("[ResearchCouncil] %s blocked by council", truncateID(suggestionID))

	case "revision_needed":
		_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
			"p_id":     suggestionID,
			"p_status": "pending_human",
			"p_review_notes": map[string]any{
				"council_consensus": consensus,
				"concerns":          allConcerns,
				"reviews":           validReviews,
				"note":              "Council could not reach consensus, escalating to human",
			},
		})
		log.Printf("[ResearchCouncil] %s needs human review", truncateID(suggestionID))
	}
}

func setupResearchHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
) {
	handler := NewResearchHandler(database, factory, pool, connRouter, cfg)
	handler.Register(router)
}
