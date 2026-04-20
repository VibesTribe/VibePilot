package runtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// PlatformProfile tracks usage for a web platform destination (courier)
// against its free-tier limits across multiple time windows.
type PlatformProfile struct {
	PlatformID   string
	Limits       PlatformLimitSchema
	Usage3h      UsageWindow
	Usage8h      UsageWindow
	UsageDay     UsageWindow
	UsageSession UsageWindow
	SessionStart time.Time
}

// PlatformUsageTracker tracks courier destination usage against web platform
// free-tier limits. It is embedded in UsageTracker (composition) and provides
// proactive tracking so courier agents don't hit free-tier walls unprepared.
//
// Thread-safe via mutex. In-memory only for now; persistence comes in Task 6.
type PlatformUsageTracker struct {
	mu        sync.Mutex
	profiles  map[string]*PlatformProfile
	bufferPct int // percentage of limit to use as threshold (e.g., 80)
}

// NewPlatformUsageTracker creates a new PlatformUsageTracker.
func NewPlatformUsageTracker(bufferPct int) *PlatformUsageTracker {
	if bufferPct <= 0 || bufferPct > 100 {
		bufferPct = 80
	}
	return &PlatformUsageTracker{
		profiles:  make(map[string]*PlatformProfile),
		bufferPct: bufferPct,
	}
}

// RegisterPlatform registers a web platform destination with its free-tier limits.
// Called during model loading when web connectors with limit_schema are parsed.
// If the platform is already registered, it updates the limits only
// (preserving existing usage data).
func (pt *PlatformUsageTracker) RegisterPlatform(platformID string, limits PlatformLimitSchema) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()

	if profile, exists := pt.profiles[platformID]; exists {
		// Update limits but preserve usage data
		profile.Limits = limits
		return
	}

	pt.profiles[platformID] = &PlatformProfile{
		PlatformID:   platformID,
		Limits:       limits,
		Usage3h:      UsageWindow{WindowStart: now, ResetAt: now.Add(3 * time.Hour)},
		Usage8h:      UsageWindow{WindowStart: now, ResetAt: now.Add(8 * time.Hour)},
		UsageDay:     UsageWindow{WindowStart: now, ResetAt: now.Add(24 * time.Hour)},
		UsageSession: UsageWindow{WindowStart: now, ResetAt: now.Add(24 * time.Hour)}, // session resets on NewSession
		SessionStart: now,
	}

	log.Printf("[PlatformTracker] Registered platform %s with limits: 3h=%v 8h=%v day=%v session=%v tpd=%v spd=%v",
		platformID,
		limits.MessagesPer3h,
		limits.MessagesPer8h,
		limits.MessagesPerDay,
		limits.MessagesPerSession,
		limits.TokensPerDay,
		limits.SessionsPerDay,
	)
}

// RecordMessageSent records that a message was sent to a web platform.
// tokensUsed is the estimated token count for the message (input + output).
func (pt *PlatformUsageTracker) RecordMessageSent(ctx context.Context, platformID string, tokensUsed int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return // platform not registered
	}

	now := time.Now()
	pt.resetExpiredWindows(profile, now)

	// Increment all applicable windows
	profile.Usage3h.Requests++
	profile.Usage3h.Tokens += tokensUsed

	profile.Usage8h.Requests++
	profile.Usage8h.Tokens += tokensUsed

	profile.UsageDay.Requests++
	profile.UsageDay.Tokens += tokensUsed

	profile.UsageSession.Requests++
	profile.UsageSession.Tokens += tokensUsed
}

// CanMakeRequest checks whether the web platform has capacity for an additional
// request with the given estimated tokens, based on all applicable limit windows.
// Returns whether the request can proceed, and if not, how long to wait.
//
// ALL applicable windows for the platform are checked. E.g., chatgpt-web has
// both a 3h window and a day window -- both must have headroom.
func (pt *PlatformUsageTracker) CanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) (canProceed bool, waitTime time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return true, 0 // no limits registered = always allowed
	}

	now := time.Now()
	buffer := float64(pt.bufferPct) / 100.0

	pt.resetExpiredWindows(profile, now)

	// Check 3-hour message limit
	if profile.Limits.MessagesPer3h != nil {
		allowed := int(float64(*profile.Limits.MessagesPer3h) * buffer)
		if profile.Usage3h.Requests >= allowed {
			return false, profile.Usage3h.ResetAt.Sub(now)
		}
	}

	// Check 8-hour message limit
	if profile.Limits.MessagesPer8h != nil {
		allowed := int(float64(*profile.Limits.MessagesPer8h) * buffer)
		if profile.Usage8h.Requests >= allowed {
			return false, profile.Usage8h.ResetAt.Sub(now)
		}
	}

	// Check daily message limit
	if profile.Limits.MessagesPerDay != nil {
		allowed := int(float64(*profile.Limits.MessagesPerDay) * buffer)
		if profile.UsageDay.Requests >= allowed {
			return false, profile.UsageDay.ResetAt.Sub(now)
		}
	}

	// Check per-session message limit
	if profile.Limits.MessagesPerSession != nil {
		allowed := int(float64(*profile.Limits.MessagesPerSession) * buffer)
		if profile.UsageSession.Requests >= allowed {
			// Session doesn't auto-reset; caller must call NewSession
			// Return a very long wait to signal session exhaustion
			return false, 24 * time.Hour
		}
	}

	// Check daily token limit
	if profile.Limits.TokensPerDay != nil && estimatedTokens > 0 {
		allowed := int(float64(*profile.Limits.TokensPerDay) * buffer)
		if profile.UsageDay.Tokens+estimatedTokens > allowed {
			return false, profile.UsageDay.ResetAt.Sub(now)
		}
	}

	// Check daily session limit (tracked via UsageSession.Requests as session count)
	// Note: sessions_per_day is checked against the day window requests,
	// but session count is tracked separately. We use a simple approach:
	// the day window tracks total messages, session count is in profile metadata.
	// For now, sessions_per_day isn't enforced at the message level since
	// NewSession is the entry point for starting sessions.

	return true, 0
}

