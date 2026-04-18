package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type ThrottleBehavior string

const (
	ThrottleHardCutoff ThrottleBehavior = "hard_cutoff"
	ThrottleSlowDown   ThrottleBehavior = "slow_down"
	ThrottleQueue      ThrottleBehavior = "queue"
	ThrottleRetryAfter ThrottleBehavior = "retry_after"
)

type RateLimits struct {
	RequestsPerMinute  *int `json:"requests_per_minute"`
	RequestsPerHour    *int `json:"requests_per_hour"`
	RequestsPerDay     *int `json:"requests_per_day"`
	TokensPerDay       *int `json:"tokens_per_day"`
	MessagesPer3Hours  *int `json:"messages_per_3_hours"`
	MessagesPerSession *int `json:"messages_per_session"`
}

type RecoveryConfig struct {
	OnRateLimit           string `json:"on_rate_limit"`
	CooldownMinutes       int    `json:"cooldown_minutes"`
	TimeoutSeconds        int    `json:"timeout_seconds"`
	HeartbeatIntervalSecs int    `json:"heartbeat_interval_seconds"`
	OrphanThresholdSecs   int    `json:"orphan_threshold_seconds"`
	MaxTaskAttempts       int    `json:"max_task_attempts"`
	ModelFailureThreshold int    `json:"model_failure_threshold"`
}

type LearnedData struct {
	AvgTaskDurationSeconds float64            `json:"avg_task_duration_seconds"`
	FailureRateByType      map[string]float64 `json:"failure_rate_by_type"`
	OptimalCooldownMinutes float64            `json:"optimal_cooldown_minutes"`
	BestForTaskTypes       []string           `json:"best_for_task_types"`
	AvoidForTaskTypes      []string           `json:"avoid_for_task_types"`
}

type APIPricing struct {
	InputPer1MUsd       float64 `json:"input_per_1m_usd"`
	OutputPer1MUsd      float64 `json:"output_per_1m_usd"`
	InputPer1MCachedUsd float64 `json:"input_per_1m_cached_usd"`
}

type ModelProfile struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Provider         string           `json:"provider"`
	AccessType       string           `json:"access_type"`
	ContextLimit     int              `json:"context_limit"`
	Capabilities     []string         `json:"capabilities"`
	AccessVia        []string         `json:"access_via"`
	APIKeyRef        string           `json:"api_key_ref"`
	Status           string           `json:"status"`
	RateLimits       RateLimits       `json:"rate_limits"`
	ThrottleBehavior ThrottleBehavior `json:"throttle_behavior"`
	BufferPct        int              `json:"buffer_pct"`
	SpacingMinSecs   int              `json:"spacing_min_seconds"`
	APIPricing       APIPricing       `json:"api_pricing"`
	Recovery         RecoveryConfig   `json:"recovery"`
	Learned          LearnedData      `json:"learned"`
	Strengths        []string         `json:"strengths"`
	Weaknesses       []string         `json:"weaknesses"`
	Notes            string           `json:"notes"`
}

type UsageWindow struct {
	Requests    int       `json:"requests"`
	Tokens      int       `json:"tokens"`
	WindowStart time.Time `json:"window_start"`
	ResetAt     time.Time `json:"reset_at"`
}

type UsageWindows struct {
	Minute UsageWindow `json:"minute"`
	Hour   UsageWindow `json:"hour"`
	Day    UsageWindow `json:"day"`
	Week   UsageWindow `json:"week"`
}

type ModelUsage struct {
	Profile           ModelProfile `json:"-"`
	UsageWindows      UsageWindows `json:"usage_windows"`
	LastRequestAt     time.Time    `json:"last_request_at"`
	LastRateLimitAt   *time.Time   `json:"last_rate_limit_at"`
	RateLimitCount    int          `json:"rate_limit_count"`
	CooldownExpiresAt *time.Time   `json:"cooldown_expires_at"`
	Learned           LearnedData  `json:"learned"`
}

type UsageTracker struct {
	mu       sync.RWMutex
	models   map[string]*ModelUsage
	defaults ModelProfile
	db       Querier
}

func NewUsageTracker(db Querier) *UsageTracker {
	return &UsageTracker{
		models: make(map[string]*ModelUsage),
		db:     db,
		defaults: ModelProfile{
			ThrottleBehavior: ThrottleSlowDown,
			BufferPct:        80,
			SpacingMinSecs:   1,
			Recovery: RecoveryConfig{
				OnRateLimit:           "cooldown",
				CooldownMinutes:       30,
				TimeoutSeconds:        300,
				HeartbeatIntervalSecs: 30,
				OrphanThresholdSecs:   300,
				MaxTaskAttempts:       3,
				ModelFailureThreshold: 3,
			},
		},
	}
}

func (t *UsageTracker) SetDefaults(defaults ModelProfile) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.defaults = defaults
}

