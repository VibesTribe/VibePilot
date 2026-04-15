// Package memory implements the VibePilot 3-Layer Memory System.
//
// Layer 1 – Short-term:  per-agent-run session context, TTL-expired via CleanExpired.
// Layer 2 – Mid-term:    project-scoped key/value state.
// Layer 3 – Long-term:   learned rules with category, priority, and confidence.
//
// All table names are config-driven (no hardcoded strings in queries).
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

// Table configuration – override via Config if needed.
const (
	DefaultSessionsTable  = "memory_sessions"
	DefaultProjectTable   = "memory_project"
	DefaultRulesTable     = "memory_rules"
	DefaultSessionTTL     = 1 * time.Hour
)

// Config holds overrides for table names and session TTL.
type Config struct {
	SessionsTable string
	ProjectTable  string
	RulesTable    string
	SessionTTL    time.Duration
}

func (c Config) sessionsTable() string {
	if c.SessionsTable != "" {
		return c.SessionsTable
	}
	return DefaultSessionsTable
}

func (c Config) projectTable() string {
	if c.ProjectTable != "" {
		return c.ProjectTable
	}
	return DefaultProjectTable
}

func (c Config) rulesTable() string {
	if c.RulesTable != "" {
		return c.RulesTable
	}
	return DefaultRulesTable
}

func (c Config) sessionTTL() time.Duration {
	if c.SessionTTL > 0 {
		return c.SessionTTL
	}
	return DefaultSessionTTL
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// Rule represents a long-term learned rule.
type Rule struct {
	ID         int64   `json:"id"`
	Category   string  `json:"category"`
	RuleText   string  `json:"rule_text"`
	Source     string  `json:"source"`
	Priority   int     `json:"priority"`
	Confidence float32 `json:"confidence"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// MemoryService provides the 3-layer memory operations backed by Supabase.
type MemoryService struct {
	db     *db.DB
	config Config
}

// New creates a MemoryService using the existing Supabase db client.
func New(database *db.DB, cfg Config) *MemoryService {
	return &MemoryService{
		db:     database,
		config: cfg,
	}
}

// ---------------------------------------------------------------------------
// Layer 1: Short-term memory (session-scoped, TTL-based)
// ---------------------------------------------------------------------------

// StoreShortTerm persists session context. It upserts on session_id so the
// latest context always wins.
func (s *MemoryService) StoreShortTerm(ctx context.Context, sessionID, agentType string, contextData map[string]any) error {
	expiresAt := time.Now().UTC().Add(s.config.sessionTTL())

	// Try update first; if no rows affected, insert.
	updated, err := s.db.Update(ctx, s.config.sessionsTable(), sessionID, map[string]any{
		"session_id": sessionID,
		"agent_type": agentType,
		"context":    contextData,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("update session %s: %w", sessionID, err)
	}

	// Update returned at least one row – done.
	var rows []json.RawMessage
	if err := json.Unmarshal(updated, &rows); err == nil && len(rows) > 0 {
		return nil
	}

	// No row matched – insert fresh.
	_, err = s.db.Insert(ctx, s.config.sessionsTable(), map[string]any{
		"session_id": sessionID,
		"agent_type": agentType,
		"context":    contextData,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("insert session %s: %w", sessionID, err)
	}
	return nil
}

// GetShortTerm retrieves session context by sessionID.
func (s *MemoryService) GetShortTerm(ctx context.Context, sessionID string) (map[string]any, error) {
	data, err := s.db.Query(ctx, s.config.sessionsTable(), map[string]any{
		"session_id": sessionID,
		"limit":      "1",
	})
	if err != nil {
		return nil, fmt.Errorf("query session %s: %w", sessionID, err)
	}

	var results []struct {
		Context map[string]any `json:"context"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	if len(results) == 0 {
		return nil, nil // not found is not an error
	}
	return results[0].Context, nil
}

// ---------------------------------------------------------------------------
// Layer 2: Mid-term memory (project-scoped key/value)
// ---------------------------------------------------------------------------

// StoreProjectState upserts a key/value pair scoped to a project.
func (s *MemoryService) StoreProjectState(ctx context.Context, projectID, key string, value map[string]any) error {
	// Attempt update first (match on project_id + key via REST filter path).
	path := s.config.projectTable() + "?project_id=eq." + projectID + "&key=eq." + key
	updated, err := s.db.REST(ctx, "PATCH", path, map[string]any{
		"value":      value,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("update project state %s/%s: %w", projectID, key, err)
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(updated, &rows); err == nil && len(rows) > 0 {
		return nil
	}

	// No existing row – insert.
	_, err = s.db.Insert(ctx, s.config.projectTable(), map[string]any{
		"project_id": projectID,
		"key":        key,
		"value":      value,
	})
	if err != nil {
		return fmt.Errorf("insert project state %s/%s: %w", projectID, key, err)
	}
	return nil
}

// GetProjectState retrieves a value by projectID and key.
func (s *MemoryService) GetProjectState(ctx context.Context, projectID, key string) (map[string]any, error) {
	data, err := s.db.Query(ctx, s.config.projectTable(), map[string]any{
		"project_id": projectID,
		"key":        key,
		"limit":      "1",
	})
	if err != nil {
		return nil, fmt.Errorf("query project state %s/%s: %w", projectID, key, err)
	}

	var results []struct {
		Value map[string]any `json:"value"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("unmarshal project state: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0].Value, nil
}

// ---------------------------------------------------------------------------
// Layer 3: Long-term memory (learned rules)
// ---------------------------------------------------------------------------

// StoreRule inserts a new learned rule into long-term memory.
func (s *MemoryService) StoreRule(ctx context.Context, category, ruleText, source string, priority int) error {
	_, err := s.db.Insert(ctx, s.config.rulesTable(), map[string]any{
		"category":   category,
		"rule_text":  ruleText,
		"source":     source,
		"priority":   priority,
		"confidence": 0.5, // start at neutral; adjust via reinforcement later
	})
	if err != nil {
		return fmt.Errorf("insert rule [%s]: %w", category, err)
	}
	return nil
}

// GetRulesByCategory returns all rules for a given category, ordered by priority desc.
func (s *MemoryService) GetRulesByCategory(ctx context.Context, category string) ([]Rule, error) {
	data, err := s.db.Query(ctx, s.config.rulesTable(), map[string]any{
		"category": category,
		"order":    "priority.desc",
	})
	if err != nil {
		return nil, fmt.Errorf("query rules by category %s: %w", category, err)
	}

	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("unmarshal rules: %w", err)
	}
	return rules, nil
}

// GetRulesByPriority returns all rules at or above the given priority threshold,
// ordered by priority descending.
func (s *MemoryService) GetRulesByPriority(ctx context.Context, minPriority int) ([]Rule, error) {
	path := s.config.rulesTable() + "?priority=gte." + fmt.Sprintf("%d", minPriority) + "&order=priority.desc"
	data, err := s.db.REST(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("query rules by priority >= %d: %w", minPriority, err)
	}

	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("unmarshal rules: %w", err)
	}
	return rules, nil
}

// ---------------------------------------------------------------------------
// Maintenance
// ---------------------------------------------------------------------------

// CleanExpired removes all session memory rows past their expires_at timestamp.
// Call this periodically (e.g. every 5 minutes) from a background goroutine.
func (s *MemoryService) CleanExpired(ctx context.Context) error {
	cutoff := time.Now().UTC().Format(time.RFC3339)
	path := s.config.sessionsTable() + "?expires_at=lt." + cutoff
	_, err := s.db.REST(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("clean expired sessions: %w", err)
	}
	return nil
}
