package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// LimitType defines what kind of limit a platform enforces.
type LimitType string

const (
	LimitTypeRequests       LimitType = "requests"        // N requests per window
	LimitTypeTokens         LimitType = "tokens"          // N tokens per window
	LimitTypeComputePoints  LimitType = "compute_points"  // N compute points per window
	LimitTypeCredit         LimitType = "credit"          // $USD credit balance (no window)
)

// LimitWindow defines the time period for a limit.
type LimitWindow string

const (
	WindowMinute LimitWindow = "minute" // 1 minute rolling
	WindowHour   LimitWindow = "hour"   // 1 hour rolling
	Window3h     LimitWindow = "3h"     // 3 hour rolling
	Window8h     LimitWindow = "8h"     // 8 hour rolling
	WindowDay    LimitWindow = "day"    // 24 hour rolling (calendar day alignment optional)
	WindowCycle  LimitWindow = "cycle"  // custom cycle (GLM: 5h, DeepSeek: monthly)
	WindowNever  LimitWindow = "never"  // credit-based, no time reset
)

// PlatformLimit defines a single limit for a platform.
// Each platform can have multiple limits with different types and windows.
// Examples:
//   - AI Studio: requests/minute=15, tokens/minute=1M, requests/day=1500
//   - ChatGPT: requests/3h=40
//   - NoteGPT: requests/day=3
//   - Poe: compute_points/day=3000
//   - DeepSeek API: credit/never=$2.81
//   - GLM: requests/cycle=400
type PlatformLimit struct {
	Type          LimitType   `json:"type"`            // requests, tokens, compute_points, credit
	Window        LimitWindow `json:"window"`          // minute, hour, 3h, 8h, day, cycle, never
	Value         int         `json:"value"`           // the limit amount (requests, tokens, points, cents for credit)
	AlertPct      int         `json:"alert_pct"`       // percentage threshold for alerting (default 80)
	WindowSeconds int         `json:"window_seconds"`  // custom window duration in seconds (for cycle type)
	ResetsAt      string      `json:"resets_at,omitempty"` // ISO timestamp when window resets (for credit/cycle)
}

// PlatformProfileV2 tracks usage for a platform against its configured limits.
// Unlike the old PlatformProfile with hardcoded 3h/8h/day/session windows,
// this version uses a dynamic set of limits driven by config.
type PlatformProfileV2 struct {
	PlatformID string
	Limits     []PlatformLimit
	// Track usage per limit index. Each entry tracks requests and tokens
	// against its corresponding PlatformLimit in the Limits slice.
	Usage     []LimitUsage
	CreatedAt time.Time
}

// LimitUsage tracks current usage against a single PlatformLimit.
type LimitUsage struct {
	Requests    int       `json:"requests"`
	Tokens      int       `json:"tokens"`
	Points      int       `json:"points"`      // for compute_points type
	CreditCents int       `json:"credit_cents"` // for credit type (in cents)
	WindowStart time.Time `json:"window_start"`
	ResetAt     time.Time `json:"reset_at"`
}

// PlatformUsageTrackerV2 is the config-driven replacement for PlatformUsageTracker.
// It supports arbitrary limit types and windows per platform, driven entirely
// by the connectors.json limit_schema configuration.
type PlatformUsageTrackerV2 struct {
	mu       sync.Mutex
	profiles map[string]*PlatformProfileV2
}

func NewPlatformUsageTrackerV2() *PlatformUsageTrackerV2 {
	return &PlatformUsageTrackerV2{
		profiles: make(map[string]*PlatformProfileV2),
	}
}

// RegisterPlatformFromConfig registers a platform with limits parsed from
// the connectors.json limit_schema. The limits parameter is the new rich format.
func (pt *PlatformUsageTrackerV2) RegisterPlatformFromConfig(platformID string, limits []PlatformLimit) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()

	if profile, exists := pt.profiles[platformID]; exists {
		// Update limits but preserve usage data where window types match
		pt.mergeLimits(profile, limits)
		return
	}

	usage := make([]LimitUsage, len(limits))
	for i, lim := range limits {
		usage[i] = pt.newUsageForLimit(lim, now)
	}

	pt.profiles[platformID] = &PlatformProfileV2{
		PlatformID: platformID,
		Limits:     limits,
		Usage:      usage,
		CreatedAt:  now,
	}

	log.Printf("[PlatformTrackerV2] Registered %s with %d limits", platformID, len(limits))
}

