package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vibepilot/governor/internal/connectors"
	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/dag"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	govmcp "github.com/vibepilot/governor/internal/mcp"
	"github.com/vibepilot/governor/internal/memory"
	"github.com/vibepilot/governor/internal/realtime"
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
	// Check for smoke test flag
	if len(os.Args) > 1 && os.Args[1] == "-smoke-test" {
		dbURL := os.Getenv("SUPABASE_URL")
		dbKey := os.Getenv("SUPABASE_SERVICE_KEY")
		if dbURL == "" || dbKey == "" {
			log.Fatal("SUPABASE_URL and SUPABASE_SERVICE_KEY required for smoke test")
		}
		runSmokeTest(dbURL, dbKey)
		return
	}

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

	cfg.SetDatabase(database)

	// Validate all dependencies before accepting work
	validateDB := &startupDBAdapter{db: database}
	startupErrors := startupValidate(configDir, validateDB)
	if startupErrors > 0 {
		log.Printf("[Startup] Governor starting in DEGRADED mode (%d validation errors). Some features may not work.", startupErrors)
	}

	if err := cfg.SyncPromptsToDB(); err != nil {
		log.Printf("Warning: failed to sync prompts to DB: %v", err)
	}

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

	// Set up worktree manager for parallel agent execution
	var worktreeMgr *gitree.WorktreeManager
	if cfg.System.Worktrees != nil && cfg.System.Worktrees.Enabled {
		worktreeMgr = gitree.NewWorktreeManager(git, cfg.System.Worktrees.BasePath)
		log.Printf("[Worktrees] Enabled, base path: %s", cfg.System.Worktrees.BasePath)
	} else {
		log.Printf("[Worktrees] Disabled (no worktrees config or enabled=false)")
	}

	v := vault.New(database)

	var courierRunner *connectors.CourierRunner // initialized after ctx is created

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
	registerConnectors(sessionFactory, cfg, v, repoPath)

	contextBuilder := runtime.NewContextBuilder(database)
	sessionFactory.SetContextBuilder(contextBuilder)

	// Wire compactor for automatic session summary generation
	compactor := memory.NewCompactor(database)
	sessionFactory.SetCompactor(compactor)

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

	// Restore persisted usage state (rate limits, cooldowns, learned data) from database
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := usageTracker.LoadFromDatabase(loadCtx); err != nil {
		log.Printf("[UsageTracker] Warning: failed to load persisted state: %v", err)
	}
	loadCancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize CourierRunner for web platform dispatch (courier agent system)
	// Nil if GITHUB_TOKEN is not available — web routing falls back to internal gracefully
	if githubToken, err := v.GetSecret(ctx, "GITHUB_TOKEN"); err == nil && githubToken != "" {
		courierRunner = connectors.NewCourierRunner(githubToken, "VibesTribe/VibePilot", database, 0)
		log.Println("[Courier] Runner initialized (GitHub Actions dispatch enabled)")
	} else {
		log.Printf("[Courier] Runner disabled (GITHUB_TOKEN unavailable: %v)", err)
	}

	// Initialize MCP registry (connects to approved external MCP servers)
	var mcpRegistry *govmcp.Registry
	if len(cfg.System.MCPServers) > 0 {
		mcpRegistry = govmcp.NewRegistry(cfg.System.MCPServers)
		if err := mcpRegistry.Start(ctx); err != nil {
			log.Printf("Warning: MCP registry startup had errors: %v", err)
		}
		// Wire MCP tools into agent context so agents know what's available
		contextBuilder.SetMCPRegistry(mcpRegistry)
		// Register MCP tools in the tool registry so agents can call them
		mcpRegistry.RegisterToolsInRegistry(toolRegistry)
	} else {
		log.Println("[MCP] No MCP servers configured")
	}

	// Start MCP server to expose governor tools to external agents
	var govMCPServer *govmcp.GovernorServer
	if cfg.System.GovernorMCP != nil && cfg.System.GovernorMCP.Enabled {
		govMCPServer = govmcp.NewGovernorServer(toolRegistry, cfg, *cfg.System.GovernorMCP)
		if err := govMCPServer.Start(ctx); err != nil {
			log.Printf("Warning: Governor MCP server failed to start: %v", err)
		}
	}

	// Load pipeline workflows from config/pipelines/
	pipelinesDir := filepath.Join(configDir, "pipelines")
	dagRegistry := dag.NewRegistry(pipelinesDir)
	if err := dagRegistry.LoadAll(); err != nil {
		log.Printf("Warning: Failed to load pipelines: %v", err)
	} else {
		for _, name := range dagRegistry.List() {
			log.Printf("[DAG] Loaded pipeline: %s", name)
		}
	}

	recoveryCfg := getRecoveryConfig(cfg)
	runStartupRecovery(ctx, database, recoveryCfg)
	runCheckpointRecovery(ctx, database, cfg, checkpointMgr)

	connRouter := runtime.NewRouter(cfg, database, usageTracker)
	eventRouter := runtime.NewEventRouter(nil)

	// Initialize Realtime client (replaces broken pg_net webhooks)
	var realtimeClient *realtime.Client
	if realtimeURL := cfg.GetRealtimeURL(); realtimeURL != "" {
		realtimeClient = realtime.NewClient(&realtime.Config{
			URL:    realtimeURL,
			APIKey: dbKey, // Use service key for full access
		}, eventRouter)

		if err := realtimeClient.Connect(); err != nil {
			log.Printf("Warning: Failed to connect to Realtime: %v (will retry)", err)
		} else {
			if err := realtimeClient.SubscribeToAllTables(); err != nil {
				log.Printf("Warning: Failed to subscribe to tables: %v", err)
			}
		}
	}

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

	githubHandler := webhooks.NewGitHubWebhookHandler(database, cfg.GetRepoPath())
	webhookServer.SetGitHubHandler(githubHandler)

	go runProcessingRecovery(ctx, database, cfg)

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, connRouter, git, stateMachine, checkpointMgr, leakDetector, usageTracker, worktreeMgr, courierRunner, v)

	// Periodically persist usage tracker state to Supabase for dashboard
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Final persist before shutdown — use fresh context since the main one is canceled
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				usageTracker.PersistToDatabase(shutdownCtx)
				cancel()
				return
			case <-ticker.C:
				usageTracker.PersistToDatabase(ctx)
			}
		}
	}()

	// Rehydrate: fire synthetic events for any active tasks/plans so the
	// governor picks them up without waiting for a realtime event.
	runStartupRehydration(ctx, database, eventRouter)

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
	if mcpRegistry != nil {
		mcpRegistry.Shutdown()
	}
	if govMCPServer != nil {
		govMCPServer.Shutdown()
	}
	if realtimeClient != nil {
		realtimeClient.Close()
	}
	if worktreeMgr != nil {
		log.Println("[Worktrees] Cleaning up worktrees...")
		worktreeMgr.CleanAllWorktrees(context.Background())
	}
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

