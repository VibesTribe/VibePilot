package runtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// ConnectorProfile tracks aggregated usage across all models sharing a connector.
// This enables proactive rate limit tracking for connector-level shared limits
// (e.g., Groq org-level 100K TPD shared across all 7 Groq models).
type ConnectorProfile struct {
	ConnectorID  string       `json:"connector_id"`
	SharedLimits RateLimits   `json:"shared_limits"`
	UsageWindows UsageWindows `json:"usage_windows"`
}

// ConnectorUsageTracker aggregates usage across all models sharing a connector.
// It is embedded in UsageTracker (composition) and provides proactive tracking
// to prevent hitting 429 rate limits, complementing the reactive RecordConnectorCooldown.
type ConnectorUsageTracker struct {
	mu       sync.Mutex
	profiles map[string]*ConnectorProfile
	bufferPct int // percentage of limit to use as threshold (e.g., 80)
}

// NewConnectorUsageTracker creates a new ConnectorUsageTracker.
func NewConnectorUsageTracker(bufferPct int) *ConnectorUsageTracker {
	if bufferPct <= 0 || bufferPct > 100 {
		bufferPct = 80
	}
	return &ConnectorUsageTracker{
		profiles:  make(map[string]*ConnectorProfile),
		bufferPct: bufferPct,
	}
}

// RegisterConnector registers a connector with its shared rate limits.
// Called during model loading when connectors are parsed.
// If a connector is already registered, it updates the shared limits only
// (preserving existing usage data).
func (ct *ConnectorUsageTracker) RegisterConnector(connectorID string, sharedLimits RateLimits) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	now := time.Now()

	if profile, exists := ct.profiles[connectorID]; exists {
		// Update limits but preserve usage windows
		profile.SharedLimits = sharedLimits
		return
	}

	ct.profiles[connectorID] = &ConnectorProfile{
		ConnectorID:  connectorID,
		SharedLimits: sharedLimits,
		UsageWindows: UsageWindows{
			Minute: UsageWindow{WindowStart: now, ResetAt: now.Add(time.Minute)},
			Hour:   UsageWindow{WindowStart: now, ResetAt: now.Add(time.Hour)},
			Day:    UsageWindow{WindowStart: now, ResetAt: now.Add(24 * time.Hour)},
			Week:   UsageWindow{WindowStart: now, ResetAt: now.Add(7 * 24 * time.Hour)},
		},
	}

	log.Printf("[ConnectorTracker] Registered connector %s with shared limits: RPM=%v RPD=%v TPM=%v TPD=%v",
		connectorID,
		sharedLimits.RequestsPerMinute,
		sharedLimits.RequestsPerDay,
		sharedLimits.TokensPerMinute,
		sharedLimits.TokensPerDay,
	)
}

// RecordUsage records token usage for a connector, aggregated across all models.
// This should be called AFTER recording model-level usage, for each connector
// the model uses (from its access_via list).
func (ct *ConnectorUsageTracker) RecordUsage(ctx context.Context, connectorID string, tokensIn, tokensOut int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	profile, exists := ct.profiles[connectorID]
	if !exists {
		return // no shared limits configured for this connector
	}

	now := time.Now()
	totalTokens := tokensIn + tokensOut

	ct.resetExpiredWindows(profile, now)

	profile.UsageWindows.Minute.Requests++
	profile.UsageWindows.Minute.Tokens += totalTokens

	profile.UsageWindows.Hour.Requests++
	profile.UsageWindows.Hour.Tokens += totalTokens

	profile.UsageWindows.Day.Requests++
	profile.UsageWindows.Day.Tokens += totalTokens

	profile.UsageWindows.Week.Requests++
	profile.UsageWindows.Week.Tokens += totalTokens
}

// CanMakeRequest checks whether the connector has capacity for an additional request
// with the given estimated tokens, based on shared connector-level limits.
// Returns whether the request can proceed, and if not, how long to wait.
func (ct *ConnectorUsageTracker) CanMakeRequest(ctx context.Context, connectorID string, estimatedTokens int) (canProceed bool, waitTime time.Duration) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	profile, exists := ct.profiles[connectorID]
	if !exists {
		return true, 0 // no shared limits = always allowed
	}

	now := time.Now()
	buffer := float64(ct.bufferPct) / 100.0

	ct.resetExpiredWindows(profile, now)

	limits := profile.SharedLimits

	if limits.RequestsPerMinute != nil {
		allowed := int(float64(*limits.RequestsPerMinute) * buffer)
		if profile.UsageWindows.Minute.Requests >= allowed {
			return false, profile.UsageWindows.Minute.ResetAt.Sub(now)
		}
	}

	if limits.RequestsPerDay != nil {
		allowed := int(float64(*limits.RequestsPerDay) * buffer)
		if profile.UsageWindows.Day.Requests >= allowed {
			return false, profile.UsageWindows.Day.ResetAt.Sub(now)
		}
	}

	if limits.TokensPerDay != nil && estimatedTokens > 0 {
		allowed := int(float64(*limits.TokensPerDay) * buffer)
		if profile.UsageWindows.Day.Tokens+estimatedTokens > allowed {
			return false, profile.UsageWindows.Day.ResetAt.Sub(now)
		}
	}

	if limits.TokensPerMinute != nil && estimatedTokens > 0 {
		allowed := int(float64(*limits.TokensPerMinute) * buffer)
		if profile.UsageWindows.Minute.Tokens+estimatedTokens > allowed {
			return false, profile.UsageWindows.Minute.ResetAt.Sub(now)
		}
	}

	return true, 0
}

