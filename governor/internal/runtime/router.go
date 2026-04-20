package runtime

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"sync/atomic"

	"github.com/vibepilot/governor/internal/db"
)

// defaultCourierEstimatedTokens is a conservative estimate for how many tokens
// a courier task will consume (prompt + response). Refined later when exact
// prompt size is known.
const defaultCourierEstimatedTokens = 4000

// defaultInternalEstimatedTokens is a conservative estimate for internal routing
// tasks, which are typically smaller than courier tasks (planner, supervisor, etc.).
const defaultInternalEstimatedTokens = 2000

type Router struct {
	cfg           *Config
	database      *db.DB
	usageTracker  *UsageTracker
	cascadeOffset uint64 // round-robin counter for model rotation
}

func NewRouter(cfg *Config, database *db.DB, usageTracker *UsageTracker) *Router {
	return &Router{
		cfg:          cfg,
		database:     database,
		usageTracker: usageTracker,
	}
}

type RoutingRequest struct {
	Role             string
	TaskType         string
	TaskCategory     string
	RoutingFlag      string
	RequiresCodebase bool
	Dependencies     []string
	ExcludeModels   []string // skip these models when cascading
}

type RoutingResult struct {
	ConnectorID string
	ModelID     string
	RoutingFlag string
	Category    string
	IsFallback  bool // true when routed to Hermes because all models in cooldown
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

	// Final dual-envelope gate: verify BOTH envelopes have headroom before
	// committing to courier dispatch. This catches edge cases where the
	// per-function checks passed but the combined state has changed (e.g.
	// concurrent requests consumed the remaining quota between checks).
	if r.usageTracker != nil {
		// Envelope A: fueling model + connector API rate limits
		decision := r.usageTracker.CanMakeRequestVia(ctx, courierModel, courierConn, defaultCourierEstimatedTokens)
		if !decision.CanProceed {
			log.Printf("[Router] Dual-envelope abort: envelope A failed for model=%s conn=%s (%s, wait %v)",
				courierModel, courierConn, decision.Reason, decision.WaitTime)
			return nil
		}

		// Envelope B: web platform free-tier limits
		canProceed, waitTime := r.usageTracker.PlatformCanMakeRequest(ctx, dest.PlatformID, defaultCourierEstimatedTokens)
		if !canProceed {
			log.Printf("[Router] Dual-envelope abort: envelope B failed for platform=%s (wait %v)",
				dest.PlatformID, waitTime)
			return nil
		}
	}

	log.Printf("[Router] Web routing: connector=%s model=%s destination=%s (dual-envelope OK)", courierConn, courierModel, dest.PlatformID)

	return &RoutingResult{
		ConnectorID: courierConn,
		ModelID:     courierModel,
		RoutingFlag: "web",
		Category:    "external",
	}
}

