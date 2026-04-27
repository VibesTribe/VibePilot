package runtime

import (
	"context"
	"log"
	"sync"
	"time"
)

// CooldownWatcher monitors models that recently exited cooldown and probes
// them to verify they're actually healthy before the router sends traffic.
// Without this, a model with a dead key would cycle: cooldown expires → router
// tries it → fails → cooldown again → repeat forever.
//
// It runs as a background goroutine, checking every pollInterval. For each
// model whose cooldown recently expired, it runs a health probe via the
// connector runner. Failed probes extend the cooldown; successful probes
// log confirmation.
type CooldownWatcher struct {
	tracker     *UsageTracker
	factory     ConnectorFactory
	cfg         *Config
	db          Querier
	pollInterval time.Duration
	probeTimeout time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc

	// Track which models we've already probed since their cooldown expired,
	// so we don't re-probe every poll cycle for the same expiry.
	probedSinceExpiry map[string]time.Time // modelID → cooldownExpiry that we probed
}

// ConnectorFactory abstracts the SessionFactory's connector lookup.
// This avoids importing the full SessionFactory type (circular dep risk).
type ConnectorFactory interface {
	GetConnector(id string) (ConnectorRunner, bool)
}

// NewCooldownWatcher creates a new watcher. Call Start() to begin background polling.
func NewCooldownWatcher(tracker *UsageTracker, factory ConnectorFactory, cfg *Config, db Querier) *CooldownWatcher {
	return &CooldownWatcher{
		tracker:          tracker,
		factory:          factory,
		cfg:              cfg,
		db:               db,
		pollInterval:     2 * time.Minute,
		probeTimeout:     15 * time.Second,
		probedSinceExpiry: make(map[string]time.Time),
	}
}

// SetPollInterval changes how often the watcher checks for expired cooldowns.
// Must be called before Start(). Default is 2 minutes.
func (w *CooldownWatcher) SetPollInterval(d time.Duration) {
	w.pollInterval = d
}

// SetProbeTimeout changes the per-model probe timeout. Default is 15 seconds.
func (w *CooldownWatcher) SetProbeTimeout(d time.Duration) {
	w.probeTimeout = d
}

// Start begins the background cooldown watcher. Call Stop() to shut it down.
func (w *CooldownWatcher) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	ctx, w.cancel = context.WithCancel(ctx)
	w.mu.Unlock()

	go w.loop(ctx)
	log.Printf("[CooldownWatcher] Started (poll every %v, probe timeout %v)", w.pollInterval, w.probeTimeout)
}

// Stop shuts down the watcher.
func (w *CooldownWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
}

func (w *CooldownWatcher) loop(ctx context.Context) {
	// First check after a short delay to let the system settle
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Run first check immediately
	w.check(ctx)

	for {
		select {
		case <-ticker.C:
			w.check(ctx)
		case <-ctx.Done():
			log.Printf("[CooldownWatcher] Stopped")
			return
		}
	}
}

// check scans all registered models for recently-expired cooldowns and probes them.
func (w *CooldownWatcher) check(ctx context.Context) {
	w.tracker.mu.RLock()
	now := time.Now()
	type modelInfo struct {
		id          string
		expiredAt   time.Time // when cooldown actually expired
		connectorID string
	}
	var toProbe []modelInfo

	for id, usage := range w.tracker.models {
		// Only check active models
		if usage.Profile.Status != "active" {
			continue
		}
		// Must have had a cooldown that recently expired
		if usage.CooldownExpiresAt == nil {
			continue
		}
		// Cooldown must have expired (past) or be about to expire (within 30s)
		if usage.CooldownExpiresAt.After(now.Add(30 * time.Second)) {
			continue
		}

		// Skip if we already probed for this particular cooldown expiry
		if lastProbed, ok := w.probedSinceExpiry[id]; ok && !lastProbed.Before(*usage.CooldownExpiresAt) {
			continue
		}

		// Find a connector to probe through
		connID := w.findConnectorForModel(usage.Profile.AccessVia)
		if connID == "" {
			continue
		}

		toProbe = append(toProbe, modelInfo{
			id:          id,
			expiredAt:   *usage.CooldownExpiresAt,
			connectorID: connID,
		})
	}
	w.tracker.mu.RUnlock()

	if len(toProbe) == 0 {
		return
	}

	log.Printf("[CooldownWatcher] Probing %d model(s) with recently expired cooldowns", len(toProbe))

	for _, m := range toProbe {
		if ctx.Err() != nil {
			return
		}
		w.probeModel(ctx, m.id, m.connectorID, m.expiredAt)
		// Stagger probes to avoid hitting rate limits
		time.Sleep(2 * time.Second)
	}
}

// probeModel sends a minimal health check to a model and handles the result.
func (w *CooldownWatcher) probeModel(ctx context.Context, modelID, connectorID string, cooldownExpiry time.Time) {
	probeCtx, cancel := context.WithTimeout(ctx, w.probeTimeout)
	defer cancel()

	runner, ok := w.factory.GetConnector(connectorID)
	if !ok {
		log.Printf("[CooldownWatcher] No runner for connector %s (model %s), marking probed anyway", connectorID, modelID)
		w.probedSinceExpiry[modelID] = cooldownExpiry
		return
	}

	hc, ok := runner.(HealthChecker)
	if !ok {
		// CLI connectors don't implement HealthChecker — that's fine
		w.probedSinceExpiry[modelID] = cooldownExpiry
		return
	}

	err := hc.HealthCheck(probeCtx)
	if err != nil {
		// Probe failed: extend cooldown. The model isn't actually healthy.
		log.Printf("[CooldownWatcher] PROBE FAILED: model %s via %s — %v (extending cooldown)", modelID, connectorID, err)
		w.tracker.RecordRateLimit(ctx, modelID)
		if w.db != nil {
			w.tracker.PersistToDatabase(ctx)
		}
	} else {
		log.Printf("[CooldownWatcher] PROBE OK: model %s via %s — healthy after cooldown expiry", modelID, connectorID)
	}

	// Mark as probed for this expiry so we don't re-probe next cycle
	w.probedSinceExpiry[modelID] = cooldownExpiry
}

// findConnectorForModel finds the first active connector from the model's access_via list.
func (w *CooldownWatcher) findConnectorForModel(accessVia []string) string {
	if w.cfg == nil || w.cfg.Connectors == nil {
		return ""
	}
	for _, connID := range accessVia {
		conn := w.cfg.GetConnector(connID)
		if conn != nil && conn.Status == "active" && conn.Type == "api" {
			return connID
		}
	}
	return ""
}
