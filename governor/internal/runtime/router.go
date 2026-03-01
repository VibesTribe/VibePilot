package runtime

import (
	"context"
	"encoding/json"
	"log"

	"github.com/vibepilot/governor/internal/db"
)

type Router struct {
	cfg      *Config
	database *db.DB
}

func NewRouter(cfg *Config, database *db.DB) *Router {
	return &Router{
		cfg:      cfg,
		database: database,
	}
}

type RoutingRequest struct {
	AgentID          string
	TaskID           string
	TaskType         string
	RequiresCodebase bool
	Dependencies     []string
}

type RoutingResult struct {
	DestinationID string
	ModelID       string
	Category      string
	Strategy      string
	FallbackUsed  bool
}

func (r *Router) SelectDestination(ctx context.Context, req RoutingRequest) (*RoutingResult, error) {
	strategy := r.cfg.GetRoutingStrategy(req.AgentID)
	priority := r.cfg.GetStrategyPriority(strategy)

	log.Printf("[Router] Agent %s using strategy %s with priority %v", req.AgentID, strategy, priority)

	for _, category := range priority {
		dests := r.cfg.GetDestinationsInCategory(category)
		if len(dests) == 0 {
			log.Printf("[Router] No active destinations in category %s, trying next", category)
			continue
		}

		for _, dest := range dests {
			if r.isDestinationAvailable(ctx, dest.ID) {
				modelID := r.selectModelForDestination(ctx, dest.ID, req.TaskType)

				log.Printf("[Router] Selected destination %s (category: %s, model: %s)",
					dest.ID, category, modelID)

				return &RoutingResult{
					DestinationID: dest.ID,
					ModelID:       modelID,
					Category:      category,
					Strategy:      strategy,
					FallbackUsed:  false,
				}, nil
			}
		}
	}

	log.Printf("[Router] No available destination for agent %s task %s", req.AgentID, req.TaskID)
	return nil, nil
}

func (r *Router) isDestinationAvailable(ctx context.Context, destID string) bool {
	dest := r.cfg.GetDestination(destID)
	if dest == nil {
		return false
	}

	if dest.Status != "active" {
		return false
	}

	if dest.Type != "cli" && dest.Type != "api" {
		log.Printf("[Router] Skipping non-executable destination type: %s for %s", dest.Type, destID)
		return false
	}

	return true
}

func (r *Router) selectModelForDestination(ctx context.Context, destID string, taskType string) string {
	if r.cfg.Models == nil {
		return ""
	}

	var bestModel string
	var bestScore float64 = -1

	for _, model := range r.cfg.Models.Models {
		for _, accessDest := range model.AccessVia {
			if accessDest == destID && model.Status == "active" {
				score := r.getModelScore(ctx, model.ID, taskType)
				if score > bestScore {
					bestScore = score
					bestModel = model.ID
				}
			}
		}
	}

	return bestModel
}

func (r *Router) getModelScore(ctx context.Context, modelID string, taskType string) float64 {
	if r.database == nil {
		return 0.5
	}

	result, err := r.database.RPC(ctx, "get_model_score_for_task", map[string]any{
		"p_model_id":  modelID,
		"p_task_type": taskType,
	})
	if err != nil {
		return 0.5
	}

	var score float64 = 0.5
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err == nil {
		if s, ok := parsed["score"].(float64); ok {
			score = s
		}
	}

	return score
}

func (r *Router) GetAvailableDestinations() []string {
	if r.cfg.Destinations == nil {
		return nil
	}

	var result []string
	for _, dest := range r.cfg.Destinations.Destinations {
		if dest.Status == "active" {
			result = append(result, dest.ID)
		}
	}
	return result
}

func (r *Router) GetFallbackAction() string {
	if r.cfg.Routing == nil {
		return "pause_and_alert"
	}

	action, ok := r.cfg.Routing.Fallback["on_all_unavailable"]
	if !ok {
		return "pause_and_alert"
	}
	return action
}
