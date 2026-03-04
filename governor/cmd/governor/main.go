package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vibepilot/governor/internal/connectors"
	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/internal/tools"
	"github.com/vibepilot/governor/internal/vault"
	"github.com/vibepilot/governor/internal/webhooks"
)

var (
	version = "2.0.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	log.Printf("VibePilot Governor %s (commit: %s, built: %s)", version, commit, date)

	configDir := getConfigDir()
	cfg, err := runtime.LoadConfig(configDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbURL := cfg.GetDatabaseURL()
	dbKey := cfg.GetDatabaseKey()
	if dbURL == "" || dbKey == "" {
		log.Fatal("Database credentials required: set SUPABASE_URL and SUPABASE_SERVICE_KEY")
	}

	database := db.New(dbURL, dbKey)
	defer database.Close()
	log.Println("Connected to database")

	stateMachine := core.NewStateMachine()
	checkpointStorage := &dbCheckpointAdapter{db: database}
	checkpointMgr := core.NewCheckpointManager(stateMachine, checkpointStorage)
	log.Println("Core state machine initialized")

	repoPath := getEnvOrDefault("REPO_PATH", ".")
	protectedBranches := cfg.GetProtectedBranches()

	git := gitree.New(&gitree.Config{
		RepoPath:          repoPath,
		ProtectedBranches: protectedBranches,
		Timeout:           time.Duration(cfg.GetGitTimeoutSeconds()) * time.Second,
		RemoteName:        cfg.GetRemoteName(),
	})

	v := vault.New(database)

	leakDetector := security.NewLeakDetector()

	// TODO: Wire in maintenance package after type refactoring
	// The maintenance package uses pkg/types.Task which doesn't match map[string]any
	// used throughout the Go rewrite. Needs refactoring to use map[string]any.
	// _ = maintenance.New(&maintenance.Config{...}, database, git)

	toolRegistry := runtime.NewToolRegistry(cfg)
	tools.RegisterAll(toolRegistry, &tools.Dependencies{
		DB:       database,
		Git:      git,
		Vault:    v,
		RepoPath: repoPath,
		Config:   cfg,
	})

	sessionFactory := runtime.NewSessionFactory(cfg)
	registerConnectors(sessionFactory, cfg, v)

	contextBuilder := runtime.NewContextBuilder(database)
	sessionFactory.SetContextBuilder(contextBuilder)

	pool := runtime.NewAgentPoolWithConcurrency(
		cfg.System.Runtime.MaxConcurrentPerModule,
		cfg.System.Runtime.MaxConcurrentTotal,
		&cfg.System.Concurrency,
	)

	usageTracker := runtime.NewUsageTracker(database)

	_, err = runtime.LoadModelsFromConfig(configDir, database, usageTracker)
	if err != nil {
		log.Printf("Warning: Failed to load model profiles: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recoveryCfg := getRecoveryConfig(cfg)
	runStartupRecovery(ctx, database, recoveryCfg)
	runCheckpointRecovery(ctx, database, cfg, checkpointMgr)

	connRouter := runtime.NewRouter(cfg, database)
	eventRouter := runtime.NewEventRouter(nil)

	var webhookSecret string
	if cfg.IsWebhooksEnabled() {
		webhookCfg := cfg.GetWebhooksConfig()
		secret, err := v.GetSecret(ctx, webhookCfg.SecretVaultKey)
		if err != nil {
			log.Printf("Warning: Failed to get webhook secret from vault: %v", err)
		} else {
			webhookSecret = secret
		}
	}

	webhookServer := webhooks.NewServer(&webhooks.Config{
		Port:   cfg.GetWebhooksConfig().Port,
		Path:   cfg.GetWebhooksConfig().Path,
		Secret: webhookSecret,
	}, eventRouter)

	githubHandler := webhooks.NewGitHubWebhookHandler(database, cfg.System.PRDWatcher.RepoPath)
	webhookServer.SetGitHubHandler(githubHandler)

	go runProcessingRecovery(ctx, database, cfg)

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, connRouter, git, stateMachine, checkpointMgr, leakDetector)

	if err := webhookServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start webhook server: %v", err)
	}

	webhookCfg := cfg.GetWebhooksConfig()
	log.Printf("Governor started (webhooks: port %d%s, max/module: %d, max total: %d, opencode limit: %d)",
		webhookCfg.Port, webhookCfg.Path, cfg.System.Runtime.MaxConcurrentPerModule, cfg.System.Runtime.MaxConcurrentTotal, cfg.System.Concurrency.GetLimit("opencode"))
	log.Println("Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()
	webhookServer.Shutdown(ctx)
	pool.Wait()

	log.Println("Governor stopped")
}

func getConfigDir() string {
	if dir := os.Getenv("GOVERNOR_CONFIG_DIR"); dir != "" {
		return dir
	}
	return "./config"
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func registerConnectors(factory *runtime.SessionFactory, cfg *runtime.Config, v *vault.Vault) {
	connectorsCfg := cfg.Connectors
	if connectorsCfg == nil {
		log.Println("Warning: no connectors configured")
		return
	}

	secretProvider := connectors.NewVaultAdapter(v)

	for _, conn := range connectorsCfg.Connectors {
		if conn.Status != "active" {
			log.Printf("Skipping inactive connector: %s", conn.ID)
			continue
		}

		switch conn.Type {
		case "cli":
			timeout := cfg.GetRunnerTimeoutSecs()
			if conn.TimeoutSeconds > 0 {
				timeout = conn.TimeoutSeconds
			}
			cliArgs := conn.CLIArgs
			if len(cliArgs) == 0 {
				cliArgs = cfg.GetDefaultCLIArgs()
			}
			runner := connectors.NewCLIRunnerWithArgs(conn.Command, cliArgs, timeout)
			factory.RegisterConnector(conn.ID, runner)
			log.Printf("Registered CLI connector: %s (%s)", conn.ID, conn.Command)
		case "api":
			runner := connectors.NewAPIRunnerFromConfig(conn, secretProvider)
			factory.RegisterConnector(conn.ID, runner)
			log.Printf("Registered API connector: %s (%s)", conn.ID, conn.Endpoint)
		default:
			log.Printf("Unknown connector type: %s for %s", conn.Type, conn.ID)
		}
	}
}

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, connRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector) {
	setupTaskHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, checkpointMgr, leakDetector)
	setupPlanHandlers(ctx, router, factory, pool, database, cfg, connRouter, git)
	setupCouncilHandlers(ctx, router, factory, pool, database, cfg, connRouter)
	setupMaintenanceHandler(ctx, router, factory, pool, database, cfg, connRouter)
	setupTestingHandlers(ctx, router, factory, pool, database, cfg, connRouter, git)
	setupResearchHandlers(ctx, router, factory, pool, database, cfg, connRouter)
}
