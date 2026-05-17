package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/notifications"
)

// CreditDB is the minimal DB interface the credit poller needs.
// Uses the same RPC pattern as the rest of the governor.
type CreditDB interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}

// CreditPoller periodically checks credit balances for paid API providers
// and updates the platform tracker V2 with current balances.
// It also syncs the live balance to the models table and fires alerts
// when credit drops below the configured threshold.
type CreditPoller struct {
	mu        sync.RWMutex
	tracker   *PlatformUsageTrackerV2
	vault     VaultKeyGetter
	db        CreditDB
	client    *http.Client
	interval  time.Duration
	platforms []creditPollTarget
}

// creditPollTarget defines a provider whose credit balance should be polled.
type creditPollTarget struct {
	PlatformID        string  // matches connectors.json id
	ModelID           string  // matches models table id for DB sync
	BalanceURL        string  // API endpoint for balance
	APIKeyRef         string  // vault key reference
	PollInterval      time.Duration // active polling interval (when recently used)
	IdleInterval      time.Duration // slow check interval (when idle)
	LastBalance       int       // cents
	InitialBalance    int       // cents — first successful poll sets this
	LastChecked       time.Time
	AlertThresholdPct float64 // alert at this % of initial credit
	AlertFired        bool     // dedup: true after first alert fires
	AlertCooldown     time.Duration // minimum time between repeat alerts
	LastAlertTime     time.Time
}

// deepseekBalanceResponse matches the DeepSeek /user/balance API response.
type deepseekBalanceResponse struct {
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// NewCreditPoller creates a credit poller from connector config.
func NewCreditPoller(tracker *PlatformUsageTrackerV2, vault VaultKeyGetter, db CreditDB) *CreditPoller {
	return &CreditPoller{
		tracker: tracker,
		vault:   vault,
		db:      db,
		client:  &http.Client{Timeout: 15 * time.Second},
		platforms: []creditPollTarget{
			{
				PlatformID:        "deepseek-api",
				ModelID:           "deepseek-v4-flash",
				BalanceURL:        "https://api.deepseek.com/user/balance",
				APIKeyRef:         "deepseek-v4-flash",
				PollInterval:      2 * time.Hour,  // active: check every 2h when recently used
				IdleInterval:      24 * time.Hour,  // idle: check once per day when not used
				AlertThresholdPct: 0.2,             // alert when 20% or less remaining
				AlertCooldown:     24 * time.Hour,
			},
		},
		interval: 30 * time.Minute, // base tick: evaluate which providers need polling
	}
}

// shouldPoll determines if a provider needs polling based on usage recency.
// If the model was used in the last 24h, use active interval. Otherwise use idle interval.
func (cp *CreditPoller) shouldPoll(target *creditPollTarget) bool {
	if target.LastChecked.IsZero() {
		return true // never checked
	}

	// Check if model was recently used by querying DB
	activeInterval := target.PollInterval
	if cp.db != nil {
		ctx := context.Background()
		data, err := cp.db.RPC(ctx, "get_model_last_used", map[string]interface{}{
			"p_model_id": target.ModelID,
		})
		if err == nil && len(data) > 0 {
			var result []map[string]interface{}
			if json.Unmarshal(data, &result) == nil && len(result) > 0 {
				if lastUsed, ok := result[0]["last_run_at"].(string); ok && lastUsed != "" {
					if t, err := time.Parse(time.RFC3339, lastUsed); err == nil {
						sinceUse := time.Since(t)
						if sinceUse < 24*time.Hour {
							// Recently used: poll at active interval
							activeInterval = target.PollInterval
						} else {
							// Idle: poll at slow idle interval
							activeInterval = target.IdleInterval
						}
					}
				}
			}
		}
	}

	return time.Since(target.LastChecked) >= activeInterval
}

// StartBackgroundPolling runs periodic credit checks in a goroutine.
func (cp *CreditPoller) StartBackgroundPolling(ctx context.Context) {
	if cp.tracker == nil || cp.vault == nil {
		log.Printf("[CreditPoller] Skipping: tracker or vault not available")
		return
	}

	log.Printf("[CreditPoller] Starting background polling for %d providers", len(cp.platforms))

	go func() {
		// Initial check on startup
		cp.pollAll(ctx)

		ticker := time.NewTicker(cp.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("[CreditPoller] Stopping background polling")
				return
			case <-ticker.C:
				cp.pollAll(ctx)
			}
		}
	}()
}

func (cp *CreditPoller) pollAll(ctx context.Context) {
	for i := range cp.platforms {
		target := &cp.platforms[i]

		// Skip providers that don't need polling yet (usage-aware)
		if !cp.shouldPoll(target) {
			continue
		}
		balanceCents, err := cp.pollProvider(ctx, target)
		if err != nil {
			log.Printf("[CreditPoller] Failed to poll %s: %v", target.PlatformID, err)
			continue
		}

		cp.mu.Lock()
		oldBalance := target.LastBalance
		target.LastBalance = balanceCents
		target.LastChecked = time.Now()

		// Set initial balance on first successful poll
		if target.InitialBalance == 0 && balanceCents > 0 {
			target.InitialBalance = balanceCents
			log.Printf("[CreditPoller] %s initial balance set: $%.2f", target.PlatformID, float64(balanceCents)/100)
		}
		cp.mu.Unlock()

		// Update the in-memory platform tracker
		if cp.tracker != nil {
			cp.tracker.UpdateCreditBalance(target.PlatformID, balanceCents)
		}

		// Sync live balance to DB so dashboard/check_subscription_thresholds sees it
		// This captures ALL credit consumption (pipeline tasks + Hermes sessions + anything else)
		cp.syncBalanceToDB(ctx, target, balanceCents)

		if oldBalance != balanceCents {
			log.Printf("[CreditPoller] %s balance changed: $%.2f -> $%.2f",
				target.PlatformID, float64(oldBalance)/100, float64(balanceCents)/100)
		}

		log.Printf("[CreditPoller] %s balance: $%.2f", target.PlatformID, float64(balanceCents)/100)

		// Check DB-defined thresholds and fire ONE email per credit purchase cycle
		cp.checkCreditAlerts(ctx)
	}
}

