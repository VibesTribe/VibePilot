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
	platforms []creditPollTarget
	interval  time.Duration
	alerts    *notifications.AlertDedup // centralized one-email-ever dedup for all alert types
}

// creditPollTarget defines a provider whose credit balance should be polled.
type creditPollTarget struct {
	PlatformID        string  // matches connectors.json id
	ModelID           string  // matches models table id for DB sync
	BalanceURL        string  // API endpoint for balance
	APIKeyRef         string  // vault key reference
	PollInterval      time.Duration
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
		alerts:  notifications.NewAlertDedup(),
		platforms: []creditPollTarget{
			{
				PlatformID:        "deepseek-api",
				ModelID:           "deepseek-v4-flash",
				BalanceURL:        "https://api.deepseek.com/user/balance",
				APIKeyRef:         "deepseek-v4-flash",
				PollInterval:      1 * time.Hour,
				AlertThresholdPct: 0.2, // alert when 20% or less remaining
				AlertCooldown:     6 * time.Hour,
			},
		},
		interval: 1 * time.Hour, // default check interval
	}
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

		// Check DB-defined thresholds and fire email alert if any model is below
		cp.checkDBThresholds(ctx)
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
func (cp *CreditPoller) checkThreshold(target *creditPollTarget, balanceCents int) {
	if target.InitialBalance <= 0 || target.AlertThresholdPct <= 0 {
		return
	}

	thresholdCents := int(float64(target.InitialBalance) * target.AlertThresholdPct)
	if balanceCents > thresholdCents {
		return // still above threshold
	}

	// Check cooldown — don't alert more than once per cooldown period
	cp.mu.Lock()
	if target.AlertFired && time.Since(target.LastAlertTime) < target.AlertCooldown {
		cp.mu.Unlock()
		return
	}
	target.AlertFired = true
	target.LastAlertTime = time.Now()
	cp.mu.Unlock()

	pctRemaining := float64(balanceCents) / float64(target.InitialBalance) * 100

	log.Printf("[CreditPoller] *** CREDIT ALERT *** %s (%s): $%.2f remaining (%.0f%% of initial $%.2f) — below %.0f%% threshold",
		target.PlatformID, target.ModelID,
		float64(balanceCents)/100, pctRemaining,
		float64(target.InitialBalance)/100,
		target.AlertThresholdPct*100)

	// Fire email notification in background
	go cp.sendCreditAlert(target, balanceCents, pctRemaining)
}

// checkDBThresholds uses the centralized AlertDedup to check subscription
// thresholds and send exactly one email per alert. Dedup resets only when
// credits are topped back above threshold (DB returns no alerts).
func (cp *CreditPoller) checkDBThresholds(ctx context.Context) {
	if cp.db == nil {
		return
	}

	// Delegate to the centralized alert system
	count := cp.alerts.CheckAndAlert(ctx, creditDBWrapper{db: cp.db})
	if count > 0 {
		log.Printf("[CreditPoller] DB threshold alert email sent for %d alert(s)", count)
	}
}

// creditDBWrapper adapts CreditDB to the notifications.DBQuerier interface.
type creditDBWrapper struct {
	db CreditDB
}

func (w creditDBWrapper) RPC(ctx context.Context, name string, params map[string]any) ([]byte, error) {
	// Convert map[string]any to map[string]interface{} for CreditDB compatibility
	converted := make(map[string]interface{}, len(params))
	for k, v := range params {
		converted[k] = v
	}
	return w.db.RPC(ctx, name, converted)
}

// sendCreditAlert dispatches an email notification for low credit.
// Uses centralized AlertDedup to guarantee exactly one email per alert.
func (cp *CreditPoller) sendCreditAlert(target *creditPollTarget, balanceCents int, pctRemaining float64) {
	dedupKey := "pct_threshold:" + target.ModelID

	subject := fmt.Sprintf("[VibePilot] Credit Alert: %s at %.0f%% ($%.2f remaining)",
		target.ModelID, pctRemaining, float64(balanceCents)/100)

	body := fmt.Sprintf(
		"VibePilot Credit Alert\n\n"+
			"Model: %s\n"+
			"Provider: %s\n"+
			"Remaining: $%.2f (%.0f%% of $%.2f)\n"+
			"Threshold: %.0f%%\n\n"+
			"This alert fires when credit drops below the threshold.\n"+
			"Top up at the provider's dashboard to avoid service interruption.\n\n"+
			"— VibePilot Governor",
		target.ModelID, target.PlatformID,
		float64(balanceCents)/100, pctRemaining,
		float64(target.InitialBalance)/100,
		target.AlertThresholdPct*100,
	)

	if sent := cp.alerts.SendEmailIfNew(dedupKey, subject, body); sent {
		log.Printf("[CreditPoller] Credit alert email sent for %s", target.ModelID)
	}
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
