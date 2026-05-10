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
)

// CreditPoller periodically checks credit balances for paid API providers
// and updates the platform tracker V2 with current balances.
// It follows the same background ticker pattern as ModelScanner.
type CreditPoller struct {
	mu       sync.RWMutex
	tracker  *PlatformUsageTrackerV2
	vault    VaultKeyGetter
	client   *http.Client
	platforms []creditPollTarget
	interval time.Duration
}

// creditPollTarget defines a provider whose credit balance should be polled.
type creditPollTarget struct {
	PlatformID    string // matches connectors.json id
	BalanceURL    string // API endpoint for balance
	APIKeyRef     string // vault key reference
	PollInterval  time.Duration
	LastBalance   int    // cents
	LastChecked   time.Time
	AlertThresholdPct float64 // alert at this % of initial credit
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
func NewCreditPoller(tracker *PlatformUsageTrackerV2, vault VaultKeyGetter) *CreditPoller {
	return &CreditPoller{
		tracker: tracker,
		vault:   vault,
		client:  &http.Client{Timeout: 15 * time.Second},
		platforms: []creditPollTarget{
			{
				PlatformID:        "deepseek-api",
				BalanceURL:        "https://api.deepseek.com/user/balance",
				APIKeyRef:         "deepseek-v4-flash",
				PollInterval:      1 * time.Hour,
				AlertThresholdPct: 0.2, // alert when 20% or less remaining
			},
		},
		interval: 1 * time.Hour, // default check interval
	}
}

// StartBackgroundPolling runs periodic credit checks in a goroutine.
// Follows the same pattern as ModelScanner.StartBackgroundScanner.
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
		cp.mu.Unlock()

		// Update the platform tracker
		if cp.tracker != nil {
			cp.tracker.UpdateCreditBalance(target.PlatformID, balanceCents)
		}

		if oldBalance != balanceCents {
			log.Printf("[CreditPoller] %s balance changed: $%.2f -> $%.2f",
				target.PlatformID, float64(oldBalance)/100, float64(balanceCents)/100)
		}

		log.Printf("[CreditPoller] %s balance: $%.2f", target.PlatformID, float64(balanceCents)/100)
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