func (t *UsageTracker) RegisterModel(profile ModelProfile) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.models[profile.ID] = &ModelUsage{
		Profile: profile,
		UsageWindows: UsageWindows{
			Minute: UsageWindow{WindowStart: now, ResetAt: now.Add(time.Minute)},
			Hour:   UsageWindow{WindowStart: now, ResetAt: now.Add(time.Hour)},
			Day:    UsageWindow{WindowStart: now, ResetAt: now.Add(24 * time.Hour)},
			Week:   UsageWindow{WindowStart: now, ResetAt: now.Add(7 * 24 * time.Hour)},
		},
	}
}

type RequestDecision struct {
	CanProceed bool          `json:"can_proceed"`
	WaitTime   time.Duration `json:"wait_time"`
	Reason     string        `json:"reason"`
	WindowType string        `json:"window_type"`
}

func (t *UsageTracker) CanMakeRequest(ctx context.Context, modelID string, estimatedTokens int) RequestDecision {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, exists := t.models[modelID]
	if !exists {
		return RequestDecision{CanProceed: false, Reason: "model not registered"}
	}

	now := time.Now()
	profile := usage.Profile

	if profile.BufferPct == 0 {
		profile.BufferPct = t.defaults.BufferPct
	}
	buffer := float64(profile.BufferPct) / 100.0

	if usage.CooldownExpiresAt != nil && usage.CooldownExpiresAt.After(now) {
		return RequestDecision{
			CanProceed: false,
			WaitTime:   usage.CooldownExpiresAt.Sub(now),
			Reason:     "cooldown_active",
		}
	}

	t.resetExpiredWindows(usage, now)

	limits := profile.RateLimits

	if limits.RequestsPerMinute != nil {
		allowed := int(float64(*limits.RequestsPerMinute) * buffer)
		if usage.UsageWindows.Minute.Requests >= allowed {
			return RequestDecision{
				CanProceed: false,
				WaitTime:   usage.UsageWindows.Minute.ResetAt.Sub(now),
				Reason:     "minute_limit_reached",
				WindowType: "minute",
			}
		}
	}

	if limits.RequestsPerHour != nil {
		allowed := int(float64(*limits.RequestsPerHour) * buffer)
		if usage.UsageWindows.Hour.Requests >= allowed {
			return RequestDecision{
				CanProceed: false,
				WaitTime:   usage.UsageWindows.Hour.ResetAt.Sub(now),
				Reason:     "hour_limit_reached",
				WindowType: "hour",
			}
		}
	}

	if limits.RequestsPerDay != nil {
		allowed := int(float64(*limits.RequestsPerDay) * buffer)
		if usage.UsageWindows.Day.Requests >= allowed {
			return RequestDecision{
				CanProceed: false,
				WaitTime:   usage.UsageWindows.Day.ResetAt.Sub(now),
				Reason:     "day_limit_reached",
				WindowType: "day",
			}
		}
	}

	if limits.TokensPerDay != nil {
		allowed := int(float64(*limits.TokensPerDay) * buffer)
		if usage.UsageWindows.Day.Tokens+estimatedTokens > allowed {
			return RequestDecision{
				CanProceed: false,
				WaitTime:   usage.UsageWindows.Day.ResetAt.Sub(now),
				Reason:     "token_limit_reached",
				WindowType: "day",
			}
		}
	}

	spacingSecs := profile.SpacingMinSecs
	if spacingSecs == 0 {
		spacingSecs = t.defaults.SpacingMinSecs
	}
	if spacingSecs > 0 && !usage.LastRequestAt.IsZero() {
		nextAllowed := usage.LastRequestAt.Add(time.Duration(spacingSecs) * time.Second)
		if nextAllowed.After(now) {
			return RequestDecision{
				CanProceed: false,
				WaitTime:   nextAllowed.Sub(now),
				Reason:     "spacing_not_met",
			}
		}
	}

	return RequestDecision{CanProceed: true}
}

func (t *UsageTracker) RecordUsage(ctx context.Context, modelID string, tokensIn, tokensOut int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, exists := t.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	now := time.Now()
	totalTokens := tokensIn + tokensOut

	t.resetExpiredWindows(usage, now)

	usage.UsageWindows.Minute.Requests++
	usage.UsageWindows.Minute.Tokens += totalTokens

	usage.UsageWindows.Hour.Requests++
	usage.UsageWindows.Hour.Tokens += totalTokens

	usage.UsageWindows.Day.Requests++
	usage.UsageWindows.Day.Tokens += totalTokens

	usage.UsageWindows.Week.Requests++
	usage.UsageWindows.Week.Tokens += totalTokens

	usage.LastRequestAt = now

	return nil
}

func (t *UsageTracker) RecordRateLimit(ctx context.Context, modelID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, exists := t.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	now := time.Now()
	usage.LastRateLimitAt = &now
	usage.RateLimitCount++

	cooldownMins := usage.Profile.Recovery.CooldownMinutes
	if cooldownMins == 0 {
		cooldownMins = t.defaults.Recovery.CooldownMinutes
	}

	if usage.Learned.OptimalCooldownMinutes > 0 {
		cooldownMins = int(usage.Learned.OptimalCooldownMinutes)
	}

	cooldownExpiry := now.Add(time.Duration(cooldownMins) * time.Minute)
	usage.CooldownExpiresAt = &cooldownExpiry

	return nil
}

