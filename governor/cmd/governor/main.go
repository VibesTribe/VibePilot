package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/vibepilot/governor/internal/pgnotify"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/internal/tools"
	"github.com/vibepilot/governor/internal/vault"
	"github.com/vibepilot/governor/internal/webhooks"

	pgkb "github.com/vibepilot/governor/internal/kb"
	"github.com/vibepilot/governor/internal/visualqa"

	"github.com/vibepilot/governor/internal/designpreview"
)

var (
	version = "2.0.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	// Check for smoke test flag
	if len(os.Args) > 1 && os.Args[1] == "-smoke-test" {
		dbURL := os.Getenv("DATABASE_URL")
		dbKey := os.Getenv("VAULT_KEY")
		if dbURL == "" {
			log.Fatal("DATABASE_URL required for smoke test")
		}
		runSmokeTest(dbURL, dbKey)
		return
	}

	// Vault CLI: ./governor vault <set|get|list|delete|rotate-key> [args...]
	if len(os.Args) > 1 && os.Args[1] == "vault" {
		runVaultCLI(os.Args[2:])
		return
	}

	log.Printf("VibePilot Governor %s (commit: %s, built: %s)", version, commit, date)

	configDir := getConfigDir()
	cfg, err := runtime.LoadConfig(configDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbType := cfg.GetDatabaseType()

	var database db.Database
	switch dbType {
	case "postgres":
		pgURL := cfg.GetPostgresURL()
		if pgURL == "" {
			log.Fatal("Postgres backend selected but DATABASE_URL not set")
		}
		var err error
		database, err = db.NewPostgres(context.Background(), pgURL)
		if err != nil {
			log.Fatalf("Failed to connect to Postgres: %v", err)
		}
		log.Println("Connected to Postgres database")
	default:
		log.Fatalf("Unsupported database type: %q. Only \"postgres\" is supported. PostgREST/Supabase backend was removed.", dbType)
	}
	defer database.Close()

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

	// Initialize vault early -- needed for GITHUB_TOKEN to bootstrap managed repo
	v := vault.New(database)
	v.InitVaultKey(cfg.GetVaultKeyEnv())

	// ==========================================================================
	// MANAGED REPO: The governor owns its own git clone, separate from any
	// editing clone or external directory. It bootstraps from GitHub on first
	// run and always resets to a clean state on startup. No shared state.
	// ==========================================================================
	repoOwner := cfg.GetGitHubOwner()
	repoName := cfg.GetGitHubRepoName()
	githubToken := ""
	if t, err := v.GetSecret(context.Background(), "GITHUB_TOKEN"); err == nil && t != "" {
		githubToken = t
	}

	managedRepo, err := gitree.NewManagedRepo(context.Background(), gitree.ManagedRepoConfig{
		GitHubOwner:       repoOwner,
		GitHubRepo:        repoName,
		GitHubToken:       githubToken,
		DataDir:           cfg.GetDataDir(),
		MainBranch:        cfg.GetDefaultMergeTarget(),
		ProtectedBranches: cfg.GetProtectedBranches(),
		Timeout:           time.Duration(cfg.GetGitTimeoutSeconds()) * time.Second,
		GitUserEmail:      cfg.GetGitUserEmail(),
		GitUserName:       cfg.GetGitUserName(),
	})
	if err != nil {
		log.Fatalf("Failed to bootstrap managed repo for %s/%s: %v", repoOwner, repoName, err)
	}
	git := managedRepo.Gitree()
	repoPath := managedRepo.LocalPath()
	log.Printf("[ManagedRepo] Ready: %s/%s at %s", repoOwner, repoName, repoPath)

	// CRITICAL: Clean git state on startup. Remove stale branches and worktrees
	// from previous runs. This is safe because the managed repo is ours alone.
	log.Printf("[Startup] Cleaning managed repo state...")
	managedRepo.CleanStaleBranches(context.Background())
	managedRepo.CleanStaleWorktrees(context.Background())
	if err := managedRepo.Reset(context.Background()); err != nil {
		log.Printf("[Startup] WARNING: managed repo reset failed: %v", err)
	}
	log.Printf("[Startup] Managed repo clean, on %s", git.MainBranch())

	// Point prompts to the managed repo's prompts directory.
	// Prompts are project-specific and live in the repo, not hardcoded.
	repoPromptsDir := filepath.Join(repoPath, "prompts")
	if _, err := os.Stat(repoPromptsDir); err != nil {
		repoPromptsDir = filepath.Join(repoPath, "config", "prompts")
	}
	cfg.SetPromptsDir(repoPromptsDir)
	log.Printf("[Prompts] Using: %s", repoPromptsDir)

	// Point all repo-path references to the managed repo.
	// Handlers use cfg.GetRepoPath() for file reads/writes.
	cfg.SetRepoPath(repoPath)
	log.Printf("[RepoPath] Using managed repo: %s", repoPath)

	// Set up worktree manager for parallel agent execution
	var worktreeMgr *gitree.WorktreeManager
	worktreeBasePath := cfg.GetWorktreeBasePath()
	if worktreeBasePath == "" {
		worktreeBasePath = managedRepo.WorktreeBasePath()
	}
	worktreeMgr = gitree.NewWorktreeManager(git, worktreeBasePath)
	log.Printf("[Worktrees] Enabled, base path: %s", worktreeBasePath)

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

	contextBuilder := runtime.NewContextBuilder(database, repoPath, cfg.System.CodeMap)
	sessionFactory.SetContextBuilder(contextBuilder)

	// Wire knowledge base provider for DB-backed code map (falls back to disk if unavailable)
	if kbConn, err := pgkb.New(context.Background(), "dbname=vibepilot"); err == nil {
		contextBuilder.SetKBProvider(&kbAdapter{inner: kbConn})
		log.Println("[KB] Knowledge base connected — code map will use DB queries")
	} else {
		log.Printf("[KB] Knowledge base unavailable, using disk-based code map: %v", err)
	}

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

	// Start CooldownWatcher: probes models coming off cooldown, brings them back or extends
	cooldownWatcher := runtime.NewCooldownWatcher(usageTracker, sessionFactory, cfg, database)
	if ms := cfg.GetCooldownPollIntervalMs(); ms > 0 {
		cooldownWatcher.SetPollInterval(time.Duration(ms) * time.Millisecond)
	}
	cooldownWatcher.Start(ctx)
	_ = cooldownWatcher // runs in background goroutine, stopped by ctx cancel

	// Initialize CourierRunner for web platform dispatch (courier agent system)
	repoSlug := repoOwner + "/" + repoName // from ManagedRepo config above
	if githubToken, err := v.GetSecret(ctx, "GITHUB_TOKEN"); err == nil && githubToken != "" {
		courierRunner = connectors.NewCourierRunner(githubToken, repoSlug, database, 0)
		// Set external URL so GitHub Actions can POST results back via cloudflare tunnel
		if cfg.System.Courier != nil && cfg.System.Courier.GovernorExternalURL != "" {
			courierRunner.SetGovernorURL(cfg.System.Courier.GovernorExternalURL)
			log.Printf("[Courier] Runner initialized for %s (callback via %s)", repoSlug, cfg.System.Courier.GovernorExternalURL)
		} else {
			log.Printf("[Courier] Runner initialized for %s (callback via localhost -- GitHub Actions dispatch will fail without external URL)", repoSlug)
		}
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

	// Refresh code map on startup if configured
	if cfg.System.CodeMap != nil && cfg.System.CodeMap.RefreshOnStartup {
		go func() {
			// Small delay to let MCP servers finish connecting
			time.Sleep(2 * time.Second)
			if mcpRegistry != nil && mcpRegistry.HasTool("index_folder") {
				log.Println("[CodeMap] Refreshing code map via jcodemunch...")
				_, err := mcpRegistry.CallTool(ctx, "index_folder", map[string]any{
					"path": repoPath,
				})
				if err != nil {
					log.Printf("[CodeMap] Refresh failed: %v (will use existing map.md)", err)
				} else {
					contextBuilder.InvalidateCache()
					log.Println("[CodeMap] Refresh complete, cache invalidated")
				}
			} else {
				log.Println("[CodeMap] jcodemunch index_folder tool not available, skipping refresh")
			}
		}()
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

	// Initialize PG Notify listener for local Postgres (replaces Supabase Realtime).
	// Listens on vp_changes channel, routes domain events internally,
	// and broadcasts generic notifications to SSE clients for dashboard live updates.
	var pgListener *pgnotify.Listener
	sseBroker := webhooks.NewSSEBroker()
	if dbType == "postgres" {
		pgURL := cfg.GetPostgresURL()
		if pgURL != "" {
			var err error
			pgListener, err = pgnotify.NewListener(ctx, pgURL, eventRouter, sseBroker)
			if err != nil {
				log.Printf("Warning: Failed to start PG Notify listener: %v", err)
			} else {
				log.Println("PG Notify listener started on vp_changes")
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
	webhookServer.SetDB(database)
	webhookServer.SetSSEBroker(sseBroker)
	webhookServer.SetVault(v)
	webhookServer.SetConfigDir(configDir)
	webhookServer.SetCreditTracker(usageTracker)

	// Admin token for vault API — read from env var (configurable in system.json).
	// If not set, vault API endpoints are disabled (403).
	if adminToken := os.Getenv("GOVERNOR_ADMIN_TOKEN"); adminToken != "" {
		webhookServer.SetAdminToken(adminToken)
		log.Println("[Admin] Vault API enabled (admin token set)")
	} else {
		log.Println("[Admin] Vault API disabled (set GOVERNOR_ADMIN_TOKEN to enable)")
	}

	// Courier result handler: writes to task_runs and notifies waiting goroutine
	if courierRunner != nil {
		webhookServer.SetCourierResultFn(func(taskID string, rawJSON json.RawMessage) error {
			var record struct {
				TaskID    string `json:"task_id"`
				Status    string `json:"status"`
				Output    string `json:"output"`
				Error     string `json:"error"`
				TokensIn  int    `json:"tokens_in"`
				TokensOut int    `json:"tokens_out"`
			}
			if err := json.Unmarshal(rawJSON, &record); err != nil {
				return fmt.Errorf("parse courier result: %w", err)
			}

			// Estimate tokens from output if courier didn't report exact counts
			// (web platforms like ChatGPT/Gemini web don't expose token counts)
			if record.TokensIn == 0 && record.TokensOut == 0 && len(record.Output) > 0 {
				// ~4 chars per token for GPT-style tokenizers
				record.TokensOut = len(record.Output) / 4
				log.Printf("[CourierResult] Estimated %d output tokens for %s (no exact counts from platform)", record.TokensOut, taskID)
			}

			// Write to task_runs table
			resultJSON, _ := json.Marshal(map[string]any{
				"output":     record.Output,
				"tokens_in":  record.TokensIn,
				"tokens_out": record.TokensOut,
			})
			params := map[string]any{
				"p_task_id":    taskID,
				"p_status":     record.Status,
				"p_result":     string(resultJSON),
				"p_error":      record.Error,
				"p_tokens_in":  record.TokensIn,
				"p_tokens_out": record.TokensOut,
			}
			if _, err := database.RPC(ctx, "record_courier_result", params); err != nil {
				log.Printf("[CourierResult] DB write failed: %v", err)
				// Don't block the notification — still deliver to waiting goroutine
			}

			courierRunner.NotifyResult(taskID, &connectors.TaskRunResult{
				Status:    record.Status,
				Output:    record.Output,
				Error:     record.Error,
				TokensIn:  record.TokensIn,
				TokensOut: record.TokensOut,
			})
			return nil
		})
	}

	githubHandler := webhooks.NewGitHubWebhookHandler(database, cfg.GetRepoPath())
	webhookServer.SetGitHubHandler(githubHandler)

	// Initialize Visual QA agent if configured
	vqaConfigPath := filepath.Join(configDir, "visualqa.json")
	if vqaCfgData, err := os.ReadFile(vqaConfigPath); err == nil {
		var vqaCfg visualqa.Config
		if json.Unmarshal(vqaCfgData, &vqaCfg) == nil && vqaCfg.Enabled {
			// Get Gemini API key from vault
			geminiKey := ""
			vaultKey := vqaCfg.GeminiVaultKey
			if vaultKey == "" {
				vaultKey = "GEMINI_GENERAL_KEY"
			}
			if k, err := v.GetSecret(ctx, vaultKey); err == nil && k != "" {
				geminiKey = k
			}

			// Get OpenRouter API key from vault for fallback vision providers
			if k, err := v.GetSecret(ctx, "OPENROUTER_API_KEY"); err == nil && k != "" {
				vqaCfg.OpenRouterKey = k
			}

			// Set repo path from managed repo
			vqaCfg.RepoPath = repoPath
			// Set fix engine source root if not explicitly configured
			if vqaCfg.FixConfig.SourceRoot == "" && repoPath != "" {
				vqaCfg.FixConfig.SourceRoot = repoPath
			}

			// Create DB adapter using the same postgres URL
			pgURL := cfg.GetPostgresURL()
			vqaDB, err := visualqa.NewPGXDB(ctx, pgURL)
			if err != nil {
				log.Printf("[VisualQA] WARNING: Failed to create DB adapter: %v (VQA disabled)", err)
			} else {
				vqa := visualqa.NewVisualQA(vqaCfg, geminiKey, vqaDB)
				webhookServer.SetVisualQA(&visualQAServerAdapter{inner: vqa})
				log.Printf("[VisualQA] Enabled: %d pages, model=%s, baselines=%s", len(vqaCfg.CapturePages), vqaCfg.Model, vqaCfg.BaselineDir)
			}
		}
	} else {
		log.Printf("[VisualQA] No config file at %s: %v", vqaConfigPath, err)
	}

	// ==========================================================================
	// DesignPreview: Pre-execution design review gate for UI tasks.
	// Generates HTML mockups via Gemini, human approves before code is written.
	// ==========================================================================
	dpConfigPath := filepath.Join(configDir, "designpreview.json")
	if dpCfgData, err := os.ReadFile(dpConfigPath); err == nil {
		var dpCfg designpreview.Config
		if err := json.Unmarshal(dpCfgData, &dpCfg); err == nil && dpCfg.Enabled {
			dpAPIKey := ""
			if dpCfg.GeminiVaultKey != "" {
				if key, err := v.GetSecret(ctx, dpCfg.GeminiVaultKey); err == nil {
					dpAPIKey = key
				}
			}
			if dpAPIKey != "" {
				// Ensure output dir is within the repo
				if dpCfg.OutputDir != "" {
					dpCfg.OutputDir = filepath.Join(cfg.GetRepoPath(), dpCfg.OutputDir)
				}
				if err := designpreview.EnsureDBSchema(); err != nil {
					log.Printf("[DesignPreview] WARNING: Schema creation failed: %v", err)
				}
				dp := designpreview.NewDesignPreview(dpCfg, dpAPIKey, &dpDBAdapter{db: database})
				webhookServer.SetDesignPreview(&dpServerAdapter{inner: dp})
				log.Println("[DesignPreview] Enabled, Gemini model:", dpCfg.GeminiModel)
			} else {
				log.Printf("[DesignPreview] Disabled: Gemini API key not available")
			}
		}
	} else {
		log.Printf("[DesignPreview] No config file at %s: %v", dpConfigPath, err)
	}

	go runProcessingRecovery(ctx, database, cfg)

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, connRouter, git, stateMachine, checkpointMgr, leakDetector, usageTracker, worktreeMgr, courierRunner, v, configDir, contextBuilder)

	// Start model scanner if configured
	if cfg.System.ModelScanner != nil && cfg.System.ModelScanner.Enabled {
		modelScanner := runtime.NewModelScanner(cfg.System.ModelScanner, database, v)
		modelScanner.StartBackgroundScanner(ctx)
		log.Println("[ModelScanner] Enabled and started")
	}

	// Start credit poller for paid API providers (DeepSeek, etc.)
	if v2tracker := usageTracker.GetPlatformTrackerV2(); v2tracker != nil && v != nil {
		creditPoller := runtime.NewCreditPoller(v2tracker, v)
		creditPoller.StartBackgroundPolling(ctx)
		log.Println("[CreditPoller] Started background credit polling")
	}

	// Usage state is persisted only on shutdown (no polling).
	// Dashboard reads model data via realtime subscriptions, not polled tables.
	// This goroutine waits for shutdown and does one final persist.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		usageTracker.PersistToDatabase(shutdownCtx)
		cancel()
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
	if pgListener != nil {
		pgListener.Close()
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

// runVaultCLI handles: ./governor vault <set|get|list|delete|rotate-key> [args...]
// Connects to the same database, uses the same vault encryption.
// Requires DATABASE_URL and VAULT_KEY env vars.
func runVaultCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: governor vault <command> [args...]")
		fmt.Println("Commands:")
		fmt.Println("  set <KEY_NAME> <value>    Encrypt and store a secret")
		fmt.Println("  get <KEY_NAME>            Decrypt and print a secret")
		fmt.Println("  list                      List all key names in vault")
		fmt.Println("  delete <KEY_NAME>         Delete a secret from vault")
		fmt.Println("  rotate-key <NEW_KEY>      Re-encrypt all secrets with new master key")
		os.Exit(1)
	}

	ctx := context.Background()

	// Bootstrap: config tells us where to find env vars
	configDir := getConfigDir()
	cfg, err := runtime.LoadConfig(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Connect to database
	dbType := cfg.GetDatabaseType()
	if dbType != "postgres" {
		fmt.Fprintf(os.Stderr, "Vault CLI requires postgres backend (current: %s)\n", dbType)
		os.Exit(1)
	}

	pgURL := cfg.GetPostgresURL()
	if pgURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		os.Exit(1)
	}

	database, err := db.NewPostgres(ctx, pgURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	v := vault.NewWithoutAudit(database)
	v.InitVaultKey(cfg.GetVaultKeyEnv())

	command := args[0]
	switch command {
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: governor vault set <KEY_NAME> <value>")
			os.Exit(1)
		}
		if err := v.StoreSecret(ctx, args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Stored %s\n", args[1])

	case "get":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: governor vault get <KEY_NAME>")
			os.Exit(1)
		}
		val, err := v.GetSecretNoCache(ctx, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(val)

	case "list":
		names, err := v.ListSecrets(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, n := range names {
			fmt.Println(n)
		}
		fmt.Printf("\n%d secret(s)\n", len(names))

	case "delete":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: governor vault delete <KEY_NAME>")
			os.Exit(1)
		}
		if err := v.DeleteSecret(ctx, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Deleted %s\n", args[1])

	case "rotate-key":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: governor vault rotate-key <NEW_VAULT_KEY>")
			os.Exit(1)
		}
		count, err := v.RotateKey(ctx, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Rotated %d secret(s) to new master key\n", count)
		fmt.Println("IMPORTANT: Update VAULT_KEY env var before next governor start")

	default:
		fmt.Fprintf(os.Stderr, "Unknown vault command: %s\n", command)
		os.Exit(1)
	}
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

	// Run startup health checks on connectors that support it
	healthyCount := 0
	hcCtx := context.Background()
	for _, conn := range connectorsCfg.Connectors {
		if conn.Status != "active" {
			continue
		}
		runner, ok := factory.GetConnector(conn.ID)
		if !ok || runner == nil {
			continue
		}
		if hc, ok := runner.(runtime.HealthChecker); ok {
			if err := hc.HealthCheck(hcCtx); err != nil {
				log.Printf("HEALTH CHECK FAILED: %s — %v (connector remains registered but may fail at runtime)", conn.ID, err)
			} else {
				healthyCount++
				log.Printf("HEALTH CHECK OK: %s", conn.ID)
			}
		}
	}
	log.Printf("Startup health checks: %d/%d API connectors healthy", healthyCount, activeCount)

	if activeCount == 0 {
		log.Println("Warning: no active connectors registered")
	}
}

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, connRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v *vault.Vault, configDir string, contextBuilder *runtime.ContextBuilder) {
	setupTaskHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, checkpointMgr, leakDetector, usageTracker, worktreeMgr, courierRunner, v, contextBuilder)
	setupPlanHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, usageTracker)
	setupCouncilHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, usageTracker)
	setupMaintenanceHandler(ctx, router, factory, pool, database, cfg, connRouter, git, worktreeMgr, usageTracker)
	setupTestingHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, worktreeMgr, usageTracker)
	actionApplier := runtime.NewResearchActionApplier(configDir, database)
	setupResearchHandlers(ctx, router, factory, pool, database, cfg, connRouter, usageTracker, actionApplier)

	// Courier result handler (pgnotify path): backup delivery channel for waiting goroutines.
	// The primary path is the webhook callback in SetCourierResultFn. This pgnotify handler
	// fires after the webhook writes to task_runs, which triggers the notify trigger.
	// pgnotify sets Record to nil, so we must query the DB to get the result data.
	if courierRunner != nil {
		router.On(runtime.EventCourierResult, func(event runtime.Event) {
			// pgnotify always sets Record to nil — query DB for the latest task_run result
			if event.ID == "" {
				return
			}
			rows, err := database.Query(context.Background(), "task_runs", map[string]any{
				"task_id": event.ID,
				"order":   "completed_at.desc",
				"limit":   1,
				"select":  "status,result,error,tokens_in,tokens_out",
			})
			if err != nil || len(rows) == 0 {
				log.Printf("[CourierResult-pgnotify] No task_run found for task %s", event.ID)
				return
			}
			var runs []struct {
				Status    string          `json:"status"`
				Result    json.RawMessage `json:"result"`
				Error     string          `json:"error"`
				TokensIn  int             `json:"tokens_in"`
				TokensOut int             `json:"tokens_out"`
			}
			if err := json.Unmarshal(rows, &runs); err != nil || len(runs) == 0 {
				log.Printf("[CourierResult-pgnotify] Parse error for task %s: %v", event.ID, err)
				return
			}
		run := runs[0]
		// Ignore non-terminal statuses — the initial INSERT (status="running")
		// triggers this same handler via pg_notify, which would prematurely
		// deliver an empty result to the waiting goroutine before the
		// GitHub Actions workflow even starts.
		if run.Status != "success" && run.Status != "failed" {
			return
		}
		output := ""
		if len(run.Result) > 0 {
			var r struct {
				Output string `json:"output"`
			}
			if json.Unmarshal(run.Result, &r) == nil {
				output = r.Output
			}
		}
		courierRunner.NotifyResult(event.ID, &connectors.TaskRunResult{
				Status:    run.Status,
				Output:    output,
				Error:     run.Error,
				TokensIn:  run.TokensIn,
				TokensOut: run.TokensOut,
			})
		})
	}
}

// visualQAServerAdapter adapts visualqa.VisualQA to webhooks.VisualQARunner interface.
type visualQAServerAdapter struct {
	inner *visualqa.VisualQA
}

func (a *visualQAServerAdapter) Run(ctx context.Context, triggeredBy, triggerDetail string) (any, error) {
	return a.inner.Run(ctx, triggeredBy, triggerDetail)
}

func (a *visualQAServerAdapter) GetEnabled() bool {
	return a.inner.GetEnabled()
}

func (a *visualQAServerAdapter) ApproveBaseline(ctx context.Context, pageName string, viewportWidth int) error {
	return a.inner.ApproveBaseline(ctx, pageName, viewportWidth)
}

func (a *visualQAServerAdapter) RecordIssueFeedback(ctx context.Context, runID, issueType, issueElement, issueDescription string, viewport int, verdict, userNote, patternKey string) error {
	return a.inner.RecordIssueFeedback(ctx, runID, issueType, issueElement, issueDescription, viewport, verdict, userNote, patternKey)
}

func (a *visualQAServerAdapter) GetIssueFeedback(ctx context.Context) ([]map[string]any, error) {
	return a.inner.GetIssueFeedback(ctx)
}

// vqaDBAdapter is no longer needed - VQA uses its own pgx connection.
// Keeping the type for compatibility with any remaining references.
type vqaDBAdapter struct {
	db db.Database
}

// dpDBAdapter adapts db.Database to designpreview.DB interface.
type dpDBAdapter struct {
	db db.Database
}

func (a *dpDBAdapter) Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error) {
	return a.db.Insert(ctx, table, data)
}

func (a *dpDBAdapter) Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error) {
	return a.db.Update(ctx, table, id, data)
}

