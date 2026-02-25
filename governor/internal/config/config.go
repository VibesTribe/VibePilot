package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Governor    GovernorConfig    `yaml:"governor"`
	Supabase    SupabaseConfig    `yaml:"supabase"`
	GitHub      GitHubConfig      `yaml:"github"`
	Server      ServerConfig      `yaml:"server"`
	Runners     RunnersConfig     `yaml:"runners"`
	Courier     CourierConfig     `yaml:"courier"`
	Security    SecurityConfig    `yaml:"security"`
	Deprecation DeprecationConfig `yaml:"depreciation"`
	Analyst     AnalystConfig     `yaml:"analyst"`
	Agents      AgentsConfig      `yaml:"agents"`
	Git         GitConfig         `yaml:"git"`
}

type GitConfig struct {
	ProtectedBranches []string `yaml:"protected_branches"`
}

type AgentsConfig struct {
	ConfigDir      string `yaml:"config_dir"`
	PromptsDir     string `yaml:"prompts_dir"`
	ParallelMax    int    `yaml:"parallel_max"`
	SequentialMode bool   `yaml:"sequential_mode"`
}

type DeprecationConfig struct {
	Enabled          bool    `yaml:"enabled"`
	ArchiveThreshold float64 `yaml:"archive_threshold"`
	MinAttempts      int     `yaml:"min_attempts"`
	CooldownHours    int     `yaml:"cooldown_hours"`
}

type AnalystConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Schedule string `yaml:"schedule"`
}

type GovernorConfig struct {
	PollInterval     time.Duration `yaml:"poll_interval"`
	MaxConcurrent    int           `yaml:"max_concurrent"`
	StuckTimeout     time.Duration `yaml:"stuck_timeout"`
	MaxPerModule     int           `yaml:"max_per_module"`
	RepoPath         string        `yaml:"repo_path"`
	TaskTimeoutSec   int           `yaml:"task_timeout_sec"`
	CouncilMaxRounds int           `yaml:"council_max_rounds"`
}

type SupabaseConfig struct {
	URL        string `yaml:"url"`
	ServiceKey string `yaml:"service_key"`
}

type GitHubConfig struct {
	Token    string `yaml:"token"`
	Owner    string `yaml:"owner"`
	Repo     string `yaml:"repo"`
	Workflow string `yaml:"workflow"`
}

type ServerConfig struct {
	Addr          string `yaml:"addr"`
	DashboardDist string `yaml:"dashboard_dist"`
}

type RunnersConfig struct {
	Internal []InternalRunner `yaml:"internal"`
}

type InternalRunner struct {
	ModelID    string `yaml:"model_id"`
	Tool       string `yaml:"tool"`
	RAMLimitMB int    `yaml:"ram_limit_mb"`
}

type CourierConfig struct {
	Enabled       bool              `yaml:"enabled"`
	MaxInFlight   int               `yaml:"max_in_flight"`
	Stagger       time.Duration     `yaml:"stagger"`
	CallbackURL   string            `yaml:"callback_url"`
	WebhookSecret string            `yaml:"webhook_secret"`
	DriverModel   string            `yaml:"driver_model"`
	Platforms     []CourierPlatform `yaml:"platforms"`
}

type CourierPlatform struct {
	ID         string `yaml:"id"`
	URL        string `yaml:"url"`
	DailyLimit int    `yaml:"daily_limit"`
}

type SecurityConfig struct {
	AllowedHosts []string `yaml:"allowed_hosts"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Governor: GovernorConfig{
			PollInterval:     15 * time.Second,
			MaxConcurrent:    3,
			StuckTimeout:     10 * time.Minute,
			MaxPerModule:     8,
			TaskTimeoutSec:   300,
			CouncilMaxRounds: 4,
		},
		Server: ServerConfig{
			Addr: ":8080",
		},
		Courier: CourierConfig{
			Enabled:     false,
			MaxInFlight: 3,
			Stagger:     30 * time.Second,
		},
		Deprecation: DeprecationConfig{
			Enabled:          true,
			ArchiveThreshold: 0.7,
			MinAttempts:      5,
			CooldownHours:    24,
		},
		Analyst: AnalystConfig{
			Enabled:  true,
			Schedule: "00:00",
		},
		Agents: AgentsConfig{
			ConfigDir:      "./config",
			PromptsDir:     "./config/prompts",
			ParallelMax:    50,
			SequentialMode: false,
		},
		Git: GitConfig{
			ProtectedBranches: []string{"main", "master"},
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.expandEnv()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Supabase.URL == "" {
		return fmt.Errorf("supabase.url is required")
	}
	if c.Supabase.ServiceKey == "" {
		return fmt.Errorf("supabase.service_key is required")
	}
	return nil
}

func (c *Config) expandEnv() {
	c.Supabase.URL = os.ExpandEnv(c.Supabase.URL)
	c.Supabase.ServiceKey = os.ExpandEnv(c.Supabase.ServiceKey)
	c.GitHub.Token = os.ExpandEnv(c.GitHub.Token)
}