func (pt *PlatformUsageTrackerV2) newUsageForLimit(lim PlatformLimit, now time.Time) LimitUsage {
	var resetAt time.Time
	switch lim.Window {
	case WindowMinute:
		resetAt = now.Add(time.Minute)
	case WindowHour:
		resetAt = now.Add(time.Hour)
	case Window3h:
		resetAt = now.Add(3 * time.Hour)
	case Window8h:
		resetAt = now.Add(8 * time.Hour)
	case WindowDay:
		// Align to next midnight local
		tomorrow := now.AddDate(0, 0, 1)
		resetAt = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
	case WindowCycle:
		if lim.WindowSeconds > 0 {
			resetAt = now.Add(time.Duration(lim.WindowSeconds) * time.Second)
		} else {
			resetAt = now.Add(24 * time.Hour) // fallback
		}
	case WindowNever:
		resetAt = time.Time{} // zero = never resets
	}
	return LimitUsage{
		WindowStart: now,
		ResetAt:     resetAt,
	}
}

func (pt *PlatformUsageTrackerV2) mergeLimits(profile *PlatformProfileV2, newLimits []PlatformLimit) {
	// Build usage for new limits, carrying over existing data where possible
	oldUsage := profile.Usage
	usage := make([]LimitUsage, len(newLimits))
	now := time.Now()

	for i, lim := range newLimits {
		// Try to find matching old limit by type+window
		matched := false
		for j, oldLim := range profile.Limits {
			if oldLim.Type == lim.Type && oldLim.Window == lim.Window {
				usage[i] = oldUsage[j]
				matched = true
				break
			}
		}
		if !matched {
			usage[i] = pt.newUsageForLimit(lim, now)
		}
	}

	profile.Limits = newLimits
	profile.Usage = usage
}

// RecordUsage records that a request was sent to a platform.
// tokensUsed is the estimated token count for the request.
// costCents is the cost in cents (for credit-based limits, 0 if not applicable).
func (pt *PlatformUsageTrackerV2) RecordUsage(ctx context.Context, platformID string, tokensUsed int, costCents int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return
	}

	now := time.Now()
	for i, lim := range profile.Limits {
		// Reset expired windows
		usage := &profile.Usage[i]
		if !usage.ResetAt.IsZero() && now.After(usage.ResetAt) {
			*usage = pt.newUsageForLimit(lim, now)
		}

		// Increment the right counter based on limit type
		switch lim.Type {
		case LimitTypeRequests, LimitTypeTokens:
			usage.Requests++
			usage.Tokens += tokensUsed
		case LimitTypeComputePoints:
			usage.Requests++
			usage.Points += tokensUsed // approximate: tokens as point proxy
		case LimitTypeCredit:
			usage.CreditCents += costCents
		}
	}
}

// CanMakeRequest checks whether the platform has capacity for an additional request.
// Returns whether the request can proceed, and if not, how long to wait.
func (pt *PlatformUsageTrackerV2) CanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) (bool, time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return true, 0 // no limits = always allowed
	}

	now := time.Now()
	var maxWait time.Duration

	for i, lim := range profile.Limits {
		usage := &profile.Usage[i]

		// Reset expired windows first
		if !usage.ResetAt.IsZero() && now.After(usage.ResetAt) {
			*usage = pt.newUsageForLimit(lim, now)
		}

		alertPct := lim.AlertPct
		if alertPct <= 0 || alertPct > 100 {
			alertPct = 80
		}
		threshold := int(float64(lim.Value) * float64(alertPct) / 100.0)

		var blocked bool
		var waitTime time.Duration

		switch lim.Type {
		case LimitTypeRequests:
			if usage.Requests >= threshold {
				blocked = true
			}
		case LimitTypeTokens:
			if usage.Tokens+estimatedTokens > threshold {
				blocked = true
			}
		case LimitTypeComputePoints:
			if usage.Points >= threshold {
				blocked = true
			}
		case LimitTypeCredit:
			// Credit: check if remaining would go below threshold
			remaining := lim.Value - usage.CreditCents
			if remaining <= threshold {
				blocked = true
			}
		}

		if blocked {
			if usage.ResetAt.IsZero() {
				// Credit/cycle with no reset = permanent block until external update
				waitTime = 24 * time.Hour
			} else {
				waitTime = usage.ResetAt.Sub(now)
				if waitTime < 0 {
					waitTime = time.Minute // minimum wait
				}
			}
			if waitTime > maxWait {
				maxWait = waitTime
			}
		}
	}

	if maxWait > 0 {
		return false, maxWait
	}
	return true, 0
}