func (a *dpDBAdapter) Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error) {
	return a.db.Query(ctx, table, filters)
}

// dpServerAdapter adapts designpreview.DesignPreview to webhooks.DesignPreviewer interface.
type dpServerAdapter struct {
	inner *designpreview.DesignPreview
}

func (a *dpServerAdapter) Generate(ctx context.Context, req webhooks.DesignPreviewRequest) (*webhooks.DesignPreviewResponse, error) {
	resp, err := a.inner.Generate(ctx, designpreview.GenerateRequest{
		TaskID:      req.TaskID,
		Title:       req.Title,
		Description: req.Description,
		TargetURL:   req.TargetURL,
		DesignHints: req.DesignHints,
	})
	if err != nil {
		return nil, err
	}
	return &webhooks.DesignPreviewResponse{
		PreviewID:   resp.PreviewID,
		HTMLContent: resp.HTMLContent,
		FilePath:    resp.FilePath,
		Model:       resp.Model,
		TokensUsed:  resp.TokensUsed,
	}, nil
}

func (a *dpServerAdapter) Approve(ctx context.Context, previewID, reviewer, notes string) error {
	return a.inner.Approve(ctx, previewID, reviewer, notes)
}

func (a *dpServerAdapter) Reject(ctx context.Context, previewID, reviewer, notes string) error {
	return a.inner.Reject(ctx, previewID, reviewer, notes)
}

func (a *dpServerAdapter) GetEnabled() bool {
	return a.inner.GetEnabled()
}
