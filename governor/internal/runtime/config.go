package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type SystemConfig struct {
	Database    DatabaseConfig         `json:"database"`
	Vault       VaultConfig            `json:"vault"`
	Git         GitConfig              `json:"git"`
	Runtime     RuntimeConfig          `json:"runtime"`
	Concurrency ConcurrencyConfig      `json:"concurrency"`
	Security    SecurityConfig         `json:"security"`
	Events      EventsConfig           `json:"events"`
	Sandbox     SandboxConfig          `json:"sandbox"`
	WebTools    WebToolsConfig         `json:"web_tools"`
	PRDWatcher  PRDWatcherSystemConfig `json:"prd_watcher"`
	Recovery    map[string]interface{} `json:"recovery"`
	Defaults    map[string]interface{} `json:"defaults"`
	Validation  ValidationConfig       `json:"validation"`
	Logging     LoggingConfig          `json:"logging"`
	PromptsDir  string                 `json:"prompts_dir"`
}

type ValidationConfig struct {
	MinTaskConfidence     float64 `json:"min_task_confidence"`
	RequirePromptPacket   bool    `json:"require_prompt_packet"`
	RequireCategory       bool    `json:"require_category"`
	RequireExpectedOutput bool    `json:"require_expected_output"`
}

// DatabaseConfig specifies how to connect to Supabase.
// KeyEnv MUST be SUPABASE_SERVICE_KEY (not anon key) because:
//   - Vault table requires service role to bypass RLS
//   - Anon key cannot read/write secrets_vault
//
// See governor/internal/vault/vault.go for full architecture docs.
type DatabaseConfig struct {
	Type   string `json:"type"`
	URLEnv string `json:"url_env"`
	KeyEnv string `json:"key_env"`
}

type VaultConfig struct {
	KeyEnv          string `json:"key_env"`
	Table           string `json:"table"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
}

type GitConfig struct {
	Host               string   `json:"host"`
	RepoEnv            string   `json:"repo_env"`
	TokenEnv           string   `json:"token_env"`
	ProtectedBranches  []string `json:"protected_branches"`
	DefaultTimeoutSecs int      `json:"default_timeout_seconds"`
	DefaultMergeTarget string   `json:"default_merge_target"`
	BranchNamePattern  string   `json:"branch_name_pattern"`
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
	DefaultTestCommand string `json:"default_test_command"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	TempDir            string `json:"temp_dir"`
}

type WebToolsConfig struct {
	SearchURL        string `json:"search_url"`
	UserAgent        string `json:"user_agent"`
	MaxFetchLength   int    `json:"max_fetch_length"`
	MaxRelatedTopics int    `json:"max_related_topics"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
}

type PRDWatcherSystemConfig struct {
	Enabled         bool   `json:"enabled"`
	RepoPath        string `json:"repo_path"`
	Branch          string `json:"branch"`
	Directory       string `json:"directory"`
	IntervalSeconds int    `json:"interval_seconds"`
}

type AgentConfig struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name,omitempty"`
	Prompt             string   `json:"prompt"`
	Capabilities       []string `json:"capabilities,omitempty"`
	Model              string   `json:"model,omitempty"`
	DefaultDestination string   `json:"default_destination"`
	Description        string   `json:"description,omitempty"`
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

type DestinationConfig struct {
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
	Extra          map[string]interface{} `json:"-"`
}

type DestinationsFile struct {
	Destinations []DestinationConfig `json:"destinations"`
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
	Destinations  *DestinationsFile
	Models        *ModelsFile
	Routing       *RoutingConfig
	PlanLifecycle *PlanLifecycleConfig

	systemPath        string
	agentsPath        string
	toolsPath         string
	destinationsPath  string
	modelsPath        string
	routingPath       string
	planLifecyclePath string
	promptsDir        string

	mu sync.RWMutex
}

func LoadConfig(configDir string) (*Config, error) {
	cfg := &Config{
		systemPath:        filepath.Join(configDir, "system.json"),
		agentsPath:        filepath.Join(configDir, "agents.json"),
		toolsPath:         filepath.Join(configDir, "tools.json"),
		destinationsPath:  filepath.Join(configDir, "destinations.json"),
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

	if dests, err := loadJSON[DestinationsFile](c.destinationsPath); err == nil {
		c.Destinations = dests
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load destinations.json: %w", err)
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
				TaskStatusesCompleted: []string{"testing", "approval"},
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

func (c *Config) GetDestination(id string) *DestinationConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Destinations == nil {
		return nil
	}
	for i := range c.Destinations.Destinations {
		if c.Destinations.Destinations[i].ID == id {
			return &c.Destinations.Destinations[i]
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
	c.mu.RLock()
	promptsDir := c.promptsDir
	if c.System != nil && c.System.PromptsDir != "" {
		promptsDir = c.System.PromptsDir
	}
	fullPath := filepath.Join(promptsDir, filepath.Base(promptPath))
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

func (c *Config) GetVaultKey() string {
	return os.Getenv(c.System.Vault.KeyEnv)
}

func (c *Config) GetProtectedBranches() []string {
	if c.System == nil || c.System.Git.ProtectedBranches == nil {
		return []string{"main", "master"}
	}
	return c.System.Git.ProtectedBranches
}

func (c *Config) GetRepoPath() string {
	if c.System == nil {
		return "."
	}
	if c.System.PRDWatcher.RepoPath != "" {
		return c.System.PRDWatcher.RepoPath
	}
	return "."
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
			TaskStatusesCompleted:    []string{"testing", "approval"},
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

func (c *Config) GetDestinationCategory(destID string) string {
	if c.Routing == nil || c.Destinations == nil {
		return "internal"
	}

	dest := c.GetDestination(destID)
	if dest == nil {
		return "internal"
	}

	for categoryName, categoryDef := range c.Routing.DestinationCategories {
		checkField, _ := categoryDef["check_field"].(string)
		checkValues, _ := categoryDef["check_values"].([]interface{})

		var fieldValue string
		switch checkField {
		case "type":
			fieldValue = dest.Type
		case "status":
			fieldValue = dest.Status
		}

		for _, v := range checkValues {
			if vs, ok := v.(string); ok && vs == fieldValue {
				return categoryName
			}
		}
	}

	return "internal"
}

func (c *Config) GetDestinationsInCategory(category string) []DestinationConfig {
	if c.Destinations == nil {
		return nil
	}

	var result []DestinationConfig
	for _, dest := range c.Destinations.Destinations {
		if c.GetDestinationCategory(dest.ID) == category && dest.Status == "active" {
			result = append(result, dest)
		}
	}
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
