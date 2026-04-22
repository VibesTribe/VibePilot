package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/vibepilot/governor/internal/db"
)

// ResearchActionApplier handles direct application of approved research findings
// to both config files and database, ensuring they stay in sync.
// No LLM middleman — this is deterministic code execution.
type ResearchActionApplier struct {
	configDir string
	db        db.Database
	mu        sync.Mutex
}

// ModelAction represents a model change from research findings.
type ModelAction struct {
	Action string       `json:"action"` // "add_model", "update_model", "remove_model", "bench_model", "unbench_model"
	Model  ModelProfile `json:"model"`
	Reason string       `json:"reason,omitempty"`
}

func NewResearchActionApplier(configDir string, database db.Database) *ResearchActionApplier {
	return &ResearchActionApplier{
		configDir: configDir,
		db:        database,
	}
}

// ApplyResearchAction applies an approved research action to both config and DB.
// Returns a human-readable summary of what was done.
func (a *ResearchActionApplier) ApplyResearchAction(ctx context.Context, suggestionType string, details map[string]interface{}) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch suggestionType {
	case "new_model", "pricing_change", "config_tweak":
		return a.applyModelAction(ctx, details)
	case "new_platform":
		return a.applyPlatformAction(ctx, details)
	default:
		return "", fmt.Errorf("unsupported research action type: %s", suggestionType)
	}
}

func (a *ResearchActionApplier) applyModelAction(ctx context.Context, details map[string]interface{}) (string, error) {
	action, _ := details["action"].(string)
	if action == "" {
		action = "add_model"
	}

	modelData, ok := details["model"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("research action missing 'model' object")
	}

	profile := modelDataToProfile(modelData)

	switch action {
	case "add_model", "update_model":
		return a.addOrUpdateModel(ctx, profile, action)
	case "remove_model":
		return a.removeModel(ctx, profile.ID, details)
	case "bench_model":
		return a.benchModel(ctx, profile.ID, details)
	case "unbench_model":
		return a.unbenchModel(ctx, profile.ID)
	default:
		return "", fmt.Errorf("unknown model action: %s", action)
	}
}

func (a *ResearchActionApplier) addOrUpdateModel(ctx context.Context, profile ModelProfile, action string) (string, error) {
	// 1. Write to config/models.json
	configPath := filepath.Join(a.configDir, "models.json")
	file, err := a.readModelsConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	found := false
	for i, m := range file.Models {
		if m.ID == profile.ID {
			file.Models[i] = profile
			found = true
			break
		}
	}
	if !found {
		file.Models = append(file.Models, profile)
	}

	if err := a.writeModelsConfig(configPath, file); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	// 2. Sync to DB (same as model_loader.go syncToDatabase)
	if err := a.syncModelToDB(ctx, profile); err != nil {
		return "", fmt.Errorf("sync to DB: %w", err)
	}

	verb := "Added"
	if found {
		verb = "Updated"
	}
	summary := fmt.Sprintf("%s model %s in config + DB", verb, profile.ID)
	log.Printf("[ResearchAction] %s", summary)
	return summary, nil
}

func (a *ResearchActionApplier) removeModel(ctx context.Context, modelID string, details map[string]interface{}) (string, error) {
	// Don't truly delete — bench it with reason
	reason := "removed by research"
	if r, ok := details["reason"].(string); ok && r != "" {
		reason = r
	}

	// 1. Update config
	configPath := filepath.Join(a.configDir, "models.json")
	file, err := a.readModelsConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	found := false
	for i, m := range file.Models {
		if m.ID == modelID {
			file.Models[i].Status = "benched"
			file.Models[i].StatusReason = reason
			found = true
			break
		}
	}
	if !found {
		// Model not in config but might be in DB — still bench in DB
		log.Printf("[ResearchAction] Model %s not in config, benching in DB only", modelID)
	}

	if found {
		if err := a.writeModelsConfig(configPath, file); err != nil {
			return "", fmt.Errorf("write config: %w", err)
		}
	}

	// 2. Bench in DB
	_, err = a.db.Update(ctx, "models", modelID, map[string]interface{}{
		"status":        "benched",
		"status_reason": reason,
	})
	if err != nil {
		return "", fmt.Errorf("bench in DB: %w", err)
	}

	summary := fmt.Sprintf("Benched model %s: %s", modelID, reason)
	log.Printf("[ResearchAction] %s", summary)
	return summary, nil
}

