package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Governor GovernorConfig `yaml:"governor"`
	Supabase SupabaseConfig `yaml:"supabase"`
	GitHub   GitHubConfig   `yaml:"github"`
	Server   ServerConfig   `yaml:"server"`
	Runners  RunnersConfig  `yaml:"runners"`
	Courier  CourierConfig  `yaml:"courier"`
	Security SecurityConfig `yaml:"security"`
}

type GovernorConfig struct {
	PollInterval  time.Duration `yaml:"poll_interval"`
	MaxConcurrent int           `yaml:"max_concurrent"`
	StuckTimeout  time.Duration `yaml:"stuck_timeout"`
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
	DriverModel string            `yaml:"driver_model"`
	Platforms   []CourierPlatform `yaml:"platforms"`
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
			PollInterval:  15 * time.Second,
			MaxConcurrent: 3,
			StuckTimeout:  10 * time.Minute,
		},
		Server: ServerConfig{
			Addr: ":8080",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.expandEnv()

	return cfg, nil
}

func (c *Config) expandEnv() {
	c.Supabase.URL = os.ExpandEnv(c.Supabase.URL)
	c.Supabase.ServiceKey = os.ExpandEnv(c.Supabase.ServiceKey)
	c.GitHub.Token = os.ExpandEnv(c.GitHub.Token)
}
