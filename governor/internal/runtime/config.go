package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type SystemConfig struct {
	Database DatabaseConfig `json:"database"`
	Vault    VaultConfig    `json:"vault"`
	Git      GitConfig      `json:"git"`
	Runtime  RuntimeConfig  `json:"runtime"`
}

type DatabaseConfig struct {
	Type   string `json:"type"`
	URLEnv string `json:"url_env"`
	KeyEnv string `json:"key_env"`
}

type VaultConfig struct {
	KeyEnv string `json:"key_env"`
	Table  string `json:"table"`
}

type GitConfig struct {
	Host     string `json:"host"`
	RepoEnv  string `json:"repo_env"`
	TokenEnv string `json:"token_env"`
}

type RuntimeConfig struct {
	MaxConcurrentPerModule int `json:"max_concurrent_per_module"`
	MaxConcurrentTotal     int `json:"max_concurrent_total"`
	EventPollIntervalMs    int `json:"event_poll_interval_ms"`
	AgentTimeoutSeconds    int `json:"agent_timeout_seconds"`
}

type AgentConfig struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name,omitempty"`
	Prompt             string          `json:"prompt"`
	Tools              []string        `json:"tools"`
	Model              string          `json:"model,omitempty"`
	DefaultDestination string          `json:"default_destination"`
	Description        string          `json:"description,omitempty"`
	Capabilities       map[string]bool `json:"capabilities,omitempty"`
}

type AgentsFile struct {
	Version string        `json:"version"`
	Agents  []AgentConfig `json:"agents"` // Array format from existing config
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
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Status    string                 `json:"status"`
	Command   string                 `json:"command,omitempty"`
	Endpoint  string                 `json:"endpoint,omitempty"`
	APIKeyRef string                 `json:"api_key_ref,omitempty"`
	Models    []string               `json:"models_available,omitempty"`
	Extra     map[string]interface{} `json:"-"` // Capture unknown fields
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
}

type ModelsFile struct {
	Models []ModelConfig `json:"models"`
}

type Config struct {
	System       *SystemConfig
	Agents       *AgentsFile
	Tools        *ToolsFile
	Destinations *DestinationsFile
	Models       *ModelsFile

	systemPath       string
	agentsPath       string
	toolsPath        string
	destinationsPath string
	modelsPath       string
	promptsDir       string

	mu sync.RWMutex
}

func LoadConfig(configDir string) (*Config, error) {
	cfg := &Config{
		systemPath:       filepath.Join(configDir, "system.json"),
		agentsPath:       filepath.Join(configDir, "agents.json"),
		toolsPath:        filepath.Join(configDir, "tools.json"),
		destinationsPath: filepath.Join(configDir, "destinations.json"),
		modelsPath:       filepath.Join(configDir, "models.json"),
		promptsDir:       filepath.Join(configDir, "prompts"),
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

	if c.System == nil {
		c.System = &SystemConfig{
			Database: DatabaseConfig{
				Type:   "supabase",
				URLEnv: "SUPABASE_URL",
				KeyEnv: "SUPABASE_KEY",
			},
			Vault: VaultConfig{
				KeyEnv: "VAULT_KEY",
				Table:  "secrets_vault",
			},
			Runtime: RuntimeConfig{
				MaxConcurrentPerModule: 8,
				MaxConcurrentTotal:     160,
				EventPollIntervalMs:    1000,
				AgentTimeoutSeconds:    300,
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
	fullPath := filepath.Join(c.promptsDir, filepath.Base(promptPath))
	c.mu.RUnlock()

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", fullPath, err)
	}
	return string(data), nil
}

func (c *Config) AgentHasTool(agentID, toolName string) bool {
	agent := c.GetAgent(agentID)
	if agent == nil {
		return false
	}
	for _, t := range agent.Tools {
		if t == toolName {
			return true
		}
	}
	return false
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