func (a *ResearchActionApplier) benchModel(ctx context.Context, modelID string, details map[string]interface{}) (string, error) {
	reason := "benched by research"
	if r, ok := details["reason"].(string); ok && r != "" {
		reason = r
	}

	// Update config
	configPath := filepath.Join(a.configDir, "models.json")
	file, err := a.readModelsConfig(configPath)
	if err == nil {
		for i, m := range file.Models {
			if m.ID == modelID {
				file.Models[i].Status = "benched"
				file.Models[i].StatusReason = reason
				break
			}
		}
		a.writeModelsConfig(configPath, file)
	}

	// Update DB
	_, err = a.db.Update(ctx, "models", modelID, map[string]interface{}{
		"status":        "benched",
		"status_reason": reason,
	})
	if err != nil {
		return "", fmt.Errorf("bench in DB: %w", err)
	}

	return fmt.Sprintf("Benched %s: %s", modelID, reason), nil
}

func (a *ResearchActionApplier) unbenchModel(ctx context.Context, modelID string) (string, error) {
	// Update config
	configPath := filepath.Join(a.configDir, "models.json")
	file, err := a.readModelsConfig(configPath)
	if err == nil {
		for i, m := range file.Models {
			if m.ID == modelID {
				file.Models[i].Status = "active"
				file.Models[i].StatusReason = ""
				break
			}
		}
		a.writeModelsConfig(configPath, file)
	}

	// Update DB
	_, err = a.db.Update(ctx, "models", modelID, map[string]interface{}{
		"status":        "active",
		"status_reason": "",
	})
	if err != nil {
		return "", fmt.Errorf("unbench in DB: %w", err)
	}

	return fmt.Sprintf("Activated %s", modelID), nil
}

func (a *ResearchActionApplier) applyPlatformAction(ctx context.Context, details map[string]interface{}) (string, error) {
	action, _ := details["action"].(string)
	if action == "" {
		action = "add_platform"
	}

	platformData, ok := details["platform"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("research action missing 'platform' object")
	}

	conn := mapDataToConnector(platformData)
	connectorsPath := filepath.Join(a.configDir, "connectors.json")

	file, err := a.readConnectorsConfig(connectorsPath)
	if err != nil {
		return "", fmt.Errorf("read connectors: %w", err)
	}

	switch action {
	case "add_platform":
		found := false
		for _, c := range file.Connectors {
			if c.ID == conn.ID {
				found = true
				break
			}
		}
		if !found {
			file.Connectors = append(file.Connectors, conn)
		}

	case "update_platform":
		for i, c := range file.Connectors {
			if c.ID == conn.ID {
				file.Connectors[i] = conn
				break
			}
		}

	case "remove_platform":
		connectorID := conn.ID
		if id, ok := details["platform_id"].(string); ok {
			connectorID = id
		}
		for i, c := range file.Connectors {
			if c.ID == connectorID {
				file.Connectors = append(file.Connectors[:i], file.Connectors[i+1:]...)
				break
			}
		}
	}

	if err := a.writeConnectorsConfig(connectorsPath, file); err != nil {
		return "", fmt.Errorf("write connectors: %w", err)
	}

	return fmt.Sprintf("%sd platform %s in config", action, conn.ID), nil
}

