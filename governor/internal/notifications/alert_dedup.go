package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

// AlertDedup provides one-time email deduplication for any alert type.
// Key format: "category:identifier" (e.g., "credit_low:deepseek-v4-flash").
// Once an alert is sent, it will not resend until explicitly cleared
// (typically when the condition resolves, e.g., credits topped up).
type AlertDedup struct {
	mu    sync.RWMutex
	sent  map[string]bool
	email string // path to notify_email.py
}

// NewAlertDedup creates a dedup tracker.
func NewAlertDedup() *AlertDedup {
	return &AlertDedup{
		sent:  make(map[string]bool),
		email: "/home/vibes/vibepilot/scripts/notify_email.py",
	}
}

// AlreadySent returns true if this exact alert key was already emailed.
func (d *AlertDedup) AlreadySent(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sent[key]
}

// MarkSent records that an alert was sent.
func (d *AlertDedup) MarkSent(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sent[key] = true
}

// Clear removes a specific key (e.g., when credits are topped back up).
func (d *AlertDedup) Clear(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sent, key)
}

// ClearAll removes all dedup state (e.g., when all alerts resolve).
func (d *AlertDedup) ClearAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sent = make(map[string]bool)
}

// SendEmailIfNew sends an email only if this key has not been alerted before.
// Returns true if email was sent, false if already sent (skipped).
func (d *AlertDedup) SendEmailIfNew(key string, subject string, body string) bool {
	if d.AlreadySent(key) {
		return false
	}

	cmd := exec.Command("python3", d.email, "--subject", subject, "--body", body)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[Notify] Email failed for key %s: %v — %s", key, err, string(output))
		return false
	}

	d.MarkSent(key)
	log.Printf("[Notify] Email sent for %s", key)
	return true
}

// SendEmailUnconditional sends an email regardless of dedup state.
// Use for test messages or forced alerts only.
func SendEmailUnconditional(subject string, body string) error {
	cmd := exec.Command("python3", "/home/vibes/vibepilot/scripts/notify_email.py",
		"--subject", subject, "--body", body)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("email failed: %v — %s", err, string(output))
	}
	return nil
}

// SubscriptionAlert represents a single DB threshold check result.
type SubscriptionAlert struct {
	ModelID        string  `json:"model_id"`
	AlertType      string  `json:"alert_type"`
	Message        string  `json:"message"`
	CurrentValue   float64 `json:"current_value"`
	ThresholdValue float64 `json:"threshold_value"`
}

// DedupKey returns the standard dedup key for a subscription alert.
func (a SubscriptionAlert) DedupKey() string {
	return a.AlertType + ":" + a.ModelID
}

// DBQuerier is the minimal interface needed to check subscription thresholds.
type DBQuerier interface {
	RPC(ctx context.Context, name string, params map[string]any) ([]byte, error)
}

// CheckAndAlert checks DB for subscription threshold alerts, deduplicates,
// and sends one email for any new alerts. Returns count of new alerts emailed.
func (d *AlertDedup) CheckAndAlert(ctx context.Context, db DBQuerier) int {
	data, err := db.RPC(ctx, "check_subscription_thresholds", map[string]any{})
	if err != nil {
		log.Printf("[Notify] check_subscription_thresholds error: %v", err)
		return 0
	}

	if len(data) == 0 || string(data) == "[]" {
		// All clear -- reset dedup so next threshold breach triggers fresh alert
		d.ClearAll()
		return 0
	}

	var alerts []SubscriptionAlert
	if err := json.Unmarshal(data, &alerts); err != nil || len(alerts) == 0 {
		return 0
	}

	// Filter to only new (not yet alerted) keys
	var newAlerts []SubscriptionAlert
	for _, a := range alerts {
		if !d.AlreadySent(a.DedupKey()) {
			newAlerts = append(newAlerts, a)
			d.MarkSent(a.DedupKey())
		}
	}

	if len(newAlerts) == 0 {
		return 0
	}

	// Build email body
	var lines []string
	for _, a := range newAlerts {
		lines = append(lines, fmt.Sprintf("%s: %s (current: $%.2f, threshold: $%.2f)",
			a.ModelID, a.AlertType, a.CurrentValue, a.ThresholdValue))
		if a.Message != "" {
			lines = append(lines, "  "+a.Message)
		}
	}

	subject := fmt.Sprintf("[VibePilot] Credit Alert: %d model(s) below threshold", len(newAlerts))
	body := "The following models have credit below their alert thresholds:\n\n" +
		strings.Join(lines, "\n") +
		"\n\nTop up at the provider's dashboard to avoid service interruption.\n\n" + "\u2014 VibePilot Governor"

	_ = SendEmailUnconditional(subject, body)
	return len(newAlerts)
}
