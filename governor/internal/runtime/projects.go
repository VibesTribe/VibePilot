package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

// ProjectConfig holds all per-project configuration loaded from the projects
// table. Each project defines its own git repo, deploy target, model keys,
// branding, and connected services — enabling VibePilot to manage multiple
// isolated projects through a single governor instance.
//
// Key design principle: a ProjectConfig is a self-contained context capsule.
// When a task carries a project_id, the resolver loads the ProjectConfig and
// that config tells the agent everything it needs: which repo, which branch
// prefix, which model keys, where to deploy. No global mutable state.
type ProjectConfig struct {
	// Identity
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Status      string `json:"status"`

	// Git / Repository
	GitHubOwner      string   `json:"github_owner"`
	GitHubRepo       string   `json:"github_repo"`
	RepoPath         string   `json:"repo_path"`
	DefaultBranch    string   `json:"default_branch"`
	BranchPrefixTask string   `json:"branch_prefix_task"`
	ProtectedBranches []string `json:"protected_branches"`

	// Tech stack
	TechStack         string `json:"tech_stack"`
	BuildCommand      string `json:"build_command"`
	TestCommand       string `json:"test_command"`
	LintCommand       string `json:"lint_command"`
	TypecheckCommand  string `json:"typecheck_command"`
	DeployCommand     string `json:"deploy_command"`

	// Deploy
	DeployTarget string `json:"deploy_target"`
	DeployURL    string `json:"deploy_url"`

	// Model allocation (env var names this project may use)
	ModelKeys []string `json:"model_keys"`

	// Connected services (for honeycomb links panel)
	ConnectedServices []ConnectedService `json:"connected_services"`

	// Theme / Branding
	Theme ProjectTheme `json:"theme"`

	// Cumulative metrics
	TotalTasks            int     `json:"total_tasks"`
	CompletedTasks        int     `json:"completed_tasks"`
	TotalTokensUsed       int64   `json:"total_tokens_used"`
	TotalTheoreticalCost  float64 `json:"total_theoretical_cost"`
	TotalActualCost       float64 `json:"total_actual_cost"`
	TotalSavings          float64 `json:"total_savings"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConnectedService represents an external service link shown in the project's
// honeycomb cell (e.g., GitHub repo, Vercel dashboard, Cloudflare).
type ConnectedService struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// ProjectTheme holds branding colors for the dashboard.
type ProjectTheme struct {
	PrimaryColor string `json:"primary_color"`
	LogoURL      string `json:"logo_url,omitempty"`
}

// ProjectResolver loads and caches ProjectConfig records from the database.
// It follows the same pattern as MemoryService: takes a db.Database, provides
// typed access methods. Results are cached for 5 minutes to avoid hitting the
// database on every task dispatch.
type ProjectResolver struct {
	db    db.Database
	cache map[string]*cacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

type cacheEntry struct {
	project  *ProjectConfig
	cachedAt time.Time
}

// NewProjectResolver creates a ProjectResolver backed by the given database.
func NewProjectResolver(database db.Database) *ProjectResolver {
	return &ProjectResolver{
		db:    database,
		cache: make(map[string]*cacheEntry),
		ttl:   5 * time.Minute,
	}
}

// GetProjectBySlug loads a project configuration by its slug (e.g., "vibepilot",
// "sealed"). Returns the cached version if fresh, otherwise queries the DB.
func (r *ProjectResolver) GetProjectBySlug(ctx context.Context, slug string) (*ProjectConfig, error) {
	cacheKey := "slug:" + slug
	if cached := r.getCached(cacheKey); cached != nil {
		return cached, nil
	}

	raw, err := r.db.Query(ctx, "projects", map[string]any{"slug": slug})
	if err != nil {
		return nil, fmt.Errorf("query project by slug %q: %w", slug, err)
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse project rows: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("project not found: %s", slug)
	}

	project, err := parseProject(rows[0])
	if err != nil {
		return nil, fmt.Errorf("parse project %s: %w", slug, err)
	}

	r.setCached(cacheKey, project)
	return project, nil
}

// GetProjectByID loads a project configuration by its UUID.
func (r *ProjectResolver) GetProjectByID(ctx context.Context, id string) (*ProjectConfig, error) {
	cacheKey := "id:" + id
	if cached := r.getCached(cacheKey); cached != nil {
		return cached, nil
	}

	raw, err := r.db.Query(ctx, "projects", map[string]any{"id": id})
	if err != nil {
		return nil, fmt.Errorf("query project by id %q: %w", id, err)
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse project rows: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("project not found: %s", id)
	}

	project, err := parseProject(rows[0])
	if err != nil {
		return nil, fmt.Errorf("parse project %s: %w", id, err)
	}

	r.setCached(cacheKey, project)
	return project, nil
}

// GetAllProjects returns all active projects. Used by the honeycomb overview.
func (r *ProjectResolver) GetAllProjects(ctx context.Context) ([]*ProjectConfig, error) {
	raw, err := r.db.Query(ctx, "projects", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("query all projects: %w", err)
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse project rows: %w", err)
	}

	projects := make([]*ProjectConfig, 0, len(rows))
	for _, row := range rows {
		p, err := parseProject(row)
		if err != nil {
			continue // skip unparseable rows rather than failing entirely
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// InvalidateCache clears all cached project configs. Call after updating a
// project's configuration to force fresh reads on next access.
func (r *ProjectResolver) InvalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]*cacheEntry)
}

// --- internal helpers ---

func (r *ProjectResolver) getCached(key string) *ProjectConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.cache[key]
	if !ok {
		return nil
	}
	if time.Since(entry.cachedAt) > r.ttl {
		return nil
	}
	return entry.project
}

func (r *ProjectResolver) setCached(key string, project *ProjectConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = &cacheEntry{
		project:  project,
		cachedAt: time.Now(),
	}
}

// parseProject converts a JSON row from the projects table into a ProjectConfig.
// Handles the JSONB columns (model_keys, connected_services, theme) which the
// db.Query method returns as raw JSON.
func parseProject(raw json.RawMessage) (*ProjectConfig, error) {
	// Use an intermediate map to handle the JSONB fields separately.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal project row: %w", err)
	}

	p := &ProjectConfig{}

	// Scalar fields — use a helper to safely extract
	p.ID = jsonStr(m, "id")
	p.Slug = jsonStr(m, "slug")
	p.DisplayName = jsonStr(m, "display_name")
	p.Description = jsonStr(m, "description")
	p.Status = jsonStr(m, "status")
	p.GitHubOwner = jsonStr(m, "github_owner")
	p.GitHubRepo = jsonStr(m, "github_repo")
	p.RepoPath = jsonStr(m, "repo_path")
	p.DefaultBranch = jsonStr(m, "default_branch")
	p.BranchPrefixTask = jsonStr(m, "branch_prefix_task")
	p.TechStack = jsonStr(m, "tech_stack")
	p.BuildCommand = jsonStr(m, "build_command")
	p.TestCommand = jsonStr(m, "test_command")
	p.LintCommand = jsonStr(m, "lint_command")
	p.TypecheckCommand = jsonStr(m, "typecheck_command")
	p.DeployCommand = jsonStr(m, "deploy_command")
	p.DeployTarget = jsonStr(m, "deploy_target")
	p.DeployURL = jsonStr(m, "deploy_url")

	// Numeric fields
	p.TotalTasks = jsonInt(m, "total_tasks")
	p.CompletedTasks = jsonInt(m, "completed_tasks")
	p.TotalTokensUsed = jsonInt64(m, "total_tokens_used")
	p.TotalTheoreticalCost = jsonFloat(m, "total_theoretical_cost")
	p.TotalActualCost = jsonFloat(m, "total_actual_cost")
	p.TotalSavings = jsonFloat(m, "total_savings")

	// Protected branches (TEXT[])
	p.ProtectedBranches = jsonStrSlice(m, "protected_branches")

	// JSONB fields
	p.ModelKeys = jsonStrSliceFromJSONB(m, "model_keys")
	p.Theme = parseTheme(m["theme"])

	if cs, err := parseConnectedServices(m["connected_services"]); err == nil {
		p.ConnectedServices = cs
	}

	return p, nil
}

// jsonStr safely extracts a string field from a raw JSON map, handling both
// quoted strings and null values.
func jsonStr(m map[string]json.RawMessage, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return ""
}

// jsonInt safely extracts an int field.
func jsonInt(m map[string]json.RawMessage, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	// pgx may return float64 for JSON numbers
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return int(f)
	}
	return 0
}

// jsonInt64 safely extracts an int64 field.
func jsonInt64(m map[string]json.RawMessage, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return int64(f)
	}
	return 0
}

// jsonFloat safely extracts a float64 field.
func jsonFloat(m map[string]json.RawMessage, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return f
	}
	return 0
}

// jsonStrSlice extracts a TEXT[] field from a raw JSON map.
// pgx returns PostgreSQL arrays as JSON arrays like ["main","master"].
func jsonStrSlice(m map[string]json.RawMessage, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	var s []string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return nil
}

// jsonStrSliceFromJSONB extracts a string array stored in a JSONB column.
func jsonStrSliceFromJSONB(m map[string]json.RawMessage, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	var s []string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return nil
}

// parseTheme extracts theme information from the JSONB theme column.
func parseTheme(raw json.RawMessage) ProjectTheme {
	t := ProjectTheme{}
	if len(raw) == 0 {
		return t
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) == nil {
		t.PrimaryColor = m["primary_color"]
		t.LogoURL = m["logo_url"]
	}
	return t
}

// parseConnectedServices extracts the connected services array from JSONB.
func parseConnectedServices(raw json.RawMessage) ([]ConnectedService, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var services []ConnectedService
	if err := json.Unmarshal(raw, &services); err != nil {
		return nil, err
	}
	return services, nil
}