// GetRemainingUsage returns the current usage stats for a platform.
func (pt *PlatformUsageTrackerV2) GetRemainingUsage(platformID string) map[string]interface{} {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return nil
	}

	result := make(map[string]interface{})
	for i, lim := range profile.Limits {
		usage := profile.Usage[i]
		key := fmt.Sprintf("%s_%s", lim.Type, lim.Window)
		result[key] = map[string]interface{}{
			"limit":      lim.Value,
			"requests":   usage.Requests,
			"tokens":     usage.Tokens,
			"remaining":  lim.Value - usage.Requests,
			"reset_at":   usage.ResetAt,
		}
	}
	return result
}

// UpdateCreditBalance updates the credit remaining for credit-based platforms.
// Called by external credit polling (DeepSeek balance API, etc).
func (pt *PlatformUsageTrackerV2) UpdateCreditBalance(platformID string, remainingCents int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	profile, exists := pt.profiles[platformID]
	if !exists {
		return
	}

	for i, lim := range profile.Limits {
		if lim.Type == LimitTypeCredit {
			// Update the limit value to reflect actual remaining credit
			profile.Limits[i].Value = remainingCents
			// Reset usage since the balance API gives us absolute remaining
			profile.Usage[i].CreditCents = 0
			break
		}
	}
}

// HasPlatform checks if a platform is registered.
func (pt *PlatformUsageTrackerV2) HasPlatform(platformID string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	_, exists := pt.profiles[platformID]
	return exists
}

// AllPlatformStatus returns status of all tracked platforms.
func (pt *PlatformUsageTrackerV2) AllPlatformStatus() map[string]map[string]interface{} {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	result := make(map[string]map[string]interface{})
	for id := range pt.profiles {
		result[id] = pt.GetRemainingUsage(id)
	}
	return result
}

// PersistToDatabase saves all platform usage data to the platform_usage table.
func (pt *PlatformUsageTrackerV2) PersistToDatabase(ctx context.Context, db Querier) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, profile := range pt.profiles {
		windows := make(map[string]interface{})
		for i, lim := range profile.Limits {
			key := fmt.Sprintf("%s_%s", lim.Type, lim.Window)
			usage := profile.Usage[i]
			windows[key] = map[string]interface{}{
				"limit":       lim.Value,
				"requests":    usage.Requests,
				"tokens":      usage.Tokens,
				"points":      usage.Points,
				"credit_cents": usage.CreditCents,
				"window_start": usage.WindowStart,
				"reset_at":     usage.ResetAt,
			}
		}

		data, _ := json.Marshal(map[string]interface{}{
			"platform_id":   profile.PlatformID,
			"usage_windows": windows,
		})

		_, err := db.RPC(ctx, "upsert_platform_usage", map[string]interface{}{
			"p_platform_id":   profile.PlatformID,
			"p_usage_windows": string(data),
		})
		if err != nil {
			log.Printf("[PlatformTrackerV2] Failed to persist %s: %v", profile.PlatformID, err)
		}
	}
}

// LoadFromDatabase restores platform usage data from the platform_usage table.
func (pt *PlatformUsageTrackerV2) LoadFromDatabase(ctx context.Context, db Querier) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	raw, err := db.RPC(ctx, "get_all_platform_usage", map[string]interface{}{})
	if err != nil {
		log.Printf("[PlatformTrackerV2] Failed to load from DB: %v", err)
		return
	}

	var entries []struct {
		PlatformID   string                 `json:"platform_id"`
		UsageWindows map[string]interface{} `json:"usage_windows"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		log.Printf("[PlatformTrackerV2] Failed to parse DB data: %v", err)
		return
	}

	now := time.Now()
	for _, entry := range entries {
		profile, exists := pt.profiles[entry.PlatformID]
		if !exists {
			continue // platform not configured, skip stale data
		}

		// Restore usage data for matching limits
		for i, lim := range profile.Limits {
			key := fmt.Sprintf("%s_%s", lim.Type, lim.Window)
			if windowData, ok := entry.UsageWindows[key]; ok {
				if wm, ok := windowData.(map[string]interface{}); ok {
					usage := &profile.Usage[i]
					if v, ok := wm["requests"].(float64); ok {
						usage.Requests = int(v)
					}
					if v, ok := wm["tokens"].(float64); ok {
						usage.Tokens = int(v)
					}
					if v, ok := wm["credit_cents"].(float64); ok {
						usage.CreditCents = int(v)
					}
					// Check if window is expired
					if !usage.ResetAt.IsZero() && now.After(usage.ResetAt) {
						*usage = pt.newUsageForLimit(lim, now)
					}
				}
			}
		}
	}

	log.Printf("[PlatformTrackerV2] Loaded usage data for %d platforms", len(entries))
}
