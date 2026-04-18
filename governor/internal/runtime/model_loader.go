package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

type ModelsConfigFile struct {
	Schema      string         `json:"$schema"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Models      []ModelProfile `json:"models"`
	Defaults    ModelProfile   `json:"defaults"`
}

type ModelLoader struct {
	configPath string
	db         *db.DB
	tracker    *UsageTracker
}

func NewModelLoader(configPath string, database *db.DB, tracker *UsageTracker) *ModelLoader {
	return &ModelLoader{
		configPath: configPath,
		db:         database,
		tracker:    tracker,
	}
}

func (l *ModelLoader) Load(ctx context.Context) error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("read models config: %w", err)
	}

	var file ModelsConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse models config: %w", err)
	}

	if l.tracker != nil && file.Defaults.BufferPct > 0 {
		l.tracker.SetDefaults(file.Defaults)
	}

	for _, profile := range file.Models {
		if l.tracker != nil {
			l.tracker.RegisterModel(profile)
		}

		if err := l.syncToDatabase(ctx, profile); err != nil {
			return fmt.Errorf("sync model %s to database: %w", profile.ID, err)
		}
	}

	return nil
}

func (l *ModelLoader) syncToDatabase(ctx context.Context, profile ModelProfile) error {
	rateLimitsJSON, _ := json.Marshal(profile.RateLimits)
	recoveryJSON, _ := json.Marshal(profile.Recovery)
	apiPricingJSON, _ := json.Marshal(profile.APIPricing)
	capabilitiesJSON, _ := json.Marshal(profile.Capabilities)
	strengthsJSON, _ := json.Marshal(profile.Strengths)
	weaknessesJSON, _ := json.Marshal(profile.Weaknesses)

	configMap := map[string]interface{}{
		"recovery":            json.RawMessage(recoveryJSON),
		"api_pricing":         json.RawMessage(apiPricingJSON),
		"capabilities":        json.RawMessage(capabilitiesJSON),
		"throttle_behavior":   string(profile.ThrottleBehavior),
		"buffer_pct":          profile.BufferPct,
		"spacing_min_seconds": profile.SpacingMinSecs,
		"strengths":           json.RawMessage(strengthsJSON),
		"weaknesses":          json.RawMessage(weaknessesJSON),
		"notes":               profile.Notes,
	}

	// Derive platform from access_via (connector name) or fall back to provider
	platform := profile.Provider
	if len(profile.AccessVia) > 0 {
		platform = profile.AccessVia[0]
	}

	// Derive courier from access_via
	courier := "api"
	if len(profile.AccessVia) > 0 {
		courier = profile.AccessVia[0]
	}
	if profile.AccessType == "cli_subscription" {
		courier = platform
	}

	// Build insert data with all required columns
	insertData := map[string]interface{}{
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
	if profile.APIPricing.InputPer1MUsd > 0 {
		insertData["cost_input_per_1k_usd"] = profile.APIPricing.InputPer1MUsd / 1000.0
	}
	if profile.APIPricing.OutputPer1MUsd > 0 {
		insertData["cost_output_per_1k_usd"] = profile.APIPricing.OutputPer1MUsd / 1000.0
	}

	_, err := l.db.Insert(ctx, "models", insertData)
	if err != nil {
		// Already exists -- update instead
		updateData := map[string]interface{}{
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
		if profile.APIPricing.InputPer1MUsd > 0 {
			updateData["cost_input_per_1k_usd"] = profile.APIPricing.InputPer1MUsd / 1000.0
		}
		if profile.APIPricing.OutputPer1MUsd > 0 {
			updateData["cost_output_per_1k_usd"] = profile.APIPricing.OutputPer1MUsd / 1000.0
		}

		_, updateErr := l.db.Update(ctx, "models", profile.ID, updateData)
		if updateErr != nil {
			return fmt.Errorf("upsert model %s: insert=%w update=%v", profile.ID, err, updateErr)
		}
	} else {
		log.Printf("[ModelLoader] Created new model %s in Supabase", profile.ID)
	}

	return nil
}

func (l *ModelLoader) Reload(ctx context.Context) error {
	return l.Load(ctx)
}

func LoadModelsFromConfig(configDir string, database *db.DB, tracker *UsageTracker) (*ModelLoader, error) {
	configPath := filepath.Join(configDir, "models.json")

	loader := NewModelLoader(configPath, database, tracker)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := loader.Load(ctx); err != nil {
		return nil, err
	}

	return loader, nil
}

func (l *ModelLoader) GetModel(modelID string) *ModelProfile {
	if l.tracker == nil {
		return nil
	}

	l.tracker.mu.RLock()
	defer l.tracker.mu.RUnlock()

	if usage, exists := l.tracker.models[modelID]; exists {
		profile := usage.Profile
		profile.Learned = usage.Learned
		return &profile
	}

	return nil
}

func (l *ModelLoader) ListModels() []string {
	if l.tracker == nil {
		return nil
	}

	l.tracker.mu.RLock()
	defer l.tracker.mu.RUnlock()

	ids := make([]string, 0, len(l.tracker.models))
	for id := range l.tracker.models {
		ids = append(ids, id)
	}
	return ids
}

func (l *ModelLoader) GetActiveModels() []string {
	if l.tracker == nil {
		return nil
	}

	l.tracker.mu.RLock()
	defer l.tracker.mu.RUnlock()

	ids := make([]string, 0)
	for id, usage := range l.tracker.models {
		if usage.Profile.Status == "active" {
			ids = append(ids, id)
		}
	}
	return ids
}

func (l *ModelLoader) GetAvailableModels(ctx context.Context) []string {
	if l.tracker == nil {
		return nil
	}

	l.tracker.mu.RLock()
	defer l.tracker.mu.RUnlock()

	now := time.Now()
	ids := make([]string, 0)

	for id, usage := range l.tracker.models {
		if usage.Profile.Status != "active" {
			continue
		}

		if usage.CooldownExpiresAt != nil && usage.CooldownExpiresAt.After(now) {
			continue
		}

		ids = append(ids, id)
	}

	return ids
}

func (l *ModelLoader) UpdateLearnedData(ctx context.Context, modelID string, learned LearnedData) error {
	if l.tracker != nil {
		l.tracker.mu.Lock()
		if usage, exists := l.tracker.models[modelID]; exists {
			usage.Learned = learned
		}
		l.tracker.mu.Unlock()
	}

	learnedJSON, _ := json.Marshal(learned)

	_, err := l.db.Update(ctx, "models", modelID, map[string]interface{}{
		"learned": json.RawMessage(learnedJSON),
	})

	return err
}