// HasConnector returns true if a connector is registered with shared limits.
func (ct *ConnectorUsageTracker) HasConnector(connectorID string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	_, exists := ct.profiles[connectorID]
	return exists
}

// GetConnectorStatus returns usage status for a connector (for dashboard/debugging).
func (ct *ConnectorUsageTracker) GetConnectorStatus(connectorID string) map[string]interface{} {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	profile, exists := ct.profiles[connectorID]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"connector_id":   profile.ConnectorID,
		"shared_limits":  profile.SharedLimits,
		"usage_windows":  profile.UsageWindows,
	}
}

// resetExpiredWindows resets usage windows whose reset time has passed.
func (ct *ConnectorUsageTracker) resetExpiredWindows(profile *ConnectorProfile, now time.Time) {
	if profile.UsageWindows.Minute.ResetAt.Before(now) {
		profile.UsageWindows.Minute = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(time.Minute),
		}
	}
	if profile.UsageWindows.Hour.ResetAt.Before(now) {
		profile.UsageWindows.Hour = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(time.Hour),
		}
	}
	if profile.UsageWindows.Day.ResetAt.Before(now) {
		profile.UsageWindows.Day = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(24 * time.Hour),
		}
	}
	if profile.UsageWindows.Week.ResetAt.Before(now) {
		profile.UsageWindows.Week = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(7 * 24 * time.Hour),
		}
	}
}

// PersistToDatabase persists all connector usage windows to the connector_usage table.
// Errors are logged but do not crash the governor.
func (ct *ConnectorUsageTracker) PersistToDatabase(ctx context.Context, db Querier) {
	ct.mu.Lock()
	snapshot := make(map[string]*ConnectorProfile, len(ct.profiles))
	for id, p := range ct.profiles {
		snapshot[id] = p
	}
	ct.mu.Unlock()

	for connectorID, profile := range snapshot {
		windowsJSON, err := json.Marshal(profile.UsageWindows)
		if err != nil {
			log.Printf("[ConnectorTracker] Failed to marshal usage_windows for %s: %v", connectorID, err)
			continue
		}

		_, err = db.RPC(ctx, "upsert_connector_usage", map[string]interface{}{
			"p_connector_id": connectorID,
			"p_usage_windows": string(windowsJSON),
		})
		if err != nil {
			log.Printf("[ConnectorTracker] Failed to persist connector %s: %v", connectorID, err)
		}
	}
}

// LoadFromDatabase restores connector usage windows from the connector_usage table.
// It only updates profiles that are already registered; unregistered connectors are skipped.
// Errors are logged but do not crash the governor.
func (ct *ConnectorUsageTracker) LoadFromDatabase(ctx context.Context, db Querier) {
	data, err := db.Query(ctx, "connector_usage", map[string]any{
		"limit": 200,
	})
	if err != nil {
		log.Printf("[ConnectorTracker] Failed to query connector_usage: %v", err)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		log.Printf("[ConnectorTracker] Failed to unmarshal connector_usage: %v", err)
		return
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	restored := 0
	for _, row := range rows {
		connectorID, _ := row["connector_id"].((string))
		if connectorID == "" {
			continue
		}

		profile, exists := ct.profiles[connectorID]
		if !exists {
			continue // connector not registered, skip
		}

		if raw, ok := row["usage_windows"]; ok && raw != nil {
			var windows UsageWindows
			if str, ok := raw.(string); ok && str != "" {
				if err := json.Unmarshal([]byte(str), &windows); err != nil {
					log.Printf("[ConnectorTracker] Warning: failed to parse usage_windows for %s: %v", connectorID, err)
				} else {
					profile.UsageWindows = windows
					restored++
				}
			}
		}
	}

	log.Printf("[ConnectorTracker] Restored persisted state for %d/%d connectors from database", restored, len(ct.profiles))
}
