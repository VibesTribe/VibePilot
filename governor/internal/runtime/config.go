package runtime
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type SystemConfig struct {
	Database    DatabaseConfig         `json:"database"`
	Vault       VaultConfig            `json:"vault"`
	Git         GitConfig              `json:"git"`
	DB          *DBConfig              `json:"db,omitempty"`
	HTTP        *HTTPConfig            `json:"http,omitempty"`
	Execution   *ExecutionConfig       `json:"execution,omitempty"`
	Session     *SessionConfig         `json:"session,omitempty"`
	Courier     *CourierConfig         `json:"courier,omitempty"`
	Runtime     RuntimeConfig          `json:"runtime"`
	Concurrency ConcurrencyConfig      `json:"concurrency"`
	Security    SecurityConfig         `json:"security"`
	Events      EventsConfig           `json:"events"`
	Sandbox     SandboxConfig          `json:"sandbox"`
	WebTools    WebToolsConfig         `json:"web_tools"`
	Recovery    map[string]interface{} `json:"recovery"`
	Defaults    map[string]interface{} `json:"defaults"`
	Validation  ValidationConfig       `json:"validation"`
	Logging     LoggingConfig          `json:"logging"`
	Core        *CoreConfig            `json:"core,omitempty"`
	Webhooks    *WebhooksConfig        `json:"webhooks,omitempty"`
	PromptsDir  string                 `json:"prompts_dir"`
	MCPServers  []MCPServerConfig      `json:"mcp_servers,omitempty"`
	GovernorMCP *GovernorMCPConfig     `json:"governor_mcp,omitempty"`
	Worktrees   *WorktreeConfig        `json:"worktrees,omitempty"`
	CodeMap     *CodeMapConfig         `json:"code_map,omitempty"`
}

// CodeMapConfig configures the code map (jcodemunch) integration.
type CodeMapConfig struct {
	Path              string `json:"path"`                          // relative to repo root, default ".context/map.md"
	CacheTTLMins      int    `json:"cache_ttl_mins"`                // cache TTL in minutes, default 60
	RefreshOnStartup  bool   `json:"refresh_on_startup"`            // run jcodemunch on boot, default true
}

// DefaultCodeMapConfig returns sensible defaults for code map configuration.
func DefaultCodeMapConfig() *CodeMapConfig {
	return &CodeMapConfig{
		Path:             ".context/map.md",
		CacheTTLMins:     60,
		RefreshOnStartup: true,
	}
}

// GovernorMCPConfig configures the MCP server that exposes governor tools to external agents.
type GovernorMCPConfig struct {
	Enabled   bool   `json:"enabled"`
	Transport string `json:"transport"` // "stdio" | "sse"
	Port      int    `json:"port"`      // for SSE mode (default 8081)
}

// WorktreeConfig configures git worktrees for parallel agent execution.
type WorktreeConfig struct {
	Enabled  bool   `json:"enabled"`
	BasePath string `json:"base_path"` // e.g. /home/vibes/VibePilot-work
}

// MCPServerConfig defines an approved MCP server connection.
// Only servers explicitly listed here are connected.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio" | "http" | "sse"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Enabled   bool              `json:"enabled"`
}

type WebhooksConfig struct {
	Enabled        bool   `json:"enabled"`
	Port           int    `json:"port"`
	Path           string `json:"path"`
	SecretVaultKey string `json:"secret_vault_key"`
}

type CoreConfig struct {
	CheckpointEnabled         bool `json:"checkpoint_enabled"`
	CheckpointIntervalPercent int  `json:"checkpoint_interval_percent"`
	StateSyncIntervalSeconds  int  `json:"state_sync_interval_seconds"`
	RecoveryEnabled           bool `json:"recovery_enabled"`
}

func (c *CoreConfig) GetCheckpointIntervalPercent() int {
	if c == nil || c.CheckpointIntervalPercent <= 0 {
		return 25
	}
	return c.CheckpointIntervalPercent
}

func (c *CoreConfig) GetStateSyncIntervalSeconds() int {
	if c == nil || c.StateSyncIntervalSeconds <= 0 {
		return 60
	}
	return c.StateSyncIntervalSeconds
}

func (c *CoreConfig) IsCheckpointEnabled() bool {
	return c != nil && c.CheckpointEnabled
}

func (c *CoreConfig) IsRecoveryEnabled() bool {
	return c == nil || c.RecoveryEnabled
}

type ValidationConfig struct {
	MinTaskConfidence     float64 `json:"min_task_confidence"`
	RequirePromptPacket   bool    `json:"require_prompt_packet"`
	RequireCategory       bool    `json:"require_category"`
	RequireExpectedOutput bool    `json:"require_expected_output"`
	DefaultMaxAttempts    int     `json:"default_max_attempts"`
}

