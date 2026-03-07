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
	Role             string
	TaskType         string
	TaskCategory     string
	RoutingFlag      string
	RequiresCodebase bool
	Dependencies     []string
}

type RoutingResult struct {
	ConnectorID string
	ModelID     string
	RoutingFlag string
	Category    string
}

func (r *Router) SelectRouting(ctx context.Context, req RoutingRequest) (*RoutingResult, error) {
	if req.RoutingFlag == "internal" {
		log.Printf("[Router] Planner flagged internal → internal")
		return r.selectInternal(ctx, req)
	}

	if !r.hasCourierAvailable(ctx) {
		log.Printf("[Router] No courier agent → internal")
		return r.selectInternal(ctx, req)
	}

	result := r.tryWebRouting(ctx, req)
	if result != nil {
		return result, nil
	}

	log.Printf("[Router] No web platform available → internal")
	return r.selectInternal(ctx, req)
}

func (r *Router) hasCourierAvailable(ctx context.Context) bool {
	courierModel := r.selectCourierModel(ctx)
	if courierModel == "" {
		return false
	}
	courierConn := r.findConnectorForModel(courierModel)
	return courierConn != ""
}

func (r *Router) tryWebRouting(ctx context.Context, req RoutingRequest) *RoutingResult {
	courierModel := r.selectCourierModel(ctx)
	if courierModel == "" {
		log.Printf("[Router] No courier model available")
		return nil
	}

	courierConn := r.findConnectorForModel(courierModel)
	if courierConn == "" {
		log.Printf("[Router] No connector for courier model %s", courierModel)
		return nil
	}

	dest := r.selectDestination(ctx, req.TaskType, req.TaskCategory)
	if dest == nil {
		log.Printf("[Router] No web destination available")
		return nil
	}

	log.Printf("[Router] Web routing: connector=%s model=%s destination=%s", courierConn, courierModel, dest.PlatformID)

	return &RoutingResult{
		ConnectorID: courierConn,
		ModelID:     courierModel,
		RoutingFlag: "web",
		Category:    "external",
	}
}

func (r *Router) selectInternal(ctx context.Context, req RoutingRequest) (*RoutingResult, error) {
	connectors := r.cfg.GetConnectorsInCategory("internal")
	if len(connectors) == 0 {
		log.Printf("[Router] No internal connectors available")
		return nil, nil
	}

	for i := range connectors {
		conn := &connectors[i]
		if !r.isConnectorExecutable(conn) {
			continue
		}

		modelID := r.selectModelForConnector(ctx, conn.ID, req.TaskType, req.TaskCategory)
		if modelID == "" {
			continue
		}

		log.Printf("[Router] Internal routing: connector=%s model=%s", conn.ID, modelID)

		return &RoutingResult{
			ConnectorID: conn.ID,
			ModelID:     modelID,
			RoutingFlag: "internal",
			Category:    "internal",
		}, nil
	}

	log.Printf("[Router] No internal routing available for role %s", req.Role)
	return nil, nil
}

func (r *Router) selectCourierModel(ctx context.Context) string {
	if r.cfg.Models == nil {
		return ""
	}

	var bestModel string
	var bestScore float64 = -1

	for _, model := range r.cfg.Models.Models {
		if model.Status != "active" {
			continue
		}

		hasBrowser := false
		for _, cap := range model.Capabilities {
			if cap == "vision" || cap == "browser" {
				hasBrowser = true
				break
			}
		}
		if !hasBrowser {
			continue
		}

		score := r.getModelScore(ctx, model.ID, "courier", "browser")
		if score > bestScore {
			bestScore = score
			bestModel = model.ID
		}
	}

	return bestModel
}

func (r *Router) findConnectorForModel(modelID string) string {
	if r.cfg.Models == nil || r.cfg.Connectors == nil {
		return ""
	}

	var model *ModelConfig
	for i := range r.cfg.Models.Models {
		if r.cfg.Models.Models[i].ID == modelID {
			model = &r.cfg.Models.Models[i]
			break
		}
	}
	if model == nil {
		return ""
	}

	for _, accessConn := range model.AccessVia {
		conn := r.cfg.GetConnector(accessConn)
		if conn != nil && conn.Status == "active" && r.isConnectorExecutable(conn) {
			return conn.ID
		}
	}

	return ""
}