func (r *Router) selectInternal(ctx context.Context, req RoutingRequest) (*RoutingResult, error) {
	// If routing for a specific agent (role), use that agent's configured model
	var agentModelID string
	if req.Role != "" {
		agent := r.cfg.GetAgent(req.Role)
		if agent != nil && agent.Model != "" {
			agentModelID = agent.Model
			log.Printf("[Router] Agent %s configured with model %s", req.Role, agentModelID)
		}
	}

	connectors := r.cfg.GetConnectorsInCategory("internal")
	if len(connectors) == 0 {
		log.Printf("[Router] No internal connectors available")
		return nil, nil
	}

	// Prefer API connectors over CLI for toolless agents (planner, supervisor, tester, council)
	// CLI connectors are heavier (spawn full agent sessions) and only needed for executor tasks
	needsTools := r.agentNeedsTools(req.Role)
	if !needsTools {
		sort.Slice(connectors, func(i, j int) bool {
			ti, tj := connectors[i].Type, connectors[j].Type
			// api first, cli second, everything else last
			order := func(t string) int {
				switch t {
				case "api":
					return 0
				case "cli":
					return 1
				default:
					return 2
				}
			}
			return order(ti) < order(tj)
		})
	}

	// If we have an agent-specific model, check if it's available first
	if agentModelID != "" {
		if r.usageTracker != nil {
			// Find connector for agent model first
			agentConnID := ""
			for i := range connectors {
				conn := &connectors[i]
				if r.isConnectorExecutable(conn) && r.canConnectorAccessModel(conn.ID, agentModelID) {
					agentConnID = conn.ID
					break
				}
			}
			decision := r.usageTracker.CanMakeRequestVia(ctx, agentModelID, agentConnID, defaultInternalEstimatedTokens)
			if !decision.CanProceed {
				log.Printf("[Router] Agent %s model %s unavailable via connector %s (%s), falling through to cascade", req.Role, agentModelID, agentConnID, decision.Reason)
			} else if agentConnID != "" {
				log.Printf("[Router] Internal routing: connector=%s model=%s (from agent %s)", agentConnID, agentModelID, req.Role)
				return &RoutingResult{
					ConnectorID: agentConnID,
					ModelID:     agentModelID,
					RoutingFlag: "internal",
					Category:    "internal",
				}, nil
			}
		} else {
			// No usage tracker, route directly to pinned model
			for i := range connectors {
				conn := &connectors[i]
				if !r.isConnectorExecutable(conn) {
					continue
				}
				if r.canConnectorAccessModel(conn.ID, agentModelID) {
					log.Printf("[Router] Internal routing: connector=%s model=%s (from agent %s)", conn.ID, agentModelID, req.Role)
					return &RoutingResult{
						ConnectorID: conn.ID,
						ModelID:     agentModelID,
						RoutingFlag: "internal",
						Category:    "internal",
					}, nil
				}
			}
		}
	}

	// Use cascade-aware model selection via UsageTracker
	cascade := r.getModelCascade()
	if r.usageTracker != nil && len(cascade) > 0 {
		// Filter out excluded models for cascade retry
		if len(req.ExcludeModels) > 0 {
			log.Printf("[Router] Excluding models %v from cascade (was %d models)", req.ExcludeModels, len(cascade))
			filtered := make([]string, 0, len(cascade))
			for _, m := range cascade {
				excluded := false
				for _, ex := range req.ExcludeModels {
					if m == ex {
						excluded = true
						break
					}
				}
				if !excluded {
					filtered = append(filtered, m)
				}
			}
			cascade = filtered
		}
		return r.selectByCascade(ctx, connectors, cascade)
	}

	// Fallback: original task-based model selection
	for i := range connectors {
		conn := &connectors[i]
		if !r.isConnectorExecutable(conn) {
			continue
		}

		modelID := r.selectModelForConnector(ctx, conn.ID, req.TaskType, req.TaskCategory)
		if modelID == "" {
			continue
		}

		// Connector-aware rate limit check: skip models whose connectors
		// have hit shared limits (e.g., Groq org TPD across all models).
		if r.usageTracker != nil {
			decision := r.usageTracker.CanMakeRequestVia(ctx, modelID, conn.ID, defaultInternalEstimatedTokens)
			if !decision.CanProceed {
				log.Printf("[Router] Internal fallback skip: model=%s connector=%s reason=%s wait=%v", modelID, conn.ID, decision.Reason, decision.WaitTime)
				continue
			}
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

// getModelCascade returns the model cascade order from routing config.
// It looks for a "free_cascade" strategy and falls back to "default" strategy priority.
func (r *Router) getModelCascade() []string {
	if r.cfg == nil || r.cfg.Routing == nil {
		return nil
	}

	// Prefer "free_cascade" strategy if available
	if strategy, ok := r.cfg.Routing.Strategies["free_cascade"]; ok && len(strategy.Priority) > 0 {
		return strategy.Priority
	}

	// Fall back to default strategy
	if r.cfg.Routing.DefaultStrategy != "" {
		return r.cfg.GetStrategyPriority(r.cfg.Routing.DefaultStrategy)
	}

	return nil
}

// selectByCascade picks the least-loaded available model from the cascade.
// Uses round-robin rotation to distribute load across models.
// If all models are in cooldown, falls back to Hermes (the human's agent).
func (r *Router) selectByCascade(ctx context.Context, connectors []ConnectorConfig, cascade []string) (*RoutingResult, error) {
	if len(cascade) == 0 {
		return nil, nil
	}

	// Advance round-robin offset so each call starts from a different position
	offset := atomic.AddUint64(&r.cascadeOffset, 1)
	startIdx := int(offset % uint64(len(cascade)))

	var shortestCooldownModel string
	var shortestCooldownSecs int = -1

	// Collect all available candidates, then pick least-loaded
	type candidate struct {
		modelID string
		connID  string
	}
	var candidates []candidate

	for i := 0; i < len(cascade); i++ {
		idx := (startIdx + i) % len(cascade)
		modelID := cascade[idx]

		// Find connector first so we can check connector-specific limits
		connID := r.findConnectorForModel(modelID)
		if connID == "" {
			for j := range connectors {
				conn := &connectors[j]
				if r.isConnectorExecutable(conn) && r.canConnectorAccessModel(conn.ID, modelID) {
					connID = conn.ID
					break
				}
			}
		}
		if connID == "" {
			continue // No connector available for this model
		}

		decision := r.usageTracker.CanMakeRequestVia(ctx, modelID, connID, defaultInternalEstimatedTokens)
		if decision.CanProceed {
			candidates = append(candidates, candidate{modelID: modelID, connID: connID})
			continue
		}

		// Log the reason for skipping this model (connector-aware limit check)
		log.Printf("[Router] Cascade skip: model=%s connector=%s reason=%s wait=%v", modelID, connID, decision.Reason, decision.WaitTime)

		// Track model with shortest cooldown for fallback info
		waitSecs := int(decision.WaitTime.Seconds())
		if shortestCooldownSecs < 0 || waitSecs < shortestCooldownSecs {
			shortestCooldownSecs = waitSecs
			shortestCooldownModel = modelID
		}
	}

	if len(candidates) > 0 {
		// Pick the candidate with fewest recent requests (load-balancing)
		best := candidates[0]
		bestCount := r.usageTracker.GetMinuteRequestCount(ctx, best.modelID)
		for _, c := range candidates[1:] {
			count := r.usageTracker.GetMinuteRequestCount(ctx, c.modelID)
			if count < bestCount {
				best = c
				bestCount = count
			}
		}
		log.Printf("[Router] Cascade routing: model=%s connector=%s (%d candidates available, picked least-loaded with %d min-requests)",
			best.modelID, best.connID, len(candidates), bestCount)
		return &RoutingResult{
			ConnectorID: best.connID,
			ModelID:     best.modelID,
			RoutingFlag: "internal",
			Category:    "internal",
		}, nil
	}

	// All models in cooldown — fall back to Hermes connector (the human's agent)
	// Hermes is always available as last resort, no rate limits apply to it
	for i := range connectors {
		conn := &connectors[i]
		if conn.ID == "hermes" && r.isConnectorExecutable(conn) {
			log.Printf("[Router] All %d models in cooldown (shortest: %s at %ds). Falling back to Hermes.",
				len(cascade), shortestCooldownModel, shortestCooldownSecs)
			return &RoutingResult{
				ConnectorID: "hermes",
				ModelID:     "glm-5",
				RoutingFlag: "internal",
				Category:    "internal",
				IsFallback:  true,
			}, nil
		}
	}

	// Not even Hermes available (should never happen)
	if shortestCooldownModel != "" {
		log.Printf("[Router] All models in cooldown and no Hermes fallback. Shortest wait: model=%s (%ds).", shortestCooldownModel, shortestCooldownSecs)
	}
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

		// Envelope A: Check fueling model + connector rate limit headroom.
		// findConnectorForModel already checks rate limits and returns ""
		// if no connector has headroom. We verify explicitly here too so
		// we can log the reason for skipping.
		connID := r.findConnectorForModel(model.ID)
		if connID == "" {
			log.Printf("[Router] Skipping courier model %s: no connector with envelope A headroom", model.ID)
			continue
		}
		if r.usageTracker != nil {
			decision := r.usageTracker.CanMakeRequestVia(ctx, model.ID, connID, defaultCourierEstimatedTokens)
			if !decision.CanProceed {
				log.Printf("[Router] Skipping courier model %s: envelope A failed (%s)", model.ID, decision.Reason)
				continue
			}
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
			// Envelope A: Verify the model+connector has rate limit headroom.
			if r.usageTracker != nil {
				decision := r.usageTracker.CanMakeRequestVia(context.Background(), modelID, conn.ID, defaultCourierEstimatedTokens)
				if !decision.CanProceed {
					log.Printf("[Router] Skipping connector %s for model %s: envelope A failed (%s)", conn.ID, modelID, decision.Reason)
					continue
				}
			}
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

		// Envelope B (primary): Check in-memory platform free-tier limits.
		// This is the proactive check so we don't dispatch to an exhausted platform.
		if r.usageTracker != nil {
			canProceed, waitTime := r.usageTracker.PlatformCanMakeRequest(ctx, conn.ID, 0) // 0 tokens = just check message limits
			if !canProceed {
				log.Printf("[Router] Skipping destination %s: envelope B platform limit hit (wait %v)", conn.ID, waitTime)
				continue
			}
		}

		// Secondary validation: Supabase RPC check for platform availability.
		// Kept as a backup / external source of truth.
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

func (r *Router) canConnectorAccessModel(connectorID string, modelID string) bool {
	if r.cfg.Models == nil {
		return false
	}

	for _, model := range r.cfg.Models.Models {
		if model.ID == modelID {
			for _, accessConn := range model.AccessVia {
				if accessConn == connectorID {
					return true
				}
			}
			return false
		}
	}
	return false
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

// GetAvailableModelCount returns the number of active models across all active connectors.
func (r *Router) GetAvailableModelCount() int {
	if r.cfg.Connectors == nil || r.cfg.Models == nil {
		return 0
	}
	count := 0
	activeConnectors := make(map[string]bool)
	for _, conn := range r.cfg.Connectors.Connectors {
		if conn.Status == "active" && (conn.Type == "cli" || conn.Type == "api") {
			activeConnectors[conn.ID] = true
		}
	}
	for _, model := range r.cfg.Models.Models {
		if model.Status != "active" {
			continue
		}
		for _, via := range model.AccessVia {
			if activeConnectors[via] {
				count++
				break
			}
		}
	}
	return count
}

// agentNeedsTools returns true for agents that need CLI connector with tools.
// Planner/supervisor/tester/council receive file content in their prompt from the governor,
// so they can use lightweight API connectors instead of spawning full CLI sessions.
func (r *Router) agentNeedsTools(role string) bool {
	switch role {
	case "task_runner", "maintenance":
		return true
	default:
		return false
	}
}