// NewSession resets the session counter for a platform, starting a fresh session.
// This should be called when a courier agent starts a new browser session.
func (pt *PlatformUsageTracker) NewSession(platformID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return
	}

	now := time.Now()
	profile.UsageSession = UsageWindow{
		WindowStart: now,
		ResetAt:     now.Add(24 * time.Hour),
	}
	profile.SessionStart = now
}

// GetPlatformStatus returns usage status for a platform (for dashboard/debugging).
func (pt *PlatformUsageTracker) GetPlatformStatus(platformID string) map[string]interface{} {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"platform_id":   profile.PlatformID,
		"limits":        profile.Limits,
		"usage_3h":      profile.Usage3h,
		"usage_8h":      profile.Usage8h,
		"usage_day":     profile.UsageDay,
		"usage_session": profile.UsageSession,
		"session_start": profile.SessionStart,
	}
}

// HasPlatform returns true if a platform is registered with limit tracking.
func (pt *PlatformUsageTracker) HasPlatform(platformID string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	_, exists := pt.profiles[platformID]
	return exists
}

// resetExpiredWindows resets usage windows whose reset time has passed.
func (pt *PlatformUsageTracker) resetExpiredWindows(profile *PlatformProfile, now time.Time) {
	if profile.Usage3h.ResetAt.Before(now) {
		profile.Usage3h = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(3 * time.Hour),
		}
	}
	if profile.Usage8h.ResetAt.Before(now) {
		profile.Usage8h = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(8 * time.Hour),
		}
	}
	if profile.UsageDay.ResetAt.Before(now) {
		profile.UsageDay = UsageWindow{
			WindowStart: now,
			ResetAt:     now.Add(24 * time.Hour),
		}
	}
	// Session window is NOT auto-reset; it resets only via NewSession()
}

// platformWindowsJSON is the serialized form of platform usage state
// stored in the platforms.usage_windows TEXT column.
type platformWindowsJSON struct {
	Usage3h      UsageWindow `json:"usage_3h"`
	Usage8h      UsageWindow `json:"usage_8h"`
	UsageDay     UsageWindow `json:"usage_day"`
	UsageSession UsageWindow `json:"usage_session"`
	SessionStart time.Time   `json:"session_start"`
}

// PersistToDatabase persists all platform usage windows to the platforms table.
// Errors are logged but do not crash the governor.
func (pt *PlatformUsageTracker) PersistToDatabase(ctx context.Context, db Querier) {
	pt.mu.Lock()
	snapshot := make(map[string]*PlatformProfile, len(pt.profiles))
	for id, p := range pt.profiles {
		snapshot[id] = p
	}
	pt.mu.Unlock()

	for platformID, profile := range snapshot {
		state := platformWindowsJSON{
			Usage3h:      profile.Usage3h,
			Usage8h:      profile.Usage8h,
			UsageDay:     profile.UsageDay,
			UsageSession: profile.UsageSession,
			SessionStart: profile.SessionStart,
		}
		windowsJSON, err := json.Marshal(state)
		if err != nil {
			log.Printf("[PlatformTracker] Failed to marshal usage_windows for %s: %v", platformID, err)
			continue
		}

		_, err = db.RPC(ctx, "update_platform_usage", map[string]interface{}{
			"p_platform_id":  platformID,
			"p_usage_windows": string(windowsJSON),
		})
		if err != nil {
			log.Printf("[PlatformTracker] Failed to persist platform %s: %v", platformID, err)
		}
	}
}

// LoadFromDatabase restores platform usage windows from the platforms table.
// It only updates profiles that are already registered; unregistered platforms are skipped.
// Errors are logged but do not crash the governor.
func (pt *PlatformUsageTracker) LoadFromDatabase(ctx context.Context, db Querier) {
	data, err := db.Query(ctx, "platforms", map[string]any{
		"limit": 200,
	})
	if err != nil {
		log.Printf("[PlatformTracker] Failed to query platforms: %v", err)
		return
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		log.Printf("[PlatformTracker] Failed to unmarshal platforms: %v", err)
		return
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	restored := 0
	for _, row := range rows {
		// Platforms may be keyed by id or name; try both
		platformID := ""
		if v, ok := row["id"]; ok {
			if s, ok := v.(string); ok {
				platformID = s
			}
		}
		// Also try matching by name if id doesn't match
		nameStr := ""
		if v, ok := row["name"]; ok {
			if s, ok := v.(string); ok {
				nameStr = s
			}
		}

		profile, exists := pt.profiles[platformID]
		if !exists && nameStr != "" {
			profile, exists = pt.profiles[nameStr]
		}
		if !exists {
			continue // platform not registered, skip
		}

		if raw, ok := row["usage_windows"]; ok && raw != nil {
			var state platformWindowsJSON
			if str, ok := raw.(string); ok && str != "" {
				if err := json.Unmarshal([]byte(str), &state); err != nil {
					log.Printf("[PlatformTracker] Warning: failed to parse usage_windows for %s: %v", platformID, err)
				} else {
					profile.Usage3h = state.Usage3h
					profile.Usage8h = state.Usage8h
					profile.UsageDay = state.UsageDay
					profile.UsageSession = state.UsageSession
					profile.SessionStart = state.SessionStart
					restored++
				}
			}
		}
	}

	log.Printf("[PlatformTracker] Restored persisted state for %d/%d platforms from database", restored, len(pt.profiles))
}