func (r *Router) selectDestination(ctx context.Context, taskType string, taskCategory string) *PlatformDestination {
	if r.cfg.Connectors == nil {
		return nil
	}

	for _, conn := range r.cfg.Connectors.Connectors {
		if conn.Type != "web" || conn.Status != "active" {
			continue
		}

		if !r.isDestinationAvailable(ctx, conn.ID) {
			continue
		}

		url, _ := conn.Extra["url"].(string)
		if url == "" {
			continue
		}

		log.Printf("[Router] Selected destination: %s for taskType=%s", conn.ID, taskType)

		return &PlatformDestination{
			PlatformID: conn.ID,
			URL:        url,
		}
	}

	return nil
}

type PlatformDestination struct {
	PlatformID string
	URL        string
}

func (r *Router) isDestinationAvailable(ctx context.Context, destinationID string) bool {
	if r.database == nil {
		return true
	}

	result, err := r.database.RPC(ctx, "check_platform_availability", map[string]any{
		"p_platform_id": destinationID,
	})
	if err != nil {
		log.Printf("[Router] Failed to check platform availability: %v", err)
		return true
	}

	var available bool = true
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err == nil {
		if a, ok := parsed["available"].(bool); ok {
			available = a
		}
	}

	return available
}

func (r *Router) isConnectorExecutable(conn *ConnectorConfig) bool {
	if conn.Status != "active" {
		return false
	}
	return conn.Type == "cli" || conn.Type == "api"
}

func (r *Router) selectModelForConnector(ctx context.Context, connectorID string, taskType string, taskCategory string) string {
	if r.cfg.Models == nil {
		return ""
	}

	var bestModel string
	var bestScore float64 = -1

	for _, model := range r.cfg.Models.Models {
		if model.Status != "active" {
			continue
		}

		canAccess := false
		for _, accessConn := range model.AccessVia {
			if accessConn == connectorID {
				canAccess = true
				break
			}
		}
		if !canAccess {
			continue
		}

		score := r.getModelScore(ctx, model.ID, taskType, taskCategory)
		if score > bestScore {
			bestScore = score
			bestModel = model.ID
		}
	}

	return bestModel
}

func (r *Router) getModelScore(ctx context.Context, modelID string, taskType string, taskCategory string) float64 {
	if r.database == nil {
		return 0.5
	}

	result, err := r.database.RPC(ctx, "get_model_score_for_task", map[string]any{
		"p_model_id":      modelID,
		"p_task_type":     taskType,
		"p_task_category": taskCategory,
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

func (r *Router) GetConnector(id string) *ConnectorConfig {
	return r.cfg.GetConnector(id)
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

type LegacyRoutingRequest struct {
	AgentID          string
	TaskID           string
	TaskType         string
	RequiresCodebase bool
	Dependencies     []string
}

type LegacyRoutingResult struct {
	DestinationID string
	ModelID       string
	Category      string
	Strategy      string
}

func (r *Router) SelectDestination(ctx context.Context, req LegacyRoutingRequest) (*LegacyRoutingResult, error) {
	newReq := RoutingRequest{
		Role:             req.AgentID,
		TaskType:         req.TaskType,
		RoutingFlag:      "",
		RequiresCodebase: req.RequiresCodebase,
	}

	result, err := r.SelectRouting(ctx, newReq)
	if err != nil || result == nil {
		return nil, err
	}

	return &LegacyRoutingResult{
		DestinationID: result.ConnectorID,
		ModelID:       result.ModelID,
		Category:      result.Category,
		Strategy:      result.RoutingFlag,
	}, nil
}

func (r *Router) GetAvailableConnectors() []string {
	if r.cfg.Connectors == nil {
		return nil
	}
	var result []string
	for _, conn := range r.cfg.Connectors.Connectors {
		if conn.Status == "active" && (conn.Type == "cli" || conn.Type == "api") {
			result = append(result, conn.ID)
		}
	}
	return result
}
