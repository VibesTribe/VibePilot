package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

// ModelScannerConfig configures the model discovery scanner.
// Lives in system.json under "model_scanner" key.
type ModelScannerConfig struct {
	Enabled                bool          `json:"enabled"`
	ScanOnStartup          bool          `json:"scan_on_startup"`
	ScanIntervalMinutes    int           `json:"scan_interval_minutes"`
	HealthCheckIntervalMin int           `json:"health_check_interval_minutes"`
	RequestTimeoutSeconds  int           `json:"request_timeout_seconds"`
	Providers              []ScannerProvider `json:"providers"`
}

// ScannerProvider defines a provider endpoint to scan.
type ScannerProvider struct {
	Name        string `json:"name"`          // e.g. "openrouter", "groq", "gemini"
	Endpoint    string `json:"endpoint"`      // models list endpoint
	VaultKey    string `json:"vault_key"`     // vault key for API key (empty = no auth needed)
	APIStyle    string `json:"api_style"`     // "openai" or "google"
	FreeOnly    bool   `json:"free_only"`     // only report free models
}

// ModelDiscovery represents a model found by the scanner.
type ModelDiscovery struct {
	ModelID       string  `json:"model_id"`
	ModelName     string  `json:"model_name"`
	Provider      string  `json:"provider"`
	ContextWindow int     `json:"context_window"`
	PromptCost    float64 `json:"prompt_cost_per_token"`
	CompCost      float64 `json:"completion_cost_per_token"`
	IsFree        bool    `json:"is_free"`
	Modality      string  `json:"modality"`
	DiscoveredAt  time.Time `json:"discovered_at"`
	Status        string  `json:"status"` // "new", "known", "disappeared", "changed"
}

// ModelScanner discovers available models from provider APIs.
type ModelScanner struct {
	cfg     *ModelScannerConfig
	db      db.Database
	vault   VaultKeyGetter
	timeout time.Duration

	mu         sync.RWMutex
	discoveries map[string]*ModelDiscovery // keyed by provider:model_id
	lastScan   time.Time
}

// VaultKeyGetter abstracts vault key retrieval for testing.
type VaultKeyGetter interface {
	GetSecret(ctx context.Context, key string) (string, error)
}

// NewModelScanner creates a scanner from config.
func NewModelScanner(cfg *ModelScannerConfig, database db.Database, vault VaultKeyGetter) *ModelScanner {
	if cfg == nil {
		cfg = &ModelScannerConfig{Enabled: false}
	}
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &ModelScanner{
		cfg:        cfg,
		db:         database,
		vault:      vault,
		timeout:    timeout,
		discoveries: make(map[string]*ModelDiscovery),
	}
}

// ScanAll scans all configured providers for available models.
func (s *ModelScanner) ScanAll(ctx context.Context) ([]*ModelDiscovery, error) {
	if !s.cfg.Enabled {
		return nil, fmt.Errorf("model scanner disabled")
	}

	var allDiscoveries []*ModelDiscovery
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, provider := range s.cfg.Providers {
		wg.Add(1)
		go func(p ScannerProvider) {
			defer wg.Done()
			results, err := s.scanProvider(ctx, p)
			if err != nil {
				log.Printf("[ModelScanner] Error scanning %s: %v", p.Name, err)
				return
			}
			mu.Lock()
			allDiscoveries = append(allDiscoveries, results...)
			mu.Unlock()
		}(provider)
	}

	wg.Wait()

	// Compare against our known models
	s.compareWithConfig(ctx, allDiscoveries)

	s.mu.Lock()
	s.lastScan = time.Now()
	s.mu.Unlock()

	log.Printf("[ModelScanner] Scan complete: %d models discovered across %d providers",
		len(allDiscoveries), len(s.cfg.Providers))

	return allDiscoveries, nil
}

// scanProvider scans a single provider for available models.
func (s *ModelScanner) scanProvider(ctx context.Context, p ScannerProvider) ([]*ModelDiscovery, error) {
	var apiKey string
	if p.VaultKey != "" && s.vault != nil {
		key, err := s.vault.GetSecret(ctx, p.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("get vault key %s: %w", p.VaultKey, err)
		}
		apiKey = key
	}

	switch p.APIStyle {
	case "openai":
		return s.scanOpenAIStyle(ctx, p, apiKey)
	case "google":
		return s.scanGoogleStyle(ctx, p, apiKey)
	default:
		return nil, fmt.Errorf("unknown api_style: %s", p.APIStyle)
	}
}