// syncBalanceToDB writes the live-polled balance to the models table.
// This is the source of truth for dashboard ROI and alert RPCs.
func (cp *CreditPoller) syncBalanceToDB(ctx context.Context, target *creditPollTarget, balanceCents int) {
	if cp.db == nil || target.ModelID == "" {
		return
	}

	balanceUSD := float64(balanceCents) / 100.0

	_, err := cp.db.RPC(ctx, "update_model_credit_balance", map[string]interface{}{
		"p_model_id":              target.ModelID,
		"p_credit_remaining_usd":  balanceUSD,
	})
	if err != nil {
		log.Printf("[CreditPoller] ERROR syncing balance to DB for %s: %v", target.ModelID, err)
	}
}

// checkThreshold fires an alert when credit drops below the configured threshold.
// Uses AlertFired + AlertCooldown to avoid spamming.
// checkCreditAlerts uses a single DB column (credit_alert_sent) to guarantee
// exactly ONE email per credit purchase cycle. No in-memory state. Survives restarts.
// The RPC get_models_needing_credit_alert() atomically checks threshold AND marks sent.
// The flag resets only when credits are topped back above threshold (reset_credit_alert_if_topped_up).
func (cp *CreditPoller) checkCreditAlerts(ctx context.Context) {
	if cp.db == nil {
		return
	}

	// First, reset alerts for any models that have been topped up
	cp.db.RPC(ctx, "reset_credit_alert_if_topped_up", map[string]interface{}{})

	// Then, atomically find models that need alerting AND mark them sent
	data, err := cp.db.RPC(ctx, "get_models_needing_credit_alert", map[string]interface{}{})
	if err != nil {
		log.Printf("[CreditPoller] checkCreditAlerts error: %v", err)
		return
	}
	if len(data) == 0 || string(data) == "[]" {
		return
	}

	var alerts []struct {
		ModelID   string  `json:"model_id"`
		Remaining float64 `json:"remaining"`
		Msg       string  `json:"msg"`
	}
	if err := json.Unmarshal(data, &alerts); err != nil || len(alerts) == 0 {
		return
	}

	// Build one email with all alerts
	var lines []string
	for _, a := range alerts {
		lines = append(lines, fmt.Sprintf("- %s: $%.2f remaining", a.ModelID, a.Remaining))
		lines = append(lines, "  "+a.Msg)
	}

	subject := fmt.Sprintf("[VibePilot] Credit Alert: %d model(s) below threshold", len(alerts))
	body := "The following models have credit below their alert thresholds:\n\n" +
		strings.Join(lines, "\n") +
		"\n\nTop up at the provider's dashboard to avoid service interruption.\n\n" +
		"You will NOT receive another alert until credits are topped up.\n\n" +
		"— VibePilot Governor"

	// Fire email directly - the DB already marked them as sent, so this is safe
	if err := notifications.SendEmailUnconditional(subject, body); err != nil {
		log.Printf("[CreditPoller] Failed to send credit alert email: %v", err)
		// Un-mark so it retries next cycle
		for _, a := range alerts {
			cp.db.RPC(ctx, "update_model", map[string]interface{}{
				"p_id":     a.ModelID,
				"p_updates": map[string]any{"credit_alert_sent": false},
			})
		}
		return
	}
	log.Printf("[CreditPoller] Credit alert email sent for %d model(s)", len(alerts))
}


func (cp *CreditPoller) pollProvider(ctx context.Context, target *creditPollTarget) (int, error) {
	switch target.PlatformID {
	case "deepseek-api":
		return cp.pollDeepSeek(ctx, target)
	default:
		return cp.pollGeneric(ctx, target)
	}
}

func (cp *CreditPoller) pollDeepSeek(ctx context.Context, target *creditPollTarget) (int, error) {
	apiKey, err := cp.vault.GetSecret(ctx, target.APIKeyRef)
	if err != nil {
		return 0, fmt.Errorf("vault key %s: %w", target.APIKeyRef, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", target.BalanceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := cp.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var balResp deepseekBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balResp); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	// Sum all balance infos, convert to cents
	totalCents := 0
	for _, info := range balResp.BalanceInfos {
		if info.Currency == "USD" || info.Currency == "" {
			var dollars float64
			fmt.Sscanf(strings.TrimSpace(info.TotalBalance), "%f", &dollars)
			totalCents += int(dollars * 100)
		}
	}

	return totalCents, nil
}

// pollGeneric is a placeholder for future providers that expose balance endpoints.
func (cp *CreditPoller) pollGeneric(ctx context.Context, target *creditPollTarget) (int, error) {
	return 0, fmt.Errorf("no balance endpoint configured for %s", target.PlatformID)
}

// GetBalances returns the current cached balances for all polled providers.
func (cp *CreditPoller) GetBalances() map[string]int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make(map[string]int, len(cp.platforms))
	for _, p := range cp.platforms {
		result[p.PlatformID] = p.LastBalance
	}
	return result
}

// min is provided by model_scanner.go in the same package.