func registerConnectors(factory *runtime.SessionFactory, cfg *runtime.Config, v *vault.Vault, repoPath string) {
	connectorsCfg := cfg.Connectors
	if connectorsCfg == nil {
		log.Println("Warning: no connectors configured")
		return
	}

	secretProvider := connectors.NewVaultAdapter(v)
	activeCount := 0

	for _, conn := range connectorsCfg.Connectors {
		if conn.Status != "active" {
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
			runner := connectors.NewCLIRunnerWithWorkDir(conn.Command, cliArgs, timeout, repoPath)
			factory.RegisterConnector(conn.ID, runner)
			log.Printf("Registered: %s (%s)", conn.ID, conn.Command)
			activeCount++
		case "api":
			runner := connectors.NewAPIRunnerFromConfig(conn, secretProvider)
			factory.RegisterConnector(conn.ID, runner)
			log.Printf("Registered: %s (api)", conn.ID)
			activeCount++
		default:
			// Skip unknown types silently
		}
	}

	if activeCount == 0 {
		log.Println("Warning: no active connectors registered")
	}
}

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, connRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v *vault.Vault) {
	setupTaskHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, checkpointMgr, leakDetector, usageTracker, worktreeMgr, courierRunner, v)
	setupPlanHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, usageTracker)
	setupCouncilHandlers(ctx, router, factory, pool, database, cfg, connRouter, git)
	setupMaintenanceHandler(ctx, router, factory, pool, database, cfg, connRouter, git)
	setupTestingHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, worktreeMgr)
	setupResearchHandlers(ctx, router, factory, pool, database, cfg, connRouter)

	// Courier result handler: delivers Supabase realtime notifications to waiting courier goroutines
	if courierRunner != nil {
		router.On(runtime.EventCourierResult, func(event runtime.Event) {
			var record struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				Output    string `json:"output"`
				Error     string `json:"error"`
				TokensIn  int    `json:"tokens_in"`
				TokensOut int    `json:"tokens_out"`
			}
			if err := json.Unmarshal(event.Record, &record); err != nil {
				log.Printf("[CourierResult] Failed to parse record: %v", err)
				return
			}
			courierRunner.NotifyResult(record.ID, &connectors.TaskRunResult{
				Status:    record.Status,
				Output:    record.Output,
				Error:     record.Error,
				TokensIn:  record.TokensIn,
				TokensOut: record.TokensOut,
			})
		})
	}
}
