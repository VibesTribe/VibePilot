package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/destinations"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/tools"
	"github.com/vibepilot/governor/internal/vault"
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

	repoPath := getEnvOrDefault("REPO_PATH", ".")
	protectedBranches := cfg.GetProtectedBranches()

	git := gitree.New(&gitree.Config{
		RepoPath:          repoPath,
		ProtectedBranches: protectedBranches,
	})

	v := vault.New(database)

	toolRegistry := runtime.NewToolRegistry(cfg)
	tools.RegisterAll(toolRegistry, &tools.Dependencies{
		DB:       database,
		Git:      git,
		Vault:    v,
		RepoPath: repoPath,
		Config:   cfg,
	})

	sessionFactory := runtime.NewSessionFactory(cfg, toolRegistry)
	registerDestinations(sessionFactory, cfg, v)

	pool := runtime.NewAgentPoolWithConcurrency(
		cfg.System.Runtime.MaxConcurrentPerModule,
		cfg.System.Runtime.MaxConcurrentTotal,
		&cfg.System.Concurrency,
	)

	pollInterval := time.Duration(cfg.System.Runtime.EventPollIntervalMs) * time.Millisecond
	watcher := runtime.NewPollingWatcher(&dbQuerierAdapter{db: database, cfg: cfg}, pollInterval)

	router := runtime.NewEventRouter(watcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupEventHandlers(ctx, router, sessionFactory, pool, database, cfg)

	if err := router.Start(ctx); err != nil {
		log.Fatalf("Failed to start event router: %v", err)
	}

	log.Printf("Governor started (poll: %v, max/module: %d, max total: %d, opencode limit: %d)",
		pollInterval, cfg.System.Runtime.MaxConcurrentPerModule, cfg.System.Runtime.MaxConcurrentTotal, cfg.System.Concurrency.GetLimit("opencode"))
	log.Println("Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()
	watcher.Close()
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

func registerDestinations(factory *runtime.SessionFactory, cfg *runtime.Config, v *vault.Vault) {
	destinationsCfg := cfg.Destinations
	if destinationsCfg == nil {
		log.Println("Warning: no destinations configured")
		return
	}

	secretProvider := destinations.NewVaultAdapter(v)

	for _, dest := range destinationsCfg.Destinations {
		if dest.Status != "active" {
			log.Printf("Skipping inactive destination: %s", dest.ID)
			continue
		}

		switch dest.Type {
		case "cli":
			timeout := destinations.DefaultTimeoutSecs
			if dest.TimeoutSeconds > 0 {
				timeout = dest.TimeoutSeconds
			}
			runner := destinations.NewCLIRunnerWithArgs(dest.Command, dest.CLIArgs, timeout)
			factory.RegisterDestination(dest.ID, runner)
			log.Printf("Registered CLI destination: %s (%s)", dest.ID, dest.Command)
		case "api":
			runner := destinations.NewAPIRunnerFromConfig(dest, secretProvider)
			factory.RegisterDestination(dest.ID, runner)
			log.Printf("Registered API destination: %s (%s)", dest.ID, dest.Endpoint)
		default:
			log.Printf("Unknown destination type: %s for %s", dest.Type, dest.ID)
		}
	}
}

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config) {
	eventsCfg := cfg.System.Events

	router.On(runtime.EventTaskAvailable, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			log.Printf("[EventTaskAvailable] Failed to parse task: %v", err)
			return
		}

		taskID, _ := task["id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "default"
		}

		session, err := factory.Create("orchestrator")
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to create orchestrator session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_available"})
			if err != nil {
				log.Printf("[EventTaskAvailable] Orchestrator session failed for %s: %v", truncateID(taskID), err)
				return err
			}
			log.Printf("[EventTaskAvailable] Task %s routed: %s", truncateID(taskID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskReview, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "review"
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			log.Printf("[EventTaskReview] Failed to create supervisor session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_review"})
			if err != nil {
				return err
			}
			log.Printf("[EventTaskReview] Task %s reviewed: %s", truncateID(taskID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventTaskReview] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskCompleted, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "complete"
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_completed"})
			if err != nil {
				return err
			}
			log.Printf("[EventTaskCompleted] Task %s completed: %s", truncateID(taskID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventTaskCompleted] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPlanCreated, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_created"})
			if err != nil {
				return err
			}
			log.Printf("[EventPlanCreated] Plan %s triaged: %s", truncateID(planID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventPlanCreated] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventCouncilDone, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "council_done"})
			if err != nil {
				return err
			}
			log.Printf("[EventCouncilDone] Council done for %s: %s", truncateID(planID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventCouncilDone] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventMaintenanceCmd, func(event runtime.Event) {
		var cmd map[string]any
		if err := json.Unmarshal(event.Record, &cmd); err != nil {
			return
		}

		cmdID, _ := cmd["id"].(string)

		session, err := factory.Create("maintenance")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "maintenance", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"command": cmd, "event": "maintenance_command"})
			if err != nil {
				return err
			}
			log.Printf("[EventMaintenanceCmd] Command %s executed: %s", truncateID(cmdID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventMaintenanceCmd] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventResearchReady, func(event runtime.Event) {
		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "research", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"event": "research_ready", "record": string(event.Record)})
			if err != nil {
				return err
			}
			log.Printf("[EventResearchReady] Research reviewed: %s", truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventResearchReady] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPRDReady, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPRDReady] Failed to parse plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)

		session, err := factory.Create("planner")
		if err != nil {
			log.Printf("[EventPRDReady] Failed to create planner session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "prd_ready"})
			if err != nil {
				log.Printf("[EventPRDReady] Planner session failed for %s: %v", truncateID(planID), err)
				return err
			}
			log.Printf("[EventPRDReady] Plan %s planned: %s", truncateID(planID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventPRDReady] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPlanReview, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPlanReview] Failed to parse plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)

		session, err := factory.Create("supervisor")
		if err != nil {
			log.Printf("[EventPlanReview] Failed to create supervisor session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_review"})
			if err != nil {
				log.Printf("[EventPlanReview] Supervisor session failed for %s: %v", truncateID(planID), err)
				return err
			}
			log.Printf("[EventPlanReview] Plan %s reviewed: %s", truncateID(planID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventPlanReview] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTestResults, func(event runtime.Event) {
		var testResult map[string]any
		if err := json.Unmarshal(event.Record, &testResult); err != nil {
			return
		}

		taskID, _ := testResult["task_id"].(string)

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "testing", "opencode", func() error {
			result, err := session.Run(ctx, map[string]any{"test_result": testResult, "event": "test_results"})
			if err != nil {
				return err
			}
			log.Printf("[EventTestResults] Task %s tests processed: %s", truncateID(taskID), truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventTestResults] Failed to submit to pool: %v", err)
		}
	})

	_ = eventsCfg
}

type dbQuerierAdapter struct {
	db  *db.DB
	cfg *runtime.Config
}

func (q *dbQuerierAdapter) Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error) {
	path := table

	if columns, ok := filters["columns"].([]any); ok && len(columns) > 0 {
		colStr := ""
		for i, c := range columns {
			if i > 0 {
				colStr += ","
			}
			colStr += toString(c)
		}
		path = table + "?select=" + colStr
	} else {
		path = table + "?select=*"
	}

	for key, val := range filters {
		if key == "columns" || key == "select" {
			continue
		}
		if orVal, ok := val.(string); ok && key == "or" {
			path = path + "&or=(" + orVal + ")"
		} else {
			path = path + "&" + key + "=eq." + toString(val)
		}
	}

	if limit, ok := filters["limit"].(float64); ok {
		path = path + "&limit=" + toString(int(limit))
	}

	return q.db.REST(ctx, "GET", path, nil)
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncateOutput(output string) string {
	if len(output) > 200 {
		return output[:200] + "..."
	}
	return output
}