func (t *UsageTracker) RecordCompletion(ctx context.Context, modelID string, taskType string, durationSeconds float64, success bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, exists := t.models[modelID]
	if !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	if usage.Learned.AvgTaskDurationSeconds == 0 {
		usage.Learned.AvgTaskDurationSeconds = durationSeconds
	} else {
		usage.Learned.AvgTaskDurationSeconds = (usage.Learned.AvgTaskDurationSeconds * 0.9) + (durationSeconds * 0.1)
	}

	if success {
		found := false
		for _, t := range usage.Learned.BestForTaskTypes {
			if t == taskType {
				found = true
				break
			}
		}
		if !found {
			usage.Learned.BestForTaskTypes = append(usage.Learned.BestForTaskTypes, taskType)
		}
	} else {
		if usage.Learned.FailureRateByType == nil {
			usage.Learned.FailureRateByType = make(map[string]float64)
		}
		usage.Learned.FailureRateByType[taskType] += 1

		if usage.CooldownExpiresAt != nil {
			learnedCooldown := float64(usage.Profile.Recovery.CooldownMinutes) * 1.5
			usage.Learned.OptimalCooldownMinutes = learnedCooldown
		}
	}

	if usage.CooldownExpiresAt != nil && usage.CooldownExpiresAt.Before(time.Now()) {
		usage.CooldownExpiresAt = nil
	}

	return nil
}

func (t *UsageTracker) resetExpiredWindows(usage *ModelUsage, now time.Time) {
	if usage.UsageWindows.Minute.ResetAt.Before(now) {
		usage.UsageWindows.Minute = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(time.Minute),
		}
	}
	if usage.UsageWindows.Hour.ResetAt.Before(now) {
		usage.UsageWindows.Hour = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(time.Hour),
		}
	}
	if usage.UsageWindows.Day.ResetAt.Before(now) {
		usage.UsageWindows.Day = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(24 * time.Hour),
		}
	}
	if usage.UsageWindows.Week.ResetAt.Before(now) {
		usage.UsageWindows.Week = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(7 * 24 * time.Hour),
		}
	}
}

func (t *UsageTracker) GetModelStatus(modelID string) (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	usage, exists := t.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not registered", modelID)
	}

	status := map[string]interface{}{
		"id":                usage.Profile.ID,
		"name":              usage.Profile.Name,
		"status":            usage.Profile.Status,
		"throttle_behavior": usage.Profile.ThrottleBehavior,
		"last_request_at":   usage.LastRequestAt,
		"rate_limit_count":  usage.RateLimitCount,
		"usage_windows":     usage.UsageWindows,
		"learned":           usage.Learned,
	}

	if usage.CooldownExpiresAt != nil {
		now := time.Now()
		if usage.CooldownExpiresAt.After(now) {
			status["cooldown_active"] = true
			status["cooldown_expires_at"] = usage.CooldownExpiresAt
			status["cooldown_remaining_seconds"] = usage.CooldownExpiresAt.Sub(now).Seconds()
		} else {
			status["cooldown_active"] = false
		}
	}

	return status, nil
}

func (t *UsageTracker) GetCooldownCountdown(modelID string) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	usage, exists := t.models[modelID]
	if !exists {
		return 0, fmt.Errorf("model %s not registered", modelID)
	}

	if usage.CooldownExpiresAt == nil {
		return 0, nil
	}

	now := time.Now()
	if usage.CooldownExpiresAt.Before(now) {
		return 0, nil
	}

	return int(usage.CooldownExpiresAt.Sub(now).Seconds()), nil
}

func (t *UsageTracker) ExportForDashboard() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	models := make([]map[string]interface{}, 0, len(t.models))
	for id := range t.models {
		status, _ := t.GetModelStatus(id)
		if status != nil {
			models = append(models, status)
		}
	}

	return json.Marshal(models)
}

func (t *UsageTracker) PersistToDatabase(ctx context.Context) {
	t.mu.RLock()
	data := make(map[string]*ModelUsage)
	for id, usage := range t.models {
		data[id] = usage
	}
	t.mu.RUnlock()

	for modelID, usage := range data {
		windowsJSON, _ := json.Marshal(usage.UsageWindows)
		learnedJSON, _ := json.Marshal(usage.Learned)

		_, err := t.db.RPC(ctx, "update_model_usage", map[string]any{
			"p_model_id":            modelID,
			"p_usage_windows":       string(windowsJSON),
			"p_cooldown_expires_at": usage.CooldownExpiresAt,
			"p_last_rate_limit_at":  usage.LastRateLimitAt,
			"p_rate_limit_count":    usage.RateLimitCount,
			"p_learned":             string(learnedJSON),
		})
		if err != nil {
			log.Printf("[UsageTracker] Failed to persist model %s: %v", modelID, err)
		}
	}
}