// scanOpenAIStyle scans an OpenAI-compatible /v1/models endpoint.
// Works for: OpenRouter, Groq, Cerebras, SambaNova, Together, Fireworks, etc.
func (s *ModelScanner) scanOpenAIStyle(ctx context.Context, p ScannerProvider, apiKey string) ([]*ModelDiscovery, error) {
	endpoint := strings.TrimRight(p.Endpoint, "/")
	// If endpoint doesn't end with /models, append it
	if !strings.HasSuffix(endpoint, "/models") {
		endpoint += "/models"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: s.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	// Parse OpenAI-style response: {"data": [...]}
	var result struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name,omitempty"`
			Created  int64  `json:"created,omitempty"`
			OwnedBy  string `json:"owned_by,omitempty"`

			// OpenRouter-specific fields
			ContextLength json.Number `json:"context_length,omitempty"`
			Pricing       *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing,omitempty"`
			Architecture *struct {
				Modality string `json:"modality,omitempty"`
			} `json:"architecture,omitempty"`

			// Groq-specific: rate limits in headers, not body
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var discoveries []*ModelDiscovery
	for _, m := range result.Data {
		promptCost := 0.0
		compCost := 0.0
		if m.Pricing != nil {
			promptCost, _ = strconv.ParseFloat(m.Pricing.Prompt, 64)
			compCost, _ = strconv.ParseFloat(m.Pricing.Completion, 64)
		}

		isFree := promptCost == 0 && compCost == 0

		// If free_only filter is set, skip paid models
		if p.FreeOnly && !isFree {
			continue
		}

		ctxLen := 0
		if m.ContextLength.String() != "" {
			cl, err := m.ContextLength.Int64()
			if err == nil {
				ctxLen = int(cl)
			}
		}

		modality := "text"
		if m.Architecture != nil && m.Architecture.Modality != "" {
			// Simplify: "text->text" → "text", "text+image->text" → "text+image"
			parts := strings.SplitN(m.Architecture.Modality, "->", 2)
			modality = parts[0]
		}

		name := m.Name
		if name == "" {
			name = m.ID
		}

		discoveries = append(discoveries, &ModelDiscovery{
			ModelID:       m.ID,
			ModelName:     name,
			Provider:      p.Name,
			ContextWindow: ctxLen,
			PromptCost:    promptCost,
			CompCost:      compCost,
			IsFree:        isFree,
			Modality:      modality,
			DiscoveredAt:  time.Now(),
		})
	}

	return discoveries, nil
}

// scanGoogleStyle scans a Google Gemini /v1beta/models endpoint.
func (s *ModelScanner) scanGoogleStyle(ctx context.Context, p ScannerProvider, apiKey string) ([]*ModelDiscovery, error) {
	endpoint := strings.TrimRight(p.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/models") {
		endpoint += "/models"
	}
	// Google uses key= query param, not Bearer header
	if apiKey != "" {
		if strings.Contains(endpoint, "?") {
			endpoint += "&key=" + apiKey
		} else {
			endpoint += "?key=" + apiKey
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: s.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	// Google response: {"models": [...]}
	var result struct {
		Models []struct {
			Name           string `json:"name"`            // "models/gemini-2.5-flash"
			DisplayName    string `json:"displayName"`
			InputTokenLimit int   `json:"inputTokenLimit,omitempty"`
			OutputTokenLimit int  `json:"outputTokenLimit,omitempty"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var discoveries []*ModelDiscovery
	for _, m := range result.Models {
		// Google free tier models are always free (with rate limits)
		modelID := strings.TrimPrefix(m.Name, "models/")
		discoveries = append(discoveries, &ModelDiscovery{
			ModelID:       modelID,
			ModelName:     m.DisplayName,
			Provider:      p.Name,
			ContextWindow: m.InputTokenLimit + m.OutputTokenLimit,
			PromptCost:    0,
			CompCost:      0,
			IsFree:        true,
			Modality:      "text+image",
			DiscoveredAt:  time.Now(),
		})
	}

	return discoveries, nil
}

// compareWithConfig compares scan results against our known models.
func (s *ModelScanner) compareWithConfig(ctx context.Context, discoveries []*ModelDiscovery) {
	// Build a set of discovered model IDs
	found := make(map[string]*ModelDiscovery)
	for _, d := range discoveries {
		key := d.Provider + ":" + d.ModelID
		found[key] = d
	}

	// Check each discovery against our known state
	for _, d := range discoveries {
		key := d.Provider + ":" + d.ModelID
		existing, known := s.discoveries[key]

		switch {
		case !known:
			d.Status = "new"
			log.Printf("[ModelScanner] NEW model: %s (%s) free=%v context=%d",
				d.ModelID, d.Provider, d.IsFree, d.ContextWindow)
		case existing.IsFree != d.IsFree:
			d.Status = "changed"
			log.Printf("[ModelScanner] CHANGED model: %s (%s) free: %v→%v",
				d.ModelID, d.Provider, existing.IsFree, d.IsFree)
		default:
			d.Status = "known"
		}
	}

	// Check for disappeared models (we had them, scanner doesn't find them anymore)
	for key, existing := range s.discoveries {
		if _, stillExists := found[key]; !stillExists {
			existing.Status = "disappeared"
			log.Printf("[ModelScanner] DISAPPEARED model: %s (%s)",
				existing.ModelID, existing.Provider)
		}
	}

	// Update our state
	s.mu.Lock()
	for _, d := range discoveries {
		key := d.Provider + ":" + d.ModelID
		s.discoveries[key] = d
	}
	s.mu.Unlock()
}

// GetDiscoveries returns all current discoveries.
func (s *ModelScanner) GetDiscoveries() []*ModelDiscovery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ModelDiscovery, 0, len(s.discoveries))
	for _, d := range s.discoveries {
		result = append(result, d)
	}
	return result
}

// GetNewModels returns only models with status "new".
func (s *ModelScanner) GetNewModels() []*ModelDiscovery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ModelDiscovery
	for _, d := range s.discoveries {
		if d.Status == "new" {
			result = append(result, d)
		}
	}
	return result
}

// GetDisappearedModels returns only models with status "disappeared".
func (s *ModelScanner) GetDisappearedModels() []*ModelDiscovery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ModelDiscovery
	for _, d := range s.discoveries {
		if d.Status == "disappeared" {
			result = append(result, d)
		}
	}
	return result
}

// GetChangedModels returns only models with status "changed".
func (s *ModelScanner) GetChangedModels() []*ModelDiscovery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ModelDiscovery
	for _, d := range s.discoveries {
		if d.Status == "changed" {
			result = append(result, d)
		}
	}
	return result
}

// LastScan returns when the last scan completed.
func (s *ModelScanner) LastScan() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastScan
}

// StartBackgroundScanner runs periodic scans in a goroutine.
func (s *ModelScanner) StartBackgroundScanner(ctx context.Context) {
	if !s.cfg.Enabled || s.cfg.ScanIntervalMinutes <= 0 {
		return
	}

	interval := time.Duration(s.cfg.ScanIntervalMinutes) * time.Minute
	log.Printf("[ModelScanner] Starting background scanner, interval=%v", interval)

	go func() {
		// Initial startup scan
		if s.cfg.ScanOnStartup {
			if _, err := s.ScanAll(ctx); err != nil {
				log.Printf("[ModelScanner] Startup scan failed: %v", err)
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("[ModelScanner] Stopping background scanner")
				return
			case <-ticker.C:
				scanCtx, cancel := context.WithTimeout(context.Background(), s.timeout*time.Duration(len(s.cfg.Providers)))
				if _, err := s.ScanAll(scanCtx); err != nil {
					log.Printf("[ModelScanner] Periodic scan failed: %v", err)
				}
				cancel()
			}
		}
	}()
}

// PersistDiscoveries saves notable discoveries to the database.
func (s *ModelScanner) PersistDiscoveries(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, d := range s.discoveries {
		if d.Status == "known" {
			continue
		}

		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"model_name":     d.ModelName,
			"context_window": d.ContextWindow,
			"is_free":        d.IsFree,
			"modality":       d.Modality,
			"prompt_cost":    d.PromptCost,
			"comp_cost":      d.CompCost,
			"provider":       d.Provider,
			"discovered_at":  d.DiscoveredAt,
		})

		// Insert as research suggestion for human review
		params := map[string]interface{}{
			"p_title":        fmt.Sprintf("Model discovery: %s (%s)", d.ModelID, d.Provider),
			"p_type":         "new_model",
			"p_complexity":   "simple",
			"p_summary":      fmt.Sprintf("Scanner found %s model %s on %s. Status: %s.", map[bool]string{true: "free", false: "paid"}[d.IsFree], d.ModelID, d.Provider, d.Status),
			"p_details":      string(detailsJSON),
			"p_findings_path": "",
		}

		if s.db != nil {
			if _, err := s.db.RPC(ctx, "create_research_suggestion", params); err != nil {
				// Log but don't fail — persistence is best-effort
				log.Printf("[ModelScanner] Warning: failed to persist discovery %s: %v", d.ModelID, err)
			}
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