// DatabaseConfig specifies how to connect to Supabase.
// KeyEnv MUST be SUPABASE_SERVICE_KEY (not anon key) because:
//   - Vault table requires service role to bypass RLS
//   - Anon key cannot read/write secrets_vault
//
// See governor/internal/vault/vault.go for full architecture docs.
type DatabaseConfig struct {
	Type           string `json:"type"`
	URLEnv         string `json:"url_env"`
	KeyEnv         string `json:"key_env"`
	PostgresURLEnv string `json:"postgres_url_env"`
}

type VaultConfig struct {
	KeyEnv          string `json:"key_env"`
	Table           string `json:"table"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
}

// DBConfig configures database HTTP timeouts and error truncation.
type DBConfig struct {
	HTTPTimeoutSeconds int `json:"http_timeout_seconds"`
	ErrorTruncateLength int `json:"error_truncate_length"`
}

// HTTPConfig configures HTTP client timeouts.
type HTTPConfig struct {
	ClientTimeoutSeconds   int `json:"client_timeout_seconds"`
	RequestTimeoutSeconds  int `json:"request_timeout_seconds"`
	ResponseTimeoutSeconds int `json:"response_timeout_seconds"`
}

// ExecutionConfig configures default execution timeouts.
type ExecutionConfig struct {
	DefaultTimeoutSeconds int `json:"default_timeout_seconds"`
}

// SessionConfig configures session-level timeouts.
type SessionConfig struct {
	DefaultTimeoutSeconds int `json:"default_timeout_seconds"`
}

// CourierConfig configures courier agent timeouts.
type CourierConfig struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

type GitConfig struct {
	Host               string              `json:"host"`
	RepoEnv            string              `json:"repo_env"`
	RepoPath           string              `json:"repo_path"`
	TokenEnv           string              `json:"token_env"`
	ProtectedBranches  []string            `json:"protected_branches"`
	DefaultTimeoutSecs int                 `json:"default_timeout_seconds"`
	DefaultMergeTarget string              `json:"default_merge_target"`
	BranchNamePattern  string              `json:"branch_name_pattern"`
	RemoteName         string              `json:"remote_name"`
	BranchPrefixes     *BranchPrefixConfig `json:"branch_prefixes"`
}

type BranchPrefixConfig struct {
	Task   string `json:"task"`
	Module string `json:"module"`
}

type LoggingConfig struct {
	MaxOutputLength int `json:"max_output_length"`
	MaxIDDisplay    int `json:"max_id_display"`
}

type RuntimeConfig struct {
	MaxConcurrentPerModule int `json:"max_concurrent_per_module"`
	MaxConcurrentTotal     int `json:"max_concurrent_total"`
	EventPollIntervalMs    int `json:"event_poll_interval_ms"`
	AgentTimeoutSeconds    int `json:"agent_timeout_seconds"`
	MaxToolTurns           int `json:"max_tool_turns"`
	EventQueryLimit        int `json:"event_query_limit"`
}

type ConcurrencyConfig struct {
	Limits       map[string]int `json:"limits"`
	DefaultLimit int            `json:"default_limit"`
}

func (c *ConcurrencyConfig) GetLimit(destination string) int {
	if limit, ok := c.Limits[destination]; ok {
		return limit
	}
	if c.DefaultLimit > 0 {
		return c.DefaultLimit
	}
	return 3
}

type SecurityConfig struct {
	LeakDetectionEnabled bool     `json:"leak_detection_enabled"`
	HTTPAllowlist        []string `json:"http_allowlist"`
}

type EventsConfig struct {
	TaskStatusesAvailable    []string `json:"task_statuses_available"`
	TaskStatusesReview       []string `json:"task_statuses_review"`
	TaskStatusesCompleted    []string `json:"task_statuses_completed"`
	PlanStatusesDraft        []string `json:"plan_statuses_draft"`
	PlanStatusesReview       []string `json:"plan_statuses_review"`
	PlanStatusesCouncil      []string `json:"plan_statuses_council"`
	PlanStatusesPendingHuman []string `json:"plan_statuses_pending_human"`
	PlanStatusesApproved     []string `json:"plan_statuses_approved"`
	MaintenanceStatus        string   `json:"maintenance_status"`
	TestResultsStatus        string   `json:"test_results_status"`
}

type SandboxConfig struct {
	DefaultTestCommand      string `json:"default_test_command"`
	TimeoutSeconds          int    `json:"timeout_seconds"`
	TempDir                 string `json:"temp_dir"`
	LintTimeoutSeconds      int    `json:"lint_timeout_seconds"`
	TypecheckTimeoutSeconds int    `json:"typecheck_timeout_seconds"`
}

type WebToolsConfig struct {
	SearchURL        string `json:"search_url"`
	UserAgent        string `json:"user_agent"`
	MaxFetchLength   int    `json:"max_fetch_length"`
	MaxRelatedTopics int    `json:"max_related_topics"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
}