// syncModelToDB mirrors model_loader.go syncToDatabase logic
func (a *ResearchActionApplier) syncModelToDB(ctx context.Context, profile ModelProfile) error {
	rateLimitsJSON, _ := json.Marshal(profile.RateLimits)
	recoveryJSON, _ := json.Marshal(profile.Recovery)
	apiPricingJSON, _ := json.Marshal(profile.APIPricing)
	capabilitiesJSON, _ := json.Marshal(profile.Capabilities)
	strengthsJSON, _ := json.Marshal(profile.Strengths)
	weaknessesJSON, _ := json.Marshal(profile.Weaknesses)

	configMap := map[string]interface{}{
		"recovery":          json.RawMessage(recoveryJSON),
		"api_pricing":       json.RawMessage(apiPricingJSON),
		"capabilities":      json.RawMessage(capabilitiesJSON),
		"throttle_behavior": string(profile.ThrottleBehavior),
		"buffer_pct":        profile.BufferPct,
		"strengths":         json.RawMessage(strengthsJSON),
		"weaknesses":        json.RawMessage(weaknessesJSON),
		"notes":             profile.Notes,
	}

	platform := ""
	courier := ""
	if len(profile.AccessVia) > 0 {
		platform = profile.AccessVia[0]
		courier = profile.AccessVia[0]
	}
	if platform == "" {
		platform = profile.Provider
	}
	if courier == "" {
		courier = "api"
	}

	upsertData := map[string]interface{}{
		"id":            profile.ID,
		"name":          profile.Name,
		"vendor":        profile.Provider,
		"platform":      platform,
		"access_type":   profile.AccessType,
		"context_limit": profile.ContextLimit,
		"status":        profile.Status,
		"rate_limits":   json.RawMessage(rateLimitsJSON),
		"config":        configMap,
		"strengths":     profile.Strengths,
		"weaknesses":    profile.Weaknesses,
		"courier":       courier,
		"rate_limit_requests_per_minute": profile.RateLimits.RequestsPerMinute,
	}
	if profile.StatusReason != "" {
		upsertData["status_reason"] = profile.StatusReason
	}
	if profile.APIPricing.InputPer1MUsd > 0 {
		upsertData["cost_input_per_1k_usd"] = profile.APIPricing.InputPer1MUsd / 1000.0
	}
	if profile.APIPricing.OutputPer1MUsd > 0 {
		upsertData["cost_output_per_1k_usd"] = profile.APIPricing.OutputPer1MUsd / 1000.0
	}

	// Try insert first, then update
	_, err := a.db.Insert(ctx, "models", upsertData)
	if err != nil {
		// Already exists — update
		delete(upsertData, "id") // don't update PK
		_, updateErr := a.db.Update(ctx, "models", profile.ID, upsertData)
		if updateErr != nil {
			return fmt.Errorf("upsert model %s: insert=%w update=%v", profile.ID, err, updateErr)
		}
		log.Printf("[ResearchAction] Updated model %s in Supabase", profile.ID)
	} else {
		log.Printf("[ResearchAction] Created model %s in Supabase", profile.ID)
	}

	return nil
}

// Config file helpers

