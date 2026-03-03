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
		conns := r.cfg.GetConnectorsInCategory(category)
		if len(conns) == 0 {
			log.Printf("[Router] No active connectors in category %s, trying next", category)
			continue
		}

		for _, conn := range conns {
			if r.isConnectorAvailable(ctx, conn.ID) {
				modelID := r.selectModelForConnector(ctx, conn.ID, req.TaskType)

				log.Printf("[Router] Selected connector %s (category: %s, model: %s)",
					conn.ID, category, modelID)

				return &RoutingResult{
					DestinationID: conn.ID,
					ModelID:       modelID,
					Category:      category,
					Strategy:      strategy,
					FallbackUsed:  false,
				}, nil
			}
		}
	}

	log.Printf("[Router] No available connector for agent %s task %s", req.AgentID, req.TaskID)
	return nil, nil
}

func (r *Router) isConnectorAvailable(ctx context.Context, connID string) bool {
	conn := r.cfg.GetConnector(connID)
	if conn == nil {
		return false
	}

	if conn.Status != "active" {
		return false
	}

	if conn.Type != "cli" && conn.Type != "api" {
		log.Printf("[Router] Skipping non-executable connector type: %s for %s", conn.Type, connID)
		return false
	}

	return true
}

func (r *Router) selectModelForConnector(ctx context.Context, connID string, taskType string) string {
	if r.cfg.Models == nil {
		return ""
	}

	var bestModel string
	var bestScore float64 = -1

	for _, model := range r.cfg.Models.Models {
		for _, accessConn := range model.AccessVia {
			if accessConn == connID && model.Status == "active" {
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

func (r *Router) GetAvailableConnectors() []string {
	if r.cfg.Connectors == nil {
		return nil
	}

	var result []string
	for _, conn := range r.cfg.Connectors.Connectors {
		if conn.Status == "active" {
			result = append(result, conn.ID)
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