type AgentConfig struct {
	ID               string   `json:"id"`
	Name             string   `json:"name,omitempty"`
	Prompt           string   `json:"prompt"`
	Capabilities     []string `json:"capabilities,omitempty"`
	Model            string   `json:"model,omitempty"`
	DefaultConnector string   `json:"default_connector"`
	Description      string   `json:"description,omitempty"`
	ContextPolicy    string   `json:"context_policy,omitempty"`
}

func (a *AgentConfig) HasCapability(capability string) bool {
	for _, c := range a.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

type AgentsFile struct {
	Version string        `json:"version"`
	Agents  []AgentConfig `json:"agents"`
}

func (a *AgentsFile) GetAgent(id string) *AgentConfig {
	for i := range a.Agents {
		if a.Agents[i].ID == id {
			return &a.Agents[i]
		}
	}
	return nil
}

type ToolParam struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type ToolConfig struct {
	Description    string               `json:"description"`
	Parameters     map[string]ToolParam `json:"parameters"`
	SecurityLevel  string               `json:"security_level"`
	Implementation string               `json:"implementation"`
}

type ToolsFile struct {
	Tools map[string]ToolConfig `json:"tools"`
}

// PlatformLimitSchema describes free-tier limits for web platform destinations
// (courier destinations like chatgpt.com, claude.ai, etc.). Only fields with
// known/documented values are set; nil means no limit or unknown.
type PlatformLimitSchema struct {
	MessagesPer3h      *int `json:"messages_per_3h,omitempty"`
	MessagesPer8h      *int `json:"messages_per_8h,omitempty"`
	MessagesPerDay     *int `json:"messages_per_day,omitempty"`
	MessagesPerSession *int `json:"messages_per_session,omitempty"`
	TokensPerDay       *int `json:"tokens_per_day,omitempty"`
	SessionsPerDay     *int `json:"sessions_per_day,omitempty"`
}

type ConnectorConfig struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	Status         string                 `json:"status"`
	Provider       string                 `json:"provider,omitempty"`
	Command        string                 `json:"command,omitempty"`
	CLIArgs        []string               `json:"cli_args,omitempty"`
	Endpoint       string                 `json:"endpoint,omitempty"`
	APIKeyRef      string                 `json:"api_key_ref,omitempty"`
	Models         []string               `json:"models_available,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	SharedLimits   RateLimits             `json:"shared_limits,omitempty"`
	LimitSchema    PlatformLimitSchema    `json:"limit_schema,omitempty"`
	Extra          map[string]interface{} `json:"-"`
}

type ConnectorsFile struct {
	Connectors []ConnectorConfig `json:"destinations"`
}

type ModelConfig struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	ContextLimit int      `json:"context_limit"`
	Capabilities []string `json:"capabilities"`
	AccessVia    []string `json:"access_via"`
	APIKeyRef    string   `json:"api_key_ref,omitempty"`
	Status       string   `json:"status"`
}

type ModelsFile struct {
	Models []ModelConfig `json:"models"`
}

type RoutingStrategy struct {
	Description string   `json:"description"`
	Priority    []string `json:"priority"`
}

type RoutingConfig struct {
	Version               string                     `json:"version,omitempty"`
	Description           string                     `json:"description,omitempty"`
	DefaultStrategy       string                     `json:"default_strategy"`
	Strategies            map[string]RoutingStrategy `json:"strategies"`
	AgentRestrictions     map[string][]string        `json:"agent_restrictions"`
	DestinationCategories map[string]map[string]any  `json:"destination_categories"`
	SelectionCriteria     map[string]any             `json:"selection_criteria"`
	Fallback              map[string]string          `json:"fallback"`
}

type PlanLifecycleConfig struct {
	Version         string                 `json:"version"`
	States          map[string]StateConfig `json:"states"`
	RevisionRules   RevisionRulesConfig    `json:"revision_rules"`
	ComplexityRules ComplexityRulesConfig  `json:"complexity_rules"`
	ConsensusRules  ConsensusRulesConfig   `json:"consensus_rules"`
	CouncilRules    CouncilRulesConfig     `json:"council_rules"`
}

type StateConfig struct {
	Description  string   `json:"description"`
	Transitions  []string `json:"transitions"`
	OnEnterEvent string   `json:"on_enter_event"`
	Final        bool     `json:"final"`
}

type RevisionRulesConfig struct {
	MaxRounds   int    `json:"max_rounds"`
	OnMaxRounds string `json:"on_max_rounds"`
	Description string `json:"description"`
}

type ComplexityRulesConfig struct {
	Simple  ComplexityCondition `json:"simple"`
	Complex ComplexityCondition `json:"complex"`
	Default string              `json:"default"`
}

type ComplexityCondition struct {
	Description string         `json:"description"`
	Conditions  map[string]any `json:"conditions"`
}

type ConsensusRulesConfig struct {
	Method      string                           `json:"method"`
	Description string                           `json:"description"`
	Methods     map[string]ConsensusMethodConfig `json:"methods"`
	BlockedOn   string                           `json:"blocked_on"`
	RevisionOn  string                           `json:"revision_on"`
}

type ConsensusMethodConfig struct {
	Approved       string             `json:"approved"`
	Blocked        string             `json:"blocked"`
	RevisionNeeded string             `json:"revision_needed"`
	Description    string             `json:"description"`
	Weights        map[string]float64 `json:"weights"`
}

type CouncilRulesConfig struct {
	MemberCount int                   `json:"member_count"`
	Lenses      []string              `json:"lenses"`
	Description string                `json:"description"`
	Strategy    CouncilStrategyConfig `json:"strategy"`
	Context     CouncilContextConfig  `json:"context"`
}

type CouncilStrategyConfig struct {
	Preferred   string `json:"preferred"`
	Fallback    string `json:"fallback"`
	Description string `json:"description"`
}

type CouncilContextConfig struct {
	IncludePRD  bool   `json:"include_prd"`
	Description string `json:"description"`
}

type Config struct {
	System        *SystemConfig
	Agents        *AgentsFile
	Tools         *ToolsFile
	Connectors    *ConnectorsFile
	Models        *ModelsFile
	Routing       *RoutingConfig
	PlanLifecycle *PlanLifecycleConfig

	systemPath        string
	agentsPath        string
	toolsPath         string
	connectorsPath    string
	modelsPath        string
	routingPath       string
	planLifecyclePath string
	promptsDir        string
	db                PromptLoader

	mu sync.RWMutex
}

type PromptLoader interface {
	REST(ctx context.Context, method, path string, body interface{}) ([]byte, error)
	RESTWithHeaders(ctx context.Context, method, path string, body interface{}, headers map[string]string) ([]byte, error)
}

func (c *Config) SetDatabase(db PromptLoader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = db
}

func (c *Config) SyncPromptsToDB() error {
	c.mu.RLock()
	db := c.db
	promptsDir := c.promptsDir
	if c.System != nil && c.System.PromptsDir != "" {
		promptsDir = c.System.PromptsDir
	}
	c.mu.RUnlock()

	if db == nil {
		return nil
	}

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return fmt.Errorf("read prompts dir %s: %w", promptsDir, err)
	}

	ctx := context.Background()
	synced := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		fullPath := filepath.Join(promptsDir, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		body := map[string]interface{}{
			"name":    entry.Name(),
			"content": string(content),
		}

		headers := map[string]string{
			"Prefer": "resolution=merge-duplicates,return=representation",
		}
		_, err = db.RESTWithHeaders(ctx, "POST", "prompts?on_conflict=name", body, headers)
		if err != nil {
			log.Printf("Warning: failed to sync prompt %s to DB: %v", entry.Name(), err)
			continue
		}
		synced++
	}

	if synced > 0 {
		log.Printf("Synced %d prompts from %s to Supabase", synced, promptsDir)
	}
	return nil
}

func LoadConfig(configDir string) (*Config, error) {
	cfg := &Config{
		systemPath:        filepath.Join(configDir, "system.json"),
		agentsPath:        filepath.Join(configDir, "agents.json"),
		toolsPath:         filepath.Join(configDir, "tools.json"),
		connectorsPath:    filepath.Join(configDir, "connectors.json"),
		modelsPath:        filepath.Join(configDir, "models.json"),
		routingPath:       filepath.Join(configDir, "routing.json"),
		planLifecyclePath: filepath.Join(configDir, "plan_lifecycle.json"),
		promptsDir:        filepath.Join(configDir, "prompts"),
	}

	if err := cfg.Reload(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if sys, err := loadJSON[SystemConfig](c.systemPath); err == nil {
		c.System = sys
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load system.json: %w", err)
	}

	if agents, err := loadJSON[AgentsFile](c.agentsPath); err == nil {
		c.Agents = agents
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load agents.json: %w", err)
	}

	if tools, err := loadJSON[ToolsFile](c.toolsPath); err == nil {
		c.Tools = tools
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load tools.json: %w", err)
	}

	if conns, err := loadJSON[ConnectorsFile](c.connectorsPath); err == nil {
		c.Connectors = conns
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load connectors.json: %w", err)
	}

	if models, err := loadJSON[ModelsFile](c.modelsPath); err == nil {
		c.Models = models
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load models.json: %w", err)
	}

	if routing, err := loadJSON[RoutingConfig](c.routingPath); err == nil {
		c.Routing = routing
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load routing.json: %w", err)
	}

	if planLifecycle, err := loadJSON[PlanLifecycleConfig](c.planLifecyclePath); err == nil {
		c.PlanLifecycle = planLifecycle
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load plan_lifecycle.json: %w", err)
	}

	if c.System == nil {
		c.System = &SystemConfig{
			Database: DatabaseConfig{
				Type:   "supabase",
				URLEnv: "SUPABASE_URL",
				KeyEnv: "SUPABASE_SERVICE_KEY",
			},
			Vault: VaultConfig{
				KeyEnv:          "VAULT_KEY",
				Table:           "secrets_vault",
				CacheTTLSeconds: 300,
			},
			Git: GitConfig{
				Host:              "github",
				ProtectedBranches: []string{"main", "master"},
			},
			Runtime: RuntimeConfig{
				MaxConcurrentPerModule: 8,
				MaxConcurrentTotal:     160,
				EventPollIntervalMs:    1000,
				AgentTimeoutSeconds:    300,
				MaxToolTurns:           10,
			},
			Events: EventsConfig{
				TaskStatusesAvailable: []string{"available"},
				TaskStatusesReview:    []string{"review"},
				TaskStatusesCompleted: []string{"testing", "merged"},
				PlanStatusesCouncil:   []string{"council_review"},
				PlanStatusesApproved:  []string{"approved"},
				MaintenanceStatus:     "pending",
			},
			Sandbox: SandboxConfig{
				DefaultTestCommand: "npm test",
				TimeoutSeconds:     60,
			},
		}
	}

	if c.PlanLifecycle == nil {
		c.PlanLifecycle = &PlanLifecycleConfig{
			Version: "1.0",
			RevisionRules: RevisionRulesConfig{
				MaxRounds:   6,
				OnMaxRounds: "pending_human",
			},
			ConsensusRules: ConsensusRulesConfig{
				Method:     "unanimous_approval",
				BlockedOn:  "any_blocked",
				RevisionOn: "any_revision_needed",
			},
			CouncilRules: CouncilRulesConfig{
				MemberCount: 3,
				Lenses:      []string{"user_alignment", "architecture", "feasibility"},
				Strategy: CouncilStrategyConfig{
					Preferred: "parallel_different_models",
					Fallback:  "sequential_same_model_different_hats",
				},
				Context: CouncilContextConfig{
					IncludePRD: true,
				},
			},
		}
	}

	return c.validate()
}

func (c *Config) validate() error {
	if c.System == nil {
		return fmt.Errorf("system config required")
	}
	if c.System.Database.URLEnv == "" {
		return fmt.Errorf("database.url_env required")
	}
	if c.System.Database.KeyEnv == "" {
		return fmt.Errorf("database.key_env required")
	}
	return nil
}

func (c *Config) GetAgent(id string) *AgentConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Agents == nil {
		return nil
	}
	return c.Agents.GetAgent(id)
}

func (c *Config) GetTool(name string) *ToolConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Tools == nil {
		return nil
	}
	tool, ok := c.Tools.Tools[name]
	if !ok {
		return nil
	}
	return &tool
}

func (c *Config) GetConnector(id string) *ConnectorConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Connectors == nil {
		return nil
	}
	for i := range c.Connectors.Connectors {
		if c.Connectors.Connectors[i].ID == id {
			return &c.Connectors.Connectors[i]
		}
	}
	return nil
}

func (c *Config) GetModel(id string) *ModelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Models == nil {
		return nil
	}
	for i := range c.Models.Models {
		if c.Models.Models[i].ID == id {
			return &c.Models.Models[i]
		}
	}
	return nil
}

func (c *Config) LoadPrompt(promptPath string) (string, error) {
	promptName := filepath.Base(promptPath)

	c.mu.RLock()
	db := c.db
	c.mu.RUnlock()

	if db != nil {
		ctx := context.Background()
		data, err := db.REST(ctx, "GET", fmt.Sprintf("prompts?name=eq.%s&select=content", promptName), nil)
		if err == nil {
			var results []struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(data, &results); err == nil && len(results) > 0 {
				return results[0].Content, nil
			}
		}
	}

	c.mu.RLock()
	promptsDir := c.promptsDir
	if c.System != nil && c.System.PromptsDir != "" {
		promptsDir = c.System.PromptsDir
	}
	fullPath := filepath.Join(promptsDir, promptName)
	c.mu.RUnlock()

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", fullPath, err)
	}
	return string(data), nil
}

func (c *Config) AgentHasCapability(agentID, capability string) bool {
	agent := c.GetAgent(agentID)
	if agent == nil {
		return false
	}
	return agent.HasCapability(capability)
}

func (c *Config) GetDatabaseURL() string {
	return os.Getenv(c.System.Database.URLEnv)
}

func (c *Config) GetDatabaseKey() string {
	return os.Getenv(c.System.Database.KeyEnv)
}

func (c *Config) GetDatabaseType() string {
	return c.System.Database.Type
}

func (c *Config) GetPostgresURL() string {
	envKey := c.System.Database.PostgresURLEnv
	if envKey == "" {
		envKey = "DATABASE_URL"
	}
	return os.Getenv(envKey)
}

// GetRealtimeURL returns the Supabase Realtime WebSocket URL.
// It constructs this from the database URL by replacing the protocol.
// Example: https://xyz.supabase.co -> wss://xyz.supabase.co/realtime/v1/websocket
func (c *Config) GetRealtimeURL() string {
	dbURL := c.GetDatabaseURL()
	if dbURL == "" {
		return ""
	}

	// Replace https:// with wss:// and add realtime path
	// https://qtpdzsinvifkgpxyxlaz.supabase.co -> wss://qtpdzsinvifkgpxyxlaz.supabase.co/realtime/v1/websocket
	realtimeURL := dbURL
	if len(dbURL) > 8 && dbURL[:8] == "https://" {
		realtimeURL = "wss://" + dbURL[8:] + "/realtime/v1/websocket"
	} else if len(dbURL) > 7 && dbURL[:7] == "http://" {
		realtimeURL = "ws://" + dbURL[7:] + "/realtime/v1/websocket"
	}

	return realtimeURL
}

func (c *Config) GetVaultKey() string {
	return os.Getenv(c.System.Vault.KeyEnv)
}

// GetVaultKeyEnv returns the env var name for the vault key (e.g. "VAULT_KEY").
func (c *Config) GetVaultKeyEnv() string {
	if c.System.Vault.KeyEnv == "" {
		return "VAULT_KEY"
	}
	return c.System.Vault.KeyEnv
}

func (c *Config) GetProtectedBranches() []string {
	if c.System == nil || c.System.Git.ProtectedBranches == nil {
		return []string{"main", "master"}
	}
	return c.System.Git.ProtectedBranches
}

func (c *Config) GetRepoPath() string {
	if c.System == nil || c.System.Git.RepoPath == "" {
		return "."
	}
	return c.System.Git.RepoPath
}

func (c *Config) GetGitTimeout() int {
	if c.System == nil || c.System.Git.DefaultTimeoutSecs == 0 {
		return 60
	}
	return c.System.Git.DefaultTimeoutSecs
}

func (c *Config) GetDefaultMergeTarget() string {
	if c.System == nil || c.System.Git.DefaultMergeTarget == "" {
		return "main"
	}
	return c.System.Git.DefaultMergeTarget
}

func (c *Config) GetBranchNamePattern() string {
	if c.System == nil || c.System.Git.BranchNamePattern == "" {
		return "^[a-zA-Z0-9_/-]+$"
	}
	return c.System.Git.BranchNamePattern
}

func (c *Config) GetGitTimeoutSeconds() int {
	if c.System == nil || c.System.Git.DefaultTimeoutSecs == 0 {
		return 60
	}
	return c.System.Git.DefaultTimeoutSecs
}

func (c *Config) GetRemoteName() string {
	if c.System == nil || c.System.Git.RemoteName == "" {
		return "origin"
	}
	return c.System.Git.RemoteName
}

func (c *Config) GetTaskBranchPrefix() string {
	if c.System == nil || c.System.Git.BranchPrefixes == nil {
		return "task/"
	}
	if c.System.Git.BranchPrefixes.Task == "" {
		return "task/"
	}
	return c.System.Git.BranchPrefixes.Task
}
func (c *Config) GetModuleBranchPrefix() string {
	if c.System == nil || c.System.Git.BranchPrefixes == nil {
		return "module/"
	}
	if c.System.Git.BranchPrefixes.Module == "" {
		return "module/"
	}
	return c.System.Git.BranchPrefixes.Module
}
func (c *Config) GetLoggingConfig() *LoggingConfig {
	if c.System == nil {
		return &LoggingConfig{
			MaxOutputLength: 5000,
			MaxIDDisplay:    8,
		}
	}
	return &c.System.Logging
}

func (c *Config) GetHTTPAllowlist() []string {
	if c.System == nil || c.System.Security.HTTPAllowlist == nil {
		return []string{
			"api.supabase.co",
			"api.github.com",
			"github.com",
		}
	}
	return c.System.Security.HTTPAllowlist
}

func (c *Config) GetEventsConfig() *EventsConfig {
	if c.System == nil {
		return &EventsConfig{
			TaskStatusesAvailable:    []string{"available"},
			TaskStatusesReview:       []string{"review"},
			TaskStatusesCompleted:    []string{"testing", "merged"},
			PlanStatusesDraft:        []string{"draft"},
			PlanStatusesCouncil:      []string{"council_review", "revision_needed"},
			PlanStatusesPendingHuman: []string{"pending_human"},
			PlanStatusesApproved:     []string{"approved"},
			MaintenanceStatus:        "pending",
			TestResultsStatus:        "pending_review",
		}
	}
	return &c.System.Events
}

func (c *Config) GetSandboxConfig() *SandboxConfig {
	if c.System == nil {
		return &SandboxConfig{
			DefaultTestCommand: "npm test",
			TimeoutSeconds:     60,
		}
	}
	return &c.System.Sandbox
}

func (c *Config) GetMaxRevisionRounds() int {
	if c.PlanLifecycle == nil {
		return 6
	}
	return c.PlanLifecycle.RevisionRules.MaxRounds
}

func (c *Config) GetOnMaxRoundsAction() string {
	if c.PlanLifecycle == nil {
		return "pending_human"
	}
	return c.PlanLifecycle.RevisionRules.OnMaxRounds
}

func (c *Config) GetCouncilLenses() []string {
	if c.PlanLifecycle == nil || c.PlanLifecycle.CouncilRules.Lenses == nil {
		return []string{"user_alignment", "architecture", "feasibility"}
	}
	return c.PlanLifecycle.CouncilRules.Lenses
}

func (c *Config) GetCouncilMemberCount() int {
	if c.PlanLifecycle == nil {
		return 3
	}
	return c.PlanLifecycle.CouncilRules.MemberCount
}

func (c *Config) ShouldCouncilIncludePRD() bool {
	if c.PlanLifecycle == nil {
		return true
	}
	return c.PlanLifecycle.CouncilRules.Context.IncludePRD
}

func (c *Config) GetConsensusMethod() string {
	if c.PlanLifecycle == nil {
		return "unanimous_approval"
	}
	return c.PlanLifecycle.ConsensusRules.Method
}

func (c *Config) GetWebToolsConfig() *WebToolsConfig {
	if c.System == nil || c.System.WebTools.SearchURL == "" {
		return &WebToolsConfig{
			SearchURL:        "https://api.duckduckgo.com/",
			UserAgent:        "Mozilla/5.0 (compatible; VibePilot/2.0)",
			MaxFetchLength:   10000,
			MaxRelatedTopics: 5,
			TimeoutSeconds:   30,
		}
	}
	return &c.System.WebTools
}

func (c *Config) GetRuntimeConfig() *RuntimeConfig {
	if c.System == nil {
		return &RuntimeConfig{
			MaxConcurrentPerModule: 8,
			MaxConcurrentTotal:     160,
			EventPollIntervalMs:    1000,
			AgentTimeoutSeconds:    300,
			MaxToolTurns:           10,
			EventQueryLimit:        10,
		}
	}
	return &c.System.Runtime
}

func (c *Config) GetValidationConfig() *ValidationConfig {
	if c.System == nil {
		return &ValidationConfig{
			MinTaskConfidence:     0.95,
			RequirePromptPacket:   true,
			RequireCategory:       true,
			RequireExpectedOutput: true,
		}
	}
	return &c.System.Validation
}

func (c *Config) GetCoreConfig() *CoreConfig {
	if c.System == nil || c.System.Core == nil {
		return &CoreConfig{
			CheckpointEnabled:         true,
			CheckpointIntervalPercent: 25,
			StateSyncIntervalSeconds:  60,
			RecoveryEnabled:           true,
		}
	}
	return c.System.Core
}

func (c *Config) GetWebhooksConfig() *WebhooksConfig {
	if c.System == nil || c.System.Webhooks == nil {
		return &WebhooksConfig{
			Enabled:        false,
			Port:           8080,
			Path:           "/webhooks",
			SecretVaultKey: "webhook_secret",
		}
	}
	return c.System.Webhooks
}

func (c *Config) IsWebhooksEnabled() bool {
	cfg := c.GetWebhooksConfig()
	return cfg != nil && cfg.Enabled
}

func (c *Config) GetProcessingTimeoutSeconds() int {
	if c.System == nil || c.System.Recovery == nil {
		return 300
	}
	if v, ok := c.System.Recovery["processing_timeout_seconds"]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 300
}

func (c *Config) GetProcessingRecoveryIntervalSeconds() int {
	if c.System == nil || c.System.Recovery == nil {
		return 60
	}
	if v, ok := c.System.Recovery["processing_recovery_interval_seconds"]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 60
}

func (c *Config) GetOrphanThresholdSeconds() int {
	if c.System == nil || c.System.Recovery == nil {
		return 300
	}
	if v, ok := c.System.Recovery["orphan_threshold_seconds"]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 300
}

func (c *Config) GetRoutingStrategy(agentID string) string {
	if c.Routing == nil {
		return "default"
	}

	internalOnly := c.Routing.AgentRestrictions["internal_only"]
	for _, id := range internalOnly {
		if id == agentID {
			return "internal_only"
		}
	}

	return c.Routing.DefaultStrategy
}

func (c *Config) GetStrategyPriority(strategyName string) []string {
	if c.Routing == nil {
		return []string{"internal"}
	}

	strategy, ok := c.Routing.Strategies[strategyName]
	if !ok {
		return []string{"internal"}
	}

	return strategy.Priority
}

func (c *Config) GetConnectorCategory(connID string) string {
	if c.Routing == nil || c.Connectors == nil {
		log.Printf("[GetConnectorCategory] %s: Routing or Connectors nil, returning 'internal'", connID)
		return "internal"
	}

	conn := c.GetConnector(connID)
	if conn == nil {
		log.Printf("[GetConnectorCategory] %s: connector not found, returning 'internal'", connID)
		return "internal"
	}

	if c.Routing.DestinationCategories == nil {
		log.Printf("[GetConnectorCategory] %s: DestinationCategories nil, returning 'internal'", connID)
		return "internal"
	}

	for categoryName, categoryDef := range c.Routing.DestinationCategories {
		checkField, _ := categoryDef["check_field"].(string)
		checkValues, _ := categoryDef["check_values"].([]interface{})

		var fieldValue string
		switch checkField {
		case "type":
			fieldValue = conn.Type
		case "status":
			fieldValue = conn.Status
		}

		for _, v := range checkValues {
			if vs, ok := v.(string); ok && vs == fieldValue {
				log.Printf("[GetConnectorCategory] %s: matched category '%s' (field=%s value=%s)", connID, categoryName, checkField, fieldValue)
				return categoryName
			}
		}
	}

	log.Printf("[GetConnectorCategory] %s: no category match, returning 'internal'", connID)
	return "internal"
}

func (c *Config) GetConnectorsInCategory(category string) []ConnectorConfig {
	if c.Connectors == nil {
		log.Printf("[GetConnectorsInCategory] Connectors is nil, returning empty")
		return nil
	}

	var result []ConnectorConfig
	for _, conn := range c.Connectors.Connectors {
		connCategory := c.GetConnectorCategory(conn.ID)
		statusMatch := conn.Status == "active"
		log.Printf("[GetConnectorsInCategory] conn=%s type=%s status=%s category=%s matchedCategory=%v statusMatch=%v",
			conn.ID, conn.Type, conn.Status, category, connCategory == category, statusMatch)
		if connCategory == category && conn.Status == "active" {
			result = append(result, conn)
		}
	}
	log.Printf("[GetConnectorsInCategory] category=%s total=%d found=%d", category, len(c.Connectors.Connectors), len(result))
	return result
}

func loadJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &result, nil
}

func (c *Config) GetRunnerTimeoutSecs() int {
	if c.System == nil {
		return 300
	}
	if v := c.System.Execution; v != nil && v.DefaultTimeoutSeconds > 0 {
		return v.DefaultTimeoutSeconds
	}
	return 300
}

func (c *Config) GetSessionTimeoutSecs() int {
	if c.System == nil {
		return 300
	}
	if c.System.Runtime.AgentTimeoutSeconds > 0 {
		return c.System.Runtime.AgentTimeoutSeconds
	}
	return 300
}

func (c *Config) GetDBHTTPTimeoutSecs() int {
	if c.System == nil || c.System.DB == nil {
		return 30
	}
	if c.System.DB.HTTPTimeoutSeconds > 0 {
		return c.System.DB.HTTPTimeoutSeconds
	}
	return 30
}

func (c *Config) GetDBErrorTruncateLen() int {
	if c.System == nil || c.System.DB == nil {
		return 200
	}
	if c.System.DB.ErrorTruncateLength > 0 {
		return c.System.DB.ErrorTruncateLength
	}
	return 200
}

func (c *Config) GetSandboxTimeoutSecs() int {
	if c.System == nil || c.System.Sandbox.TimeoutSeconds == 0 {
		return 60
	}
	return c.System.Sandbox.TimeoutSeconds
}

func (c *Config) GetLintTimeoutSecs() int {
	if c.System == nil || c.System.Sandbox.LintTimeoutSeconds == 0 {
		return 60
	}
	return c.System.Sandbox.LintTimeoutSeconds
}

func (c *Config) GetTypecheckTimeoutSecs() int {
	if c.System == nil || c.System.Sandbox.TypecheckTimeoutSeconds == 0 {
		return 120
	}
	return c.System.Sandbox.TypecheckTimeoutSeconds
}

func (c *Config) GetHTTPClientTimeoutSecs() int {
	if c.System == nil || c.System.HTTP == nil {
		return 30
	}
	if c.System.HTTP.ClientTimeoutSeconds > 0 {
		return c.System.HTTP.ClientTimeoutSeconds
	}
	return 30
}

func (c *Config) GetHTTPIdleTimeoutSecs() int {
	if c.System == nil || c.System.HTTP == nil {
		return 30
	}
	if c.System.HTTP.ResponseTimeoutSeconds > 0 {
		return c.System.HTTP.ResponseTimeoutSeconds
	}
	return 30
}

func (c *Config) GetCourierTimeoutSecs() int {
	if c.System == nil || c.System.Courier == nil {
		return 30
	}
	if c.System.Courier.TimeoutSeconds > 0 {
		return c.System.Courier.TimeoutSeconds
	}
	return 30
}

func (c *Config) GetDefaultCLIArgs() []string {
	return []string{"run", "--format", "json"}
}