func (a *ResearchActionApplier) readModelsConfig(path string) (*ModelsConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file ModelsConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (a *ResearchActionApplier) writeModelsConfig(path string, file *ModelsConfigFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

type connectorsFile struct {
	Connectors []ConnectorConfig `json:"destinations"`
}

func (a *ResearchActionApplier) readConnectorsConfig(path string) (*connectorsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file connectorsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (a *ResearchActionApplier) writeConnectorsConfig(path string, file *connectorsFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// modelDataToProfile converts a loose map from research details into a ModelProfile.
// Missing fields get sensible defaults.
func modelDataToProfile(data map[string]interface{}) ModelProfile {
	profile := ModelProfile{
		Status:  "active",
		BufferPct: 80,
		ThrottleBehavior: "slow_down",
		Recovery: RecoveryConfig{
			OnRateLimit:           "cooldown",
			CooldownMinutes:      5,
			TimeoutSeconds:       300,
			HeartbeatIntervalSecs: 30,
			OrphanThresholdSecs:  300,
		},
		Learned: LearnedData{
			FailureRateByType:   map[string]float64{},
			BestForTaskTypes:    []string{},
			AvoidForTaskTypes:   []string{},
		},
	}

	if v, ok := data["id"].(string); ok {
		profile.ID = v
	}
	if v, ok := data["name"].(string); ok {
		profile.Name = v
	}
	if v, ok := data["provider"].(string); ok {
		profile.Provider = v
	}
	if v, ok := data["vendor"].(string); ok {
		profile.Provider = v
	}
	if v, ok := data["access_type"].(string); ok {
		profile.AccessType = v
	}
	if v, ok := data["status"].(string); ok {
		profile.Status = v
	}
	if v, ok := data["context_limit"].(float64); ok {
		profile.ContextLimit = int(v)
	}
	if v, ok := data["notes"].(string); ok {
		profile.Notes = v
	}
	if v, ok := data["status_reason"].(string); ok {
		profile.StatusReason = v
	}
	if v, ok := data["api_key_ref"].(string); ok {
		profile.APIKeyRef = v
	}
	if v, ok := data["throttle_behavior"].(string); ok {
		profile.ThrottleBehavior = ThrottleBehavior(v)
	}

	// access_via can be string or []string
	switch av := data["access_via"].(type) {
	case string:
		profile.AccessVia = []string{av}
	case []string:
		profile.AccessVia = av
	case []interface{}:
		for _, item := range av {
			if s, ok := item.(string); ok {
				profile.AccessVia = append(profile.AccessVia, s)
			}
		}
	}

	// strengths
	if v, ok := data["strengths"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				profile.Strengths = append(profile.Strengths, s)
			}
		}
	}

	// weaknesses
	if v, ok := data["weaknesses"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				profile.Weaknesses = append(profile.Weaknesses, s)
			}
		}
	}

	// capabilities
	if v, ok := data["capabilities"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				profile.Capabilities = append(profile.Capabilities, s)
			}
		}
	}

	// rate_limits
	if rl, ok := data["rate_limits"].(map[string]interface{}); ok {
		if v, ok := rl["requests_per_minute"].(float64); ok {
			iv := int(v)
			profile.RateLimits.RequestsPerMinute = &iv
		}
		if v, ok := rl["requests_per_hour"].(float64); ok {
			iv := int(v)
			profile.RateLimits.RequestsPerHour = &iv
		}
		if v, ok := rl["requests_per_day"].(float64); ok {
			iv := int(v)
			profile.RateLimits.RequestsPerDay = &iv
		}
		if v, ok := rl["tokens_per_day"].(float64); ok {
			iv := int(v)
			profile.RateLimits.TokensPerDay = &iv
		}
		if v, ok := rl["tokens_per_minute"].(float64); ok {
			iv := int(v)
			profile.RateLimits.TokensPerMinute = &iv
		}
	}

	// api_pricing
	if ap, ok := data["api_pricing"].(map[string]interface{}); ok {
		if v, ok := ap["input_per_1m_usd"].(float64); ok {
			profile.APIPricing.InputPer1MUsd = v
		}
		if v, ok := ap["output_per_1m_usd"].(float64); ok {
			profile.APIPricing.OutputPer1MUsd = v
		}
	}

	return profile
}

func mapDataToConnector(data map[string]interface{}) ConnectorConfig {
	conn := ConnectorConfig{}
	if v, ok := data["id"].(string); ok {
		conn.ID = v
	}
	if v, ok := data["name"].(string); ok {
		conn.Name = v
	}
	if v, ok := data["type"].(string); ok {
		conn.Type = v
	}
	if v, ok := data["url"].(string); ok {
		conn.Endpoint = v
	}
	if v, ok := data["provider"].(string); ok {
		conn.Provider = v
	}
	if v, ok := data["status"].(string); ok {
		conn.Status = v
	}
	return conn
}
