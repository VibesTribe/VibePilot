package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vibepilot/governor/internal/core"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/destinations"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/security"
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
	registerDestinations(sessionFactory, cfg, v)

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

	pollInterval := time.Duration(cfg.System.Runtime.EventPollIntervalMs) * time.Millisecond
	watcher := runtime.NewPollingWatcher(database, pollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recoveryCfg := getRecoveryConfig(cfg)
	runStartupRecovery(ctx, database, recoveryCfg)
	runCheckpointRecovery(ctx, database, cfg, checkpointMgr)

	watcher.SetConfig(cfg)

	destRouter := runtime.NewRouter(cfg, database)
	eventRouter := runtime.NewEventRouter(watcher)

	prdWatcher := runtime.NewPRDWatcher(database, runtime.PRDWatcherConfig{
		Enabled:   cfg.System.PRDWatcher.Enabled,
		RepoPath:  cfg.System.PRDWatcher.RepoPath,
		Branch:    cfg.System.PRDWatcher.Branch,
		Directory: cfg.System.PRDWatcher.Directory,
		Interval:  time.Duration(cfg.System.PRDWatcher.IntervalSeconds) * time.Second,
	})
	go prdWatcher.Start(ctx)

	go runProcessingRecovery(ctx, database, cfg)

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, destRouter, git, stateMachine, checkpointMgr, leakDetector)

	if err := eventRouter.Start(ctx); err != nil {
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

type dbCheckpointAdapter struct {
	db *db.DB
}

func (a *dbCheckpointAdapter) RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error) {
	result, err := a.db.RPC(ctx, fn, args)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (a *dbCheckpointAdapter) Save(ctx context.Context, taskID string, checkpoint core.Checkpoint) error {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Save(ctx, taskID, checkpoint)
}

func (a *dbCheckpointAdapter) Load(ctx context.Context, taskID string) (*core.Checkpoint, error) {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Load(ctx, taskID)
}

func (a *dbCheckpointAdapter) Delete(ctx context.Context, taskID string) error {
	storage := core.NewDBCheckpointStorage(a)
	return storage.Delete(ctx, taskID)
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
			timeout := cfg.GetRunnerTimeoutSecs()
			if dest.TimeoutSeconds > 0 {
				timeout = dest.TimeoutSeconds
			}
			cliArgs := dest.CLIArgs
			if len(cliArgs) == 0 {
				cliArgs = cfg.GetDefaultCLIArgs()
			}
			runner := destinations.NewCLIRunnerWithArgs(dest.Command, cliArgs, timeout)
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

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, destRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector) {
	eventsCfg := cfg.System.Events

	selectDestination := func(agentID, taskID, taskType string) string {
		result, err := destRouter.SelectDestination(ctx, runtime.RoutingRequest{
			AgentID:  agentID,
			TaskID:   taskID,
			TaskType: taskType,
		})
		if err != nil || result == nil {
			log.Printf("[Router] No destination available for agent %s, using fallback", agentID)
			dests := destRouter.GetAvailableDestinations()
			if len(dests) > 0 {
				return dests[0]
			}
			return ""
		}
		return result.DestinationID
	}

	router.On(runtime.EventTaskAvailable, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			log.Printf("[EventTaskAvailable] Failed to parse task: %v", err)
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		taskNumber, _ := task["task_number"].(string)
		taskCategory, _ := task["category"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "default"
		}

		processingBy := fmt.Sprintf("task_runner:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskAvailable] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskAvailable] Task %s already being processed", truncateID(taskID))
			return
		}

		taskPacket, err := database.GetTaskPacket(ctx, taskID)
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to fetch task packet for %s: %v", truncateID(taskID), err)
			_, _ = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "error",
			})
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		if taskPacket.Prompt == "" {
			log.Printf("[EventTaskAvailable] Task %s has empty prompt packet - cannot execute", truncateID(taskID))
			_, _ = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "error",
			})
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		log.Printf("[EventTaskAvailable] Task %s packet loaded: prompt_len=%d category=%s", truncateID(taskID), len(taskPacket.Prompt), taskCategory)

		taskPrefix := cfg.GetTaskBranchPrefix()
		if taskPrefix == "" {
			taskPrefix = "task/"
		}
		branchName := fmt.Sprintf("%s%s", taskPrefix, taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("%s%s", taskPrefix, truncateID(taskID))
		}

		if err := git.CreateBranch(ctx, branchName); err != nil {
			log.Printf("[EventTaskAvailable] Failed to create branch %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskAvailable] Created branch %s for task %s", branchName, truncateID(taskID))
		}

		_, err = database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "in_progress",
		})
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to update status to in_progress: %v", err)
		}

		destID := selectDestination("task_runner", taskID, taskCategory)
		if destID == "" {
			destID = selectDestination("task_runner", taskID, taskType)
		}
		if destID == "" {
			log.Printf("[EventTaskAvailable] No destination available for task %s (category=%s, type=%s)", truncateID(taskID), taskCategory, taskType)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "task_runner", taskCategory)
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to create task_runner session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		if cfg.GetCoreConfig().IsCheckpointEnabled() {
			_, err := database.RPC(ctx, "save_checkpoint", map[string]any{
				"p_task_id":  taskID,
				"p_step":     "execution",
				"p_progress": 0,
				"p_output":   "",
				"p_files":    []string{},
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Warning: Failed to save initial checkpoint: %v", err)
			} else {
				log.Printf("[EventTaskAvailable] Checkpoint saved for task %s", truncateID(taskID))
			}
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})

			var contextData map[string]any
			if len(taskPacket.Context) > 0 {
				json.Unmarshal(taskPacket.Context, &contextData)
			}

			result, err := session.Run(ctx, map[string]any{
				"task_id":         taskID,
				"task_number":     taskNumber,
				"title":           task["title"],
				"type":            taskType,
				"category":        taskCategory,
				"prompt_packet":   taskPacket.Prompt,
				"expected_output": taskPacket.ExpectedOutput,
				"context":         contextData,
				"dependencies":    task["dependencies"],
				"event":           "task_available",
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Task runner failed for %s: %v", truncateID(taskID), err)
				return err
			}

			cleanOutput, leaks := leakDetector.Scan(result.Output)
			if len(leaks) > 0 {
				log.Printf("[EventTaskAvailable] SECURITY: %d leak(s) detected and redacted in task %s output", len(leaks), truncateID(taskID))
			}

			runnerOutput := map[string]any{
				"raw_output": cleanOutput,
				"model_id":   "unknown",
				"task_id":    taskID,
				"duration":   result.Duration.Seconds(),
			}

			taskOutput, parseErr := runtime.ParseTaskRunnerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventTaskAvailable] Failed to parse runner output: %v", parseErr)
				runnerOutput["parse_error"] = parseErr.Error()
			} else {
				runnerOutput["files"] = taskOutput.Files
				runnerOutput["summary"] = taskOutput.Summary
				runnerOutput["status"] = taskOutput.Status
			}

			if err := git.CommitOutput(ctx, branchName, runnerOutput); err != nil {
				log.Printf("[EventTaskAvailable] Failed to commit output to %s: %v", branchName, err)
			} else {
				log.Printf("[EventTaskAvailable] Committed output to %s", branchName)
			}

			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "review",
			})
			if err != nil {
				log.Printf("[EventTaskAvailable] Failed to update status to review: %v", err)
			}

			if cfg.GetCoreConfig().IsCheckpointEnabled() {
				_, delErr := database.RPC(ctx, "delete_checkpoint", map[string]any{
					"p_task_id": taskID,
				})
				if delErr != nil {
					log.Printf("[EventTaskAvailable] Warning: Failed to delete checkpoint: %v", delErr)
				}
			}

			log.Printf("[EventTaskAvailable] Task %s output committed, status=review", truncateID(taskID))
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			log.Printf("[EventTaskAvailable] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskReview, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "review"
		}

		processingBy := fmt.Sprintf("supervisor_review:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskReview] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskReview] Task %s already being processed", truncateID(taskID))
			return
		}

		destID := selectDestination("supervisor", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskReview] No destination available for task %s", truncateID(taskID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
		if err != nil {
			log.Printf("[EventTaskReview] Failed to create supervisor session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})

			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_review"})
			if err != nil {
				return err
			}

			decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
			if parseErr != nil {
				log.Printf("[EventTaskReview] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
				log.Printf("[EventTaskReview] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventTaskReview] Task %s decision: %s, next: %s", truncateID(taskID), decision.Decision, decision.NextAction)

			switch decision.Decision {
			case "pass":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "testing",
				})
				if err != nil {
					log.Printf("[EventTaskReview] Failed to update task status to testing: %v", err)
				}

			case "fail":
				for _, issue := range decision.Issues {
					failureCategory := runtime.CategorizeFailure(issue.Type)
					_, err := database.RPC(ctx, "record_failure", map[string]any{
						"p_task_id":          taskID,
						"p_failure_type":     issue.Type,
						"p_failure_category": failureCategory,
						"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
						"p_model_id":         modelID,
						"p_task_type":        taskType,
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to record failure: %v", err)
					}
				}

				switch decision.NextAction {
				case "return_to_runner":
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "available",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to reset task to available: %v", err)
					}

				case "split_task", "escalate":
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "escalated",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to escalate task: %v", err)
					}

				default:
					_, err := database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "available",
					})
					if err != nil {
						log.Printf("[EventTaskReview] Failed to reset task: %v", err)
					}
				}

			case "reroute":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
				if err != nil {
					log.Printf("[EventTaskReview] Failed to reroute task: %v", err)
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			log.Printf("[EventTaskReview] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTaskCompleted, func(event runtime.Event) {
		var task map[string]any
		if err := json.Unmarshal(event.Record, &task); err != nil {
			return
		}

		taskID, _ := task["id"].(string)
		taskType, _ := task["type"].(string)
		taskNumber, _ := task["task_number"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "complete"
		}

		processingBy := fmt.Sprintf("supervisor_completed:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "tasks",
			"p_id":            taskID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventTaskCompleted] Task %s already being processed or claim failed", truncateID(taskID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTaskCompleted] Task %s already being processed", truncateID(taskID))
			return
		}

		branchName := fmt.Sprintf("task/%s", taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("task/%s", truncateID(taskID))
		}

		destID := selectDestination("supervisor", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskCompleted] No destination available for task %s", truncateID(taskID))
			recordModelFailure(ctx, database, modelID, taskID, "no_destination")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
		if err != nil {
			recordModelFailure(ctx, database, modelID, taskID, "session_create_failed")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{"task": task, "event": "task_completed"})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[EventTaskCompleted] Supervisor session failed for %s: %v", truncateID(taskID), sessionErr)
			recordModelFailure(ctx, database, modelID, taskID, "session_error")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
			return
		}

		decision, parseErr := runtime.ParseSupervisorDecision(result.Output)
		if parseErr != nil {
			log.Printf("[EventTaskCompleted] Failed to parse decision for %s: %v", truncateID(taskID), parseErr)
			log.Printf("[EventTaskCompleted] Raw output: %s", truncateOutput(result.Output))
			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
			if err != nil {
				log.Printf("[EventTaskCompleted] Failed to escalate task: %v", err)
			}
			return
		}

		log.Printf("[EventTaskCompleted] Task %s decision: %s, next: %s", truncateID(taskID), decision.Decision, decision.NextAction)

		output := map[string]any{
			"output":     result.Output,
			"model_id":   modelID,
			"task_id":    taskID,
			"duration":   duration,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
			"decision":   decision.Decision,
		}
		if err := git.CommitOutput(ctx, branchName, output); err != nil {
			log.Printf("[EventTaskCompleted] Failed to commit output to %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskCompleted] Committed output to %s", branchName)
		}

		switch decision.Decision {
		case "pass":
			if decision.NextAction == "final_merge" {
				targetBranch := cfg.GetDefaultMergeTarget()
				if err := git.MergeBranch(ctx, branchName, targetBranch); err != nil {
					log.Printf("[EventTaskCompleted] Failed to merge %s to %s: %v", branchName, targetBranch, err)
					_, err = database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "escalated",
					})
				} else {
					log.Printf("[EventTaskCompleted] Merged %s to %s", branchName, targetBranch)
					_, err = database.RPC(ctx, "update_task_status", map[string]any{
						"p_task_id": taskID,
						"p_status":  "merged",
					})
					git.DeleteBranch(ctx, branchName)
				}
			} else {
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "approval",
				})
			}
			recordModelSuccess(ctx, database, modelID, taskType, duration)

		case "fail":
			for _, issue := range decision.Issues {
				failureCategory := runtime.CategorizeFailure(issue.Type)
				_, err := database.RPC(ctx, "record_failure", map[string]any{
					"p_task_id":          taskID,
					"p_failure_type":     issue.Type,
					"p_failure_category": failureCategory,
					"p_failure_details":  map[string]any{"description": issue.Description, "severity": issue.Severity},
					"p_model_id":         modelID,
					"p_task_type":        taskType,
				})
				if err != nil {
					log.Printf("[EventTaskCompleted] Failed to record failure: %v", err)
				}
			}

			switch decision.NextAction {
			case "return_to_runner":
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
			default:
				_, err = database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "escalated",
				})
			}
			recordModelFailure(ctx, database, modelID, taskID, decision.NextAction)

		default:
			_, err = database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "escalated",
			})
		}

		if err != nil {
			log.Printf("[EventTaskCompleted] Failed to update task status: %v", err)
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "tasks", "p_id": taskID})
	})

	router.On(runtime.EventPlanCreated, func(event runtime.Event) {
		startTime := time.Now()
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		currentStatus, _ := plan["status"].(string)

		processingBy := fmt.Sprintf("plan_created:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventPlanCreated] Plan %s already being processed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPlanCreated] Plan %s already being processed", truncateID(planID))
			return
		}

		destID := selectDestination("supervisor", planID, "plan_review")
		if destID == "" {
			log.Printf("[EventPlanCreated] No destination available for plan %s", truncateID(planID))
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "no_destination")
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "session_creation_failed")
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "review", "plan_review_started")

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_created"})
			if err != nil {
				database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), false, map[string]any{"error": err.Error()})
				return err
			}

			database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), true, nil)
			log.Printf("[EventPlanCreated] Plan %s triaged via %s: %s", truncateID(planID), destID, truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "pool_submit_failed")
			log.Printf("[EventPlanCreated] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventCouncilDone, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("council_done:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventCouncilDone] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventCouncilDone] Plan %s already being processed", truncateID(planID))
			return
		}

		councilReviews := extractCouncilReviews(plan)

		if len(councilReviews) == 0 {
			log.Printf("[EventCouncilDone] No council reviews for plan %s - direct supervisor approval, creating tasks", truncateID(planID))
			if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
				var validationErr *ValidationFailedError
				if errors.As(err, &validationErr) {
					log.Printf("[EventCouncilDone] Task validation failed for plan %s - sending back to planner", truncateID(planID))
					var concerns []string
					var taskNumbers []string
					for _, e := range validationErr.Errors {
						concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
						taskNumbers = append(taskNumbers, e.TaskNumber)
					}
					_, _ = database.RPC(ctx, "record_planner_revision", map[string]any{
						"p_plan_id":                planID,
						"p_concerns":               concerns,
						"p_tasks_needing_revision": taskNumbers,
					})
					_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
						"p_plan_id":      planID,
						"p_status":       "revision_needed",
						"p_review_notes": map[string]any{"validation_errors": concerns},
					})
				} else {
					log.Printf("[EventCouncilDone] Failed to create tasks: %v", err)
					_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
						"p_plan_id":      planID,
						"p_status":       "error",
						"p_review_notes": map[string]any{"error": err.Error()},
					})
				}
			} else {
				_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
					"p_plan_id": planID,
					"p_status":  "approved",
				})
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		destID := selectDestination("supervisor", planID, "council_done")
		if destID == "" {
			log.Printf("[EventCouncilDone] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			_, err := session.Run(ctx, map[string]any{"plan": plan, "event": "council_done"})
			if err != nil {
				return err
			}

			approved := 0
			revisionNeeded := 0
			blocked := 0

			for _, r := range councilReviews {
				vote, _ := r["vote"].(string)
				switch vote {
				case "APPROVED":
					approved++
				case "REVISION_NEEDED":
					revisionNeeded++
				case "BLOCKED":
					blocked++
				}
			}

			memberCount := cfg.GetCouncilMemberCount()
			consensusMethod := cfg.GetConsensusMethod()

			var consensus string
			if consensusMethod == "unanimous_approval" {
				if approved == memberCount {
					consensus = "approved"
				} else if blocked > 0 {
					consensus = "blocked"
				} else {
					consensus = "revision_needed"
				}
			} else {
				if approved == memberCount {
					consensus = "approved"
				} else if blocked > 0 {
					consensus = "blocked"
				} else {
					consensus = "revision_needed"
				}
			}

			log.Printf("[EventCouncilDone] Plan %s consensus: %s (approved=%d, revision=%d, blocked=%d, method=%s)", truncateID(planID), consensus, approved, revisionNeeded, blocked, consensusMethod)

			_, err = database.RPC(ctx, "set_council_consensus", map[string]any{
				"p_plan_id":   planID,
				"p_consensus": consensus,
			})
			if err != nil {
				log.Printf("[EventCouncilDone] Failed to set council consensus: %v", err)
			}

			switch consensus {
			case "approved":
				if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
					var validationErr *ValidationFailedError
					if errors.As(err, &validationErr) {
						log.Printf("[EventCouncilDone] Task validation failed for plan %s - sending back to planner", truncateID(planID))
						var concerns []string
						var taskNumbers []string
						for _, e := range validationErr.Errors {
							concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
							taskNumbers = append(taskNumbers, e.TaskNumber)
						}
						_, _ = database.RPC(ctx, "record_planner_revision", map[string]any{
							"p_plan_id":                planID,
							"p_concerns":               concerns,
							"p_tasks_needing_revision": taskNumbers,
						})
						_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
							"p_plan_id":      planID,
							"p_status":       "revision_needed",
							"p_review_notes": map[string]any{"validation_errors": concerns, "source": "council_approved_but_validation_failed"},
						})
					} else {
						log.Printf("[EventCouncilDone] Failed to create tasks: %v", err)
						_, _ = database.RPC(ctx, "update_plan_status", map[string]any{
							"p_plan_id":      planID,
							"p_status":       "error",
							"p_review_notes": map[string]any{"error": err.Error()},
						})
					}
				}

			case "revision_needed", "blocked":
				for _, r := range councilReviews {
					concerns, _ := r["concerns"].([]interface{})
					for _, c := range concerns {
						if cm, ok := c.(map[string]interface{}); ok {
							description, _ := cm["description"].(string)
							if description != "" {
								_, err := database.RPC(ctx, "create_planner_rule", map[string]any{
									"p_applies_to": "*",
									"p_rule_type":  "council_feedback",
									"p_rule_text":  "Avoid: " + description,
									"p_source":     "council",
								})
								if err != nil {
									log.Printf("[EventCouncilDone] Failed to create planner rule: %v", err)
								}
							}
						}
					}
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventCouncilDone] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventRevisionNeeded, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("planner_revision:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventRevisionNeeded] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventRevisionNeeded] Plan %s already being processed", truncateID(planID))
			return
		}

		maxRounds := cfg.GetMaxRevisionRounds()
		onMaxRounds := cfg.GetOnMaxRoundsAction()

		currentRound, _ := plan["revision_round"].(float64)
		if int(currentRound) >= maxRounds {
			log.Printf("[EventRevisionNeeded] Plan %s revision limit (%d) reached (current: %d), escalating", truncateID(planID), maxRounds, int(currentRound))
			_, err := database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  onMaxRounds,
				"p_review_notes": map[string]any{
					"error":         "revision_limit_reached",
					"max_rounds":    maxRounds,
					"current_round": int(currentRound),
				},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		limitReached, _ := database.RPC(ctx, "check_revision_limit", map[string]any{
			"p_plan_id":    planID,
			"p_max_rounds": maxRounds,
		})

		var limitReachedBool bool
		if limitReached != nil {
			if err := json.Unmarshal(limitReached, &limitReachedBool); err != nil {
				var result []bool
				if err := json.Unmarshal(limitReached, &result); err == nil && len(result) > 0 {
					limitReachedBool = result[0]
				}
			}
		}

		if limitReachedBool {
			log.Printf("[EventRevisionNeeded] Plan %s revision limit (%d) reached, escalating to human", truncateID(planID), maxRounds)
			_, err := database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  onMaxRounds,
				"p_review_notes": map[string]any{
					"error":      "revision_limit_reached",
					"max_rounds": maxRounds,
				},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		_, err = database.RPC(ctx, "increment_revision_round", map[string]any{
			"p_plan_id": planID,
		})
		if err != nil {
			log.Printf("[EventRevisionNeeded] Failed to increment revision round: %v", err)
		}

		revisionHistory, _ := plan["revision_history"].([]interface{})
		var latestFeedback map[string]any
		if len(revisionHistory) > 0 {
			if rh, ok := revisionHistory[len(revisionHistory)-1].(map[string]any); ok {
				latestFeedback = rh
			}
		}

		destID := selectDestination("planner", planID, "revision")
		if destID == "" {
			log.Printf("[EventRevisionNeeded] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.CreateWithContext(ctx, "planner", "revision")
		if err != nil {
			log.Printf("[EventRevisionNeeded] Failed to create planner session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			result, err := session.Run(ctx, map[string]any{
				"plan":             plan,
				"event":            "revision_needed",
				"revision_history": revisionHistory,
				"latest_feedback":  latestFeedback,
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Planner session failed for %s: %v", truncateID(planID), err)
				return err
			}

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventRevisionNeeded] Failed to parse planner output: %v", parseErr)
				return nil
			}

			log.Printf("[EventRevisionNeeded] Plan %s revised, status: %s", truncateID(planID), plannerOutput.Status)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
				if err := git.CommitOutput(ctx, "main", output); err != nil {
					log.Printf("[EventRevisionNeeded] Failed to commit plan to GitHub: %v", err)
				}
			}

			newStatus := plannerOutput.Status
			if newStatus == "" {
				newStatus = "review"
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       newStatus,
				"p_plan_path":    plannerOutput.PlanPath,
				"p_review_notes": map[string]any{"plan_content": plannerOutput.PlanContent, "total_tasks": plannerOutput.TotalTasks, "revised": true},
			})
			if err != nil {
				log.Printf("[EventRevisionNeeded] Failed to update plan status: %v", err)
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventRevisionNeeded] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventPlanApproved, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		log.Printf("[EventPlanApproved] Plan %s already approved (direct), tasks should exist", truncateID(planID))
	})

	router.On(runtime.EventPlanBlocked, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		log.Printf("[EventPlanBlocked] Plan %s blocked - requires human intervention", truncateID(planID))
	})

	router.On(runtime.EventPRDIncomplete, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		reviewNotes, _ := plan["review_notes"].(map[string]any)
		blockedReason, _ := reviewNotes["blocked_reason"].(string)

		log.Printf("[EventPRDIncomplete] Plan %s PRD incomplete: %s", truncateID(planID), blockedReason)

		// Transition to pending_human with the blocked reason
		_, err := database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  "pending_human",
			"p_review_notes": map[string]any{
				"blocked_reason": blockedReason,
				"action_needed":  "Update PRD with missing information",
			},
		})
		if err != nil {
			log.Printf("[EventPRDIncomplete] Failed to update plan status: %v", err)
		}
	})

	router.On(runtime.EventPlanError, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)
		reviewNotes, _ := plan["review_notes"].(map[string]any)
		errorMsg, _ := reviewNotes["error"].(string)
		log.Printf("[EventPlanError] Plan %s in error state: %s", truncateID(planID), errorMsg)
	})

	router.On(runtime.EventCouncilReview, func(event runtime.Event) {
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			return
		}

		planID, _ := plan["id"].(string)

		processingBy := fmt.Sprintf("council_review:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventCouncilReview] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventCouncilReview] Plan %s already being processed", truncateID(planID))
			return
		}

		memberCount := cfg.GetCouncilMemberCount()
		lenses := cfg.GetCouncilLenses()
		includePRD := cfg.ShouldCouncilIncludePRD()

		var prdContent string
		if includePRD {
			if prdPath, ok := plan["prd_path"].(string); ok && prdPath != "" {
				fullPath := filepath.Join(cfg.GetRepoPath(), prdPath)
				if content, err := os.ReadFile(fullPath); err == nil {
					prdContent = string(content)
				}
			}
		}

		destID := selectDestination("council", planID, "council_review")
		if destID == "" {
			log.Printf("[EventCouncilReview] No destination available for plan %s", truncateID(planID))
			return
		}

		councilMode := "sequential_same_model_different_hats"
		councilModels := []map[string]any{}

		availableDests := destRouter.GetAvailableDestinations()
		internalDests := 0
		for _, d := range availableDests {
			category := cfg.GetDestinationCategory(d)
			if category == "internal" {
				internalDests++
			}
		}

		if internalDests >= memberCount {
			councilMode = "parallel_different_models"
		}

		log.Printf("[EventCouncilReview] Plan %s council starting (mode: %s, members: %d)", truncateID(planID), councilMode, memberCount)

		reviews := make([]map[string]any, memberCount)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < memberCount; i++ {
			wg.Add(1)
			go func(memberIndex int) {
				defer wg.Done()

				lens := lenses[memberIndex%len(lenses)]
				session, err := factory.CreateWithContext(ctx, "council", lens)
				if err != nil {
					log.Printf("[EventCouncilReview] Failed to create council session for member %d: %v", memberIndex+1, err)
					return
				}

				contextData := map[string]any{
					"plan":          plan,
					"lens":          lens,
					"member_number": memberIndex + 1,
				}
				if prdContent != "" {
					contextData["prd_content"] = prdContent
				}

				result, err := session.Run(ctx, contextData)
				if err != nil {
					log.Printf("[EventCouncilReview] Council member %d failed: %v", memberIndex+1, err)
					return
				}

				vote, parseErr := runtime.ParseCouncilVote(result.Output)
				if parseErr != nil {
					log.Printf("[EventCouncilReview] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
					return
				}

				mu.Lock()
				reviews[memberIndex] = map[string]any{
					"member_number": memberIndex + 1,
					"lens":          lens,
					"vote":          vote.Vote,
					"concerns":      vote.Concerns,
					"reasoning":     vote.Reasoning,
					"destination":   destID,
				}
				councilModels = append(councilModels, map[string]any{
					"lens":        lens,
					"destination": destID,
				})
				mu.Unlock()

				log.Printf("[EventCouncilReview] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
			}(i)
		}
		wg.Wait()

		validReviews := make([]map[string]any, 0)
		for _, r := range reviews {
			if r != nil {
				validReviews = append(validReviews, r)
			}
		}

		if len(validReviews) == 0 {
			log.Printf("[EventCouncilReview] No valid votes for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		reviewsJSON, _ := json.Marshal(validReviews)
		modelsJSON, _ := json.Marshal(councilModels)
		_, storeErr := database.RPC(ctx, "store_council_reviews", map[string]any{
			"p_plan_id": planID,
			"p_reviews": reviewsJSON,
			"p_mode":    councilMode,
			"p_models":  modelsJSON,
		})
		if storeErr != nil {
			log.Printf("[EventCouncilReview] Failed to store council reviews: %v", storeErr)
		}

		approved := 0
		revisionNeeded := 0
		blocked := 0
		var allConcerns []string
		var tasksNeedingRevision []string

		for _, r := range validReviews {
			vote, _ := r["vote"].(string)
			switch vote {
			case "APPROVED":
				approved++
			case "REVISION_NEEDED":
				revisionNeeded++
			case "BLOCKED":
				blocked++
			}
			if concerns, ok := r["concerns"].([]interface{}); ok {
				for _, c := range concerns {
					if concern, ok := c.(string); ok {
						allConcerns = append(allConcerns, concern)
					}
				}
			}
		}

		consensusMethod := cfg.GetConsensusMethod()
		var consensus string
		if consensusMethod == "unanimous_approval" {
			if approved == memberCount {
				consensus = "approved"
			} else if blocked > 0 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		} else {
			if approved > memberCount/2 {
				consensus = "approved"
			} else if blocked > memberCount/2 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		}

		log.Printf("[EventCouncilReview] Plan %s consensus: %s (approved=%d, revision=%d, blocked=%d, method=%s)", truncateID(planID), consensus, approved, revisionNeeded, blocked, consensusMethod)

		var newStatus string
		switch consensus {
		case "approved":
			newStatus = "approved"
		case "blocked":
			newStatus = "blocked"
		case "revision_needed":
			newStatus = "revision_needed"
			_, feedbackErr := database.RPC(ctx, "record_revision_feedback", map[string]any{
				"p_plan_id":                planID,
				"p_source":                 "council",
				"p_feedback":               map[string]any{"concerns": allConcerns},
				"p_tasks_needing_revision": tasksNeedingRevision,
			})
			if feedbackErr != nil {
				log.Printf("[EventCouncilReview] Failed to record revision feedback: %v", feedbackErr)
			}
		}

		_, updateErr := database.RPC(ctx, "update_plan_status", map[string]any{
			"p_plan_id": planID,
			"p_status":  newStatus,
			"p_review_notes": map[string]any{
				"consensus":        consensus,
				"approved_count":   approved,
				"revision_count":   revisionNeeded,
				"blocked_count":    blocked,
				"council_mode":     councilMode,
				"consensus_method": consensusMethod,
			},
		})
		if updateErr != nil {
			log.Printf("[EventCouncilReview] Failed to update plan status: %v", updateErr)
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
	})

	router.On(runtime.EventMaintenanceCmd, func(event runtime.Event) {
		var cmd map[string]any
		if err := json.Unmarshal(event.Record, &cmd); err != nil {
			return
		}

		cmdID, _ := cmd["id"].(string)

		processingBy := fmt.Sprintf("maintenance_cmd:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "maintenance_commands",
			"p_id":            cmdID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventMaintenanceCmd] Command %s already being processed or claim failed", truncateID(cmdID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventMaintenanceCmd] Command %s already being processed", truncateID(cmdID))
			return
		}

		destID := selectDestination("maintenance", cmdID, "maintenance")
		if destID == "" {
			log.Printf("[EventMaintenanceCmd] No destination available for command %s", truncateID(cmdID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			return
		}

		session, err := factory.Create("maintenance")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "maintenance", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})

			result, err := session.Run(ctx, map[string]any{"command": cmd, "event": "maintenance_command"})
			if err != nil {
				return err
			}
			log.Printf("[EventMaintenanceCmd] Command %s executed via %s: %s", truncateID(cmdID), destID, truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "maintenance_commands", "p_id": cmdID})
			log.Printf("[EventMaintenanceCmd] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventResearchReady, func(event runtime.Event) {
		var suggestion map[string]any
		if err := json.Unmarshal(event.Record, &suggestion); err != nil {
			log.Printf("[EventResearchReady] Failed to parse suggestion: %v", err)
			return
		}

		suggestionID, _ := suggestion["id"].(string)
		suggestionType, _ := suggestion["type"].(string)
		complexity, _ := suggestion["complexity"].(string)

		processingBy := fmt.Sprintf("research_ready:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "research_suggestions",
			"p_id":            suggestionID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventResearchReady] Suggestion %s already being processed or claim failed", truncateID(suggestionID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventResearchReady] Suggestion %s already being processed", truncateID(suggestionID))
			return
		}

		log.Printf("[EventResearchReady] Processing research suggestion %s (type: %s, complexity: %s)", truncateID(suggestionID), suggestionType, complexity)

		switch complexity {
		case "human":
			log.Printf("[EventResearchReady] Human review required for %s", truncateID(suggestionID))
			_, err := database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":           suggestionID,
				"p_status":       "pending_human",
				"p_review_notes": map[string]any{"reason": "complexity=human, requires human decision"},
			})
			if err != nil {
				log.Printf("[EventResearchReady] Failed to update status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return

		case "complex":
			log.Printf("[EventResearchReady] Complex item %s - routing to council", truncateID(suggestionID))
			_, err := database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":           suggestionID,
				"p_status":       "council_review",
				"p_review_notes": map[string]any{"source": "research", "type": suggestionType},
			})
			if err != nil {
				log.Printf("[EventResearchReady] Failed to update status: %v", err)
			}
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		destID := selectDestination("supervisor", suggestionID, "research_review")
		if destID == "" {
			log.Printf("[EventResearchReady] No destination available")
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "research", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})

			result, err := session.Run(ctx, map[string]any{
				"event":      "research_review",
				"suggestion": suggestion,
			})
			if err != nil {
				return err
			}

			review, parseErr := runtime.ParseResearchReview(result.Output)
			if parseErr != nil {
				log.Printf("[EventResearchReady] Failed to parse review: %v", parseErr)
				log.Printf("[EventResearchReady] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventResearchReady] Suggestion %s review: decision=%s", truncateID(suggestionID), review.Decision)

			switch review.Decision {
			case "approved":
				if review.MaintenanceCommand != nil {
					cmdJSON, _ := json.Marshal(review.MaintenanceCommand.Details)
					_, err := database.RPC(ctx, "create_maintenance_command", map[string]any{
						"p_command_type": review.MaintenanceCommand.Action,
						"p_payload":      json.RawMessage(cmdJSON),
						"p_source":       "research_review",
						"p_approved_by":  "supervisor",
					})
					if err != nil {
						log.Printf("[EventResearchReady] Failed to create maintenance command: %v", err)
					} else {
						log.Printf("[EventResearchReady] Created maintenance command: %s", review.MaintenanceCommand.Action)
					}
				}
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "approved",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"notes":     review.Notes,
					},
				})

			case "rejected":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "rejected",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
					},
				})

			case "council_review":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "council_review",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"source":    "research",
					},
				})

			case "human_review":
				_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
					"p_id":     suggestionID,
					"p_status": "pending_human",
					"p_review_notes": map[string]any{
						"reasoning": review.Reasoning,
						"urgency":   review.Urgency,
					},
				})
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			log.Printf("[EventResearchReady] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventResearchCouncil, func(event runtime.Event) {
		var suggestion map[string]any
		if err := json.Unmarshal(event.Record, &suggestion); err != nil {
			log.Printf("[EventResearchCouncil] Failed to parse suggestion: %v", err)
			return
		}

		suggestionID, _ := suggestion["id"].(string)
		title, _ := suggestion["title"].(string)

		processingBy := fmt.Sprintf("research_council:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "research_suggestions",
			"p_id":            suggestionID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventResearchCouncil] Suggestion %s already being processed or claim failed", truncateID(suggestionID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventResearchCouncil] Suggestion %s already being processed", truncateID(suggestionID))
			return
		}

		log.Printf("[EventResearchCouncil] Starting council review for %s: %s", truncateID(suggestionID), title)

		memberCount := cfg.GetCouncilMemberCount()
		lenses := cfg.GetCouncilLenses()
		if len(lenses) == 0 {
			lenses = []string{"user_alignment", "architecture", "feasibility"}
		}

		councilMode := "sequential_same_model"
		availableDests := destRouter.GetAvailableDestinations()
		internalDests := 0
		for _, d := range availableDests {
			category := cfg.GetDestinationCategory(d)
			if category == "internal" {
				internalDests++
			}
		}

		if internalDests >= memberCount {
			councilMode = "parallel_different_models"
		}

		log.Printf("[EventResearchCouncil] Council starting (mode: %s, members: %d)", councilMode, memberCount)

		reviews := make([]map[string]any, memberCount)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < memberCount; i++ {
			wg.Add(1)
			go func(memberIndex int) {
				defer wg.Done()

				lens := lenses[memberIndex%len(lenses)]
				session, err := factory.CreateWithContext(ctx, "council", lens)
				if err != nil {
					log.Printf("[EventResearchCouncil] Failed to create council session for member %d: %v", memberIndex+1, err)
					return
				}

				contextData := map[string]any{
					"research":      suggestion,
					"lens":          lens,
					"member_number": memberIndex + 1,
					"review_type":   "research",
				}

				result, err := session.Run(ctx, contextData)
				if err != nil {
					log.Printf("[EventResearchCouncil] Council member %d failed: %v", memberIndex+1, err)
					return
				}

				vote, parseErr := runtime.ParseCouncilVote(result.Output)
				if parseErr != nil {
					log.Printf("[EventResearchCouncil] Failed to parse vote from member %d: %v", memberIndex+1, parseErr)
					return
				}

				mu.Lock()
				reviews[memberIndex] = map[string]any{
					"member_number": memberIndex + 1,
					"lens":          lens,
					"vote":          vote.Vote,
					"concerns":      vote.Concerns,
					"reasoning":     vote.Reasoning,
				}
				mu.Unlock()

				log.Printf("[EventResearchCouncil] Member %d (%s) voted: %s", memberIndex+1, lens, vote.Vote)
			}(i)
		}
		wg.Wait()

		validReviews := make([]map[string]any, 0)
		for _, r := range reviews {
			if r != nil {
				validReviews = append(validReviews, r)
			}
		}

		if len(validReviews) == 0 {
			log.Printf("[EventResearchCouncil] No valid votes for suggestion %s", truncateID(suggestionID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
			return
		}

		approved := 0
		revisionNeeded := 0
		blocked := 0
		var allConcerns []string

		for _, r := range validReviews {
			vote, _ := r["vote"].(string)
			switch vote {
			case "APPROVED":
				approved++
			case "REVISION_NEEDED":
				revisionNeeded++
			case "BLOCKED":
				blocked++
			}
			if concerns, ok := r["concerns"].([]interface{}); ok {
				for _, c := range concerns {
					if cm, ok := c.(map[string]interface{}); ok {
						if desc, ok := cm["description"].(string); ok && desc != "" {
							allConcerns = append(allConcerns, desc)
						} else if issue, ok := cm["issue"].(string); ok && issue != "" {
							allConcerns = append(allConcerns, issue)
						}
					}
				}
			}
		}

		consensusMethod := cfg.GetConsensusMethod()
		var consensus string
		if consensusMethod == "unanimous_approval" {
			if approved == memberCount {
				consensus = "approved"
			} else if blocked > 0 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		} else {
			if approved > memberCount/2 {
				consensus = "approved"
			} else if blocked > memberCount/2 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}
		}

		log.Printf("[EventResearchCouncil] Consensus: %s (approved=%d, revision=%d, blocked=%d)", consensus, approved, revisionNeeded, blocked)

		switch consensus {
		case "approved":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "approved",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"reviews":           validReviews,
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s approved by council", truncateID(suggestionID))

		case "blocked":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "rejected",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"concerns":          allConcerns,
					"reviews":           validReviews,
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s blocked by council", truncateID(suggestionID))

		case "revision_needed":
			_, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     suggestionID,
				"p_status": "pending_human",
				"p_review_notes": map[string]any{
					"council_consensus": consensus,
					"concerns":          allConcerns,
					"reviews":           validReviews,
					"note":              "Council could not reach consensus, escalating to human",
				},
			})
			log.Printf("[EventResearchCouncil] Research suggestion %s needs human review", truncateID(suggestionID))
		}

		database.RPC(ctx, "clear_processing", map[string]any{"p_table": "research_suggestions", "p_id": suggestionID})
	})

	router.On(runtime.EventPRDReady, func(event runtime.Event) {
		startTime := time.Now()
		var plan map[string]any
		if err := json.Unmarshal(event.Record, &plan); err != nil {
			log.Printf("[EventPRDReady] Failed to parse plan: %v", err)
			return
		}

		planID, _ := plan["id"].(string)
		currentStatus, _ := plan["status"].(string)

		processingBy := fmt.Sprintf("planner:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventPRDReady] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPRDReady] Plan %s already being processed", truncateID(planID))
			return
		}

		destID := selectDestination("planner", planID, "planning")
		if destID == "" {
			log.Printf("[EventPRDReady] No destination available for plan %s", truncateID(planID))
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "no_destination")
			return
		}

		projectType := "general"
		if prdPath, ok := plan["prd_path"].(string); ok {
			if strings.Contains(strings.ToLower(prdPath), "dashboard") || strings.Contains(strings.ToLower(prdPath), "ui") {
				projectType = "frontend"
			} else if strings.Contains(strings.ToLower(prdPath), "api") {
				projectType = "backend"
			}
		}

		session, err := factory.CreateWithContext(ctx, "planner", projectType)
		if err != nil {
			log.Printf("[EventPRDReady] Failed to create planner session: %v", err)
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "session_creation_failed")
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", destID, func() error {
			defer database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "review", "planning_complete")

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "prd_ready"})
			if err != nil {
				log.Printf("[EventPRDReady] Planner session failed for %s: %v", truncateID(planID), err)
				database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), false, map[string]any{"error": err.Error()})
				return err
			}

			database.RecordPerformanceMetric(ctx, "prd_to_plan", planID, time.Since(startTime), true, nil)

			log.Printf("[EventPRDReady] Raw output for %s (len=%d): %s", truncateID(planID), len(result.Output), truncateOutput(result.Output))

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventPRDReady] Failed to parse planner output: %v", parseErr)
				return nil
			}

			if plannerOutput.Status == "" {
				plannerOutput.Status = "review"
			}

			log.Printf("[EventPRDReady] Plan %s created with %d tasks, status: %s", truncateID(planID), plannerOutput.TotalTasks, plannerOutput.Status)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
				if err := git.CommitOutput(ctx, "main", output); err != nil {
					log.Printf("[EventPRDReady] Failed to commit plan to GitHub: %v", err)
				}
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       plannerOutput.Status,
				"p_plan_path":    plannerOutput.PlanPath,
				"p_review_notes": map[string]any{"plan_content": plannerOutput.PlanContent, "total_tasks": plannerOutput.TotalTasks},
			})
			if err != nil {
				log.Printf("[EventPRDReady] Failed to update plan status: %v", err)
			}

			return nil
		})
		if err != nil {
			database.ClearProcessingAndRecordTransition(ctx, "plans", planID, currentStatus, "error", "pool_submit_failed")
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

		processingBy := fmt.Sprintf("supervisor:%d", time.Now().UnixNano())
		claimed, err := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "plans",
			"p_id":            planID,
			"p_processing_by": processingBy,
		})
		if err != nil || claimed == nil {
			log.Printf("[EventPlanReview] Plan %s already being processed or claim failed", truncateID(planID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventPlanReview] Plan %s already being processed", truncateID(planID))
			return
		}

		destID := selectDestination("supervisor", planID, "plan_review")
		if destID == "" {
			log.Printf("[EventPlanReview] No destination available for plan %s", truncateID(planID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			log.Printf("[EventPlanReview] Failed to create supervisor session: %v", err)
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})

			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_review"})
			if err != nil {
				log.Printf("[EventPlanReview] Supervisor session failed for %s: %v", truncateID(planID), err)
				return err
			}

			review, parseErr := runtime.ParseInitialReview(result.Output)
			if parseErr != nil {
				log.Printf("[EventPlanReview] Failed to parse review: %v", parseErr)
				log.Printf("[EventPlanReview] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventPlanReview] Plan %s review: decision=%s complexity=%s", truncateID(planID), review.Decision, review.Complexity)

			var newStatus string
			var statusError error
			switch review.Decision {
			case "approved":
				if err := createTasksFromApprovedPlan(ctx, database, plan, cfg.GetValidationConfig(), cfg.GetRepoPath()); err != nil {
					var validationErr *ValidationFailedError
					if errors.As(err, &validationErr) {
						log.Printf("[EventPlanReview] Task validation failed for plan %s - sending back to planner", truncateID(planID))
						newStatus = "revision_needed"

						var concerns []string
						var taskNumbers []string
						for _, e := range validationErr.Errors {
							concerns = append(concerns, fmt.Sprintf("%s: %s", e.TaskNumber, e.Issue))
							taskNumbers = append(taskNumbers, e.TaskNumber)
						}

						_, recordErr := database.RPC(ctx, "record_planner_revision", map[string]any{
							"p_plan_id":                planID,
							"p_concerns":               concerns,
							"p_tasks_needing_revision": taskNumbers,
						})
						if recordErr != nil {
							log.Printf("[EventPlanReview] Failed to record validation feedback: %v", recordErr)
						}

						_, recordErr = database.RPC(ctx, "record_supervisor_rule", map[string]any{
							"p_rule_text":  fmt.Sprintf("Plan passed review but failed task validation: %s", strings.Join(concerns, "; ")),
							"p_applies_to": "plan_review",
							"p_source":     "validation_safety_net",
						})
						if recordErr != nil {
							log.Printf("[EventPlanReview] Failed to record supervisor rule: %v", recordErr)
						}

						log.Printf("[EventPlanReview] Validation concerns: %v", concerns)
						statusError = err
					} else {
						log.Printf("[EventPlanReview] Failed to create tasks: %v", err)
						newStatus = "error"
						statusError = err
					}
				} else {
					newStatus = "approved"
				}
			case "needs_revision":
				newStatus = "revision_needed"
				_, err := database.RPC(ctx, "record_planner_revision", map[string]any{
					"p_plan_id":                planID,
					"p_concerns":               review.Concerns,
					"p_tasks_needing_revision": review.TasksNeedingRevision,
				})
				if err != nil {
					log.Printf("[EventPlanReview] Failed to record revision feedback: %v", err)
				}
				log.Printf("[EventPlanReview] Plan %s needs revision: %v", truncateID(planID), review.Concerns)
			case "council_review":
				newStatus = "council_review"
			default:
				newStatus = "revision_needed"
			}

			reviewNotes := map[string]any{
				"complexity": review.Complexity,
				"reasoning":  review.Reasoning,
				"concerns":   review.Concerns,
				"task_count": review.TaskCount,
			}
			if statusError != nil {
				reviewNotes["error"] = statusError.Error()
			}

			log.Printf("[EventPlanReview] Updating plan %s status to: %s", truncateID(planID), newStatus)
			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       newStatus,
				"p_review_notes": reviewNotes,
			})
			if err != nil {
				log.Printf("[EventPlanReview] Failed to update plan status: %v", err)
			} else {
				log.Printf("[EventPlanReview] Plan %s status updated to: %s", truncateID(planID), newStatus)
			}

			if statusError != nil {
				return statusError
			}
			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "plans", "p_id": planID})
			log.Printf("[EventPlanReview] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventTestResults, func(event runtime.Event) {
		var testResult map[string]any
		if err := json.Unmarshal(event.Record, &testResult); err != nil {
			return
		}

		resultID, _ := testResult["id"].(string)
		taskID, _ := testResult["task_id"].(string)
		taskNumber, _ := testResult["task_number"].(string)

		processingBy := fmt.Sprintf("test_results:%d", time.Now().UnixNano())
		claimed, claimErr := database.RPC(ctx, "set_processing", map[string]any{
			"p_table":         "test_results",
			"p_id":            resultID,
			"p_processing_by": processingBy,
		})
		if claimErr != nil || claimed == nil {
			log.Printf("[EventTestResults] Test result %s already being processed or claim failed", truncateID(resultID))
			return
		}
		var claimSuccess bool
		if err := json.Unmarshal(claimed, &claimSuccess); err != nil || !claimSuccess {
			log.Printf("[EventTestResults] Test result %s already being processed", truncateID(resultID))
			return
		}

		destID := selectDestination("supervisor", taskID, "test_review")
		if destID == "" {
			log.Printf("[EventTestResults] No destination available for task %s", truncateID(taskID))
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			return
		}

		err = pool.SubmitWithDestination(ctx, "testing", destID, func() error {
			defer database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})

			result, err := session.Run(ctx, map[string]any{"test_result": testResult, "event": "test_results"})
			if err != nil {
				return err
			}

			testOutput, parseErr := runtime.ParseTestResults(result.Output)
			if parseErr != nil {
				log.Printf("[EventTestResults] Failed to parse test output: %v", parseErr)
				log.Printf("[EventTestResults] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventTestResults] Task %s test outcome: %s, next: %s", truncateID(taskID), testOutput.TestOutcome, testOutput.NextAction)

			switch testOutput.NextAction {
			case "final_merge":
				branchName := fmt.Sprintf("task/%s", taskNumber)
				sliceID, _ := testResult["slice_id"].(string)
				if sliceID == "" {
					sliceID = "default"
				}
				targetBranch := fmt.Sprintf("module/%s", sliceID)

				if err := git.MergeBranch(ctx, branchName, targetBranch); err != nil {
					log.Printf("[EventTestResults] Failed to merge %s to %s: %v", branchName, targetBranch, err)
					return nil
				}

				if err := git.DeleteBranch(ctx, branchName); err != nil {
					log.Printf("[EventTestResults] Failed to delete branch %s: %v", branchName, err)
				}

				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "complete",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to update task status to complete: %v", err)
				}

				_, err = database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
					"p_completed_task_id": taskID,
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to unlock dependents: %v", err)
				}

			case "return_for_fix":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "available",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to reset task for fix: %v", err)
				}

			case "await_human_approval":
				_, err := database.RPC(ctx, "update_task_status", map[string]any{
					"p_task_id": taskID,
					"p_status":  "awaiting_human",
				})
				if err != nil {
					log.Printf("[EventTestResults] Failed to set task awaiting_human: %v", err)
				}
			}

			return nil
		})
		if err != nil {
			database.RPC(ctx, "clear_processing", map[string]any{"p_table": "test_results", "p_id": resultID})
			log.Printf("[EventTestResults] Failed to submit to pool: %v", err)
		}
	})

	_ = eventsCfg
}

func recordModelSuccess(ctx context.Context, database *db.DB, modelID, taskType string, durationSeconds float64) {
	if modelID == "" {
		return
	}
	_, err := database.RPC(ctx, "record_model_success", map[string]any{
		"p_model_id":         modelID,
		"p_task_type":        taskType,
		"p_duration_seconds": durationSeconds,
	})
	if err != nil {
		log.Printf("[Learning] Failed to record model success: %v", err)
	}
}

func recordModelFailure(ctx context.Context, database *db.DB, modelID, taskID, failureType string) {
	if modelID == "" {
		return
	}
	_, err := database.RPC(ctx, "record_model_failure", map[string]any{
		"p_model_id":     modelID,
		"p_failure_type": failureType,
		"p_task_id":      taskID,
	})
	if err != nil {
		log.Printf("[Learning] Failed to record model failure: %v", err)
	}
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncateOutput(output string) string {
	if len(output) > 5000 {
		return output[:5000] + "..."
	}
	return output
}

func extractCouncilReviews(plan map[string]any) []map[string]any {
	reviews := plan["council_reviews"]
	if reviews == nil {
		return nil
	}

	switch v := reviews.(type) {
	case []interface{}:
		var result []map[string]any
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	case []map[string]interface{}:
		return v
	default:
		return nil
	}
}

type RecoveryConfig struct {
	OrphanThresholdSeconds int
	MaxTaskAttempts        int
	ModelFailureThreshold  int
}

func getRecoveryConfig(cfg *runtime.Config) RecoveryConfig {
	recovery := RecoveryConfig{
		OrphanThresholdSeconds: 300,
		MaxTaskAttempts:        3,
		ModelFailureThreshold:  3,
	}

	if cfg.System != nil && cfg.System.Recovery != nil {
		if v := cfg.System.Recovery["orphan_threshold_seconds"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.OrphanThresholdSeconds = int(val)
			case int:
				recovery.OrphanThresholdSeconds = val
			}
		}
		if v := cfg.System.Recovery["max_task_attempts"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.MaxTaskAttempts = int(val)
			case int:
				recovery.MaxTaskAttempts = val
			}
		}
		if v := cfg.System.Recovery["model_failure_threshold"]; v != nil {
			switch val := v.(type) {
			case float64:
				recovery.ModelFailureThreshold = int(val)
			case int:
				recovery.ModelFailureThreshold = val
			}
		}
	}

	return recovery
}

func runStartupRecovery(ctx context.Context, database *db.DB, cfg RecoveryConfig) {
	log.Println("Running startup recovery...")

	orphans, err := database.RPC(ctx, "find_orphaned_sessions", map[string]interface{}{
		"p_orphan_threshold_seconds": cfg.OrphanThresholdSeconds,
	})
	if err != nil {
		log.Printf("[Recovery] Warning: Could not check for orphans: %v", err)
		return
	}

	var orphanList []map[string]interface{}
	if err := json.Unmarshal(orphans, &orphanList); err != nil {
		log.Printf("[Recovery] Warning: Could not parse orphan list: %v", err)
		return
	}

	if len(orphanList) == 0 {
		log.Println("[Recovery] No orphaned sessions found")
		return
	}

	log.Printf("[Recovery] Found %d orphaned session(s)", len(orphanList))

	for _, orphan := range orphanList {
		sessionID, _ := orphan["id"].(string)
		taskID, _ := orphan["task_id"].(string)
		secondsSince, _ := orphan["seconds_since_heartbeat"].(float64)

		log.Printf("[Recovery] Recovering orphan session %s (task %s, %d seconds since heartbeat)",
			truncateID(sessionID), truncateID(taskID), int(secondsSince))

		_, err := database.RPC(ctx, "recover_orphaned_session", map[string]interface{}{
			"p_session_id": sessionID,
			"p_reason":     "startup_recovery",
		})
		if err != nil {
			log.Printf("[Recovery] Failed to recover session %s: %v", truncateID(sessionID), err)
		}
	}

	log.Printf("[Recovery] Recovery complete - %d session(s) recovered", len(orphanList))
}

func runProcessingRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config) {
	timeout := cfg.GetProcessingTimeoutSeconds()
	interval := time.Duration(cfg.GetProcessingRecoveryIntervalSeconds()) * time.Second

	if interval == 0 {
		interval = 60 * time.Second
	}
	if timeout == 0 {
		timeout = 300
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[ProcessingRecovery] Starting (interval: %v, timeout: %ds)", interval, timeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("[ProcessingRecovery] Stopped")
			return
		case <-ticker.C:
			recoverStaleProcessing(ctx, database, "plans", timeout)
			recoverStaleProcessing(ctx, database, "tasks", timeout)
			recoverStaleProcessing(ctx, database, "test_results", timeout)
			recoverStaleProcessing(ctx, database, "research_suggestions", timeout)
			recoverStaleProcessing(ctx, database, "maintenance_commands", timeout)
		}
	}
}

func recoverStaleProcessing(ctx context.Context, database *db.DB, table string, timeout int) {
	stale, err := database.RPC(ctx, "find_stale_processing", map[string]any{
		"p_table":           table,
		"p_timeout_seconds": timeout,
	})
	if err != nil {
		log.Printf("[ProcessingRecovery] Failed to find stale %s: %v", table, err)
		return
	}

	var staleItems []map[string]any
	if err := json.Unmarshal(stale, &staleItems); err != nil {
		return
	}

	if len(staleItems) == 0 {
		return
	}

	for _, item := range staleItems {
		id, _ := item["id"].(string)
		processingBy, _ := item["processing_by"].(string)
		secondsStale, _ := item["seconds_stale"].(float64)

		log.Printf("[ProcessingRecovery] Recovering stale %s %s (processing_by: %s, stale for %ds)",
			table[:len(table)-1], truncateID(id), processingBy, int(secondsStale))

		_, err := database.RPC(ctx, "recover_stale_processing", map[string]any{
			"p_table":  table,
			"p_id":     id,
			"p_reason": fmt.Sprintf("timeout_recovery (%ds)", int(secondsStale)),
		})
		if err != nil {
			log.Printf("[ProcessingRecovery] Failed to recover %s %s: %v", table[:len(table)-1], truncateID(id), err)
		}
	}

	log.Printf("[ProcessingRecovery] Recovered %d stale %s", len(staleItems), table)
}

func runCheckpointRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config, checkpointMgr *core.CheckpointManager) {
	coreCfg := cfg.GetCoreConfig()
	if !coreCfg.IsRecoveryEnabled() {
		log.Println("[CheckpointRecovery] Recovery disabled, skipping")
		return
	}

	log.Println("[CheckpointRecovery] Checking for tasks with checkpoints...")

	result, err := database.RPC(ctx, "find_tasks_with_checkpoints", map[string]any{
		"p_statuses": []string{"in_progress", "review", "testing"},
	})
	if err != nil {
		log.Printf("[CheckpointRecovery] Warning: Could not query checkpoints: %v", err)
		return
	}

	var tasks []map[string]any
	if err := json.Unmarshal(result, &tasks); err != nil {
		log.Printf("[CheckpointRecovery] Warning: Could not parse checkpoint list: %v", err)
		return
	}

	if len(tasks) == 0 {
		log.Println("[CheckpointRecovery] No tasks with checkpoints found")
		return
	}

	log.Printf("[CheckpointRecovery] Found %d task(s) with checkpoints", len(tasks))

	for _, task := range tasks {
		taskID, _ := task["task_id"].(string)
		taskNumber, _ := task["task_number"].(string)
		status, _ := task["status"].(string)
		step, _ := task["step"].(string)
		progress, _ := task["progress"].(float64)

		log.Printf("[CheckpointRecovery] Task %s (status: %s, step: %s, progress: %d%%)",
			taskNumber, status, step, int(progress))

		switch step {
		case "execution":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to reset task %s: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Reset task %s to available for re-execution", taskNumber)
				database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
			}

		case "review":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "review",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to set task %s to review: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Task %s will be picked up for review", taskNumber)
			}

		case "testing":
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "testing",
			})
			if err != nil {
				log.Printf("[CheckpointRecovery] Failed to set task %s to testing: %v", taskNumber, err)
			} else {
				log.Printf("[CheckpointRecovery] Task %s will be picked up for testing", taskNumber)
			}

		default:
			log.Printf("[CheckpointRecovery] Unknown step '%s' for task %s, resetting to available", step, taskNumber)
			_, err := database.RPC(ctx, "update_task_status", map[string]any{
				"p_task_id": taskID,
				"p_status":  "available",
			})
			if err == nil {
				database.RPC(ctx, "delete_checkpoint", map[string]any{"p_task_id": taskID})
			}
		}
	}

	log.Printf("[CheckpointRecovery] Recovery complete - processed %d task(s)", len(tasks))
}

type TaskData struct {
	TaskNumber       string
	Title            string
	Confidence       float64
	Dependencies     []string
	Category         string
	RequiresCodebase bool
	Type             string
	PromptPacket     string
	ExpectedOutput   string
}

type ValidationError struct {
	TaskNumber string
	Issue      string
	Severity   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("task %s: %s", e.TaskNumber, e.Issue)
}

type ValidationFailedError struct {
	Errors []ValidationError
}

func (e *ValidationFailedError) Error() string {
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%s (%s)", err.Issue, err.TaskNumber))
	}
	return strings.Join(msgs, "; ")
}

func validateTasks(tasks []TaskData, validationCfg *runtime.ValidationConfig) *ValidationFailedError {
	var errors []ValidationError

	if validationCfg == nil {
		validationCfg = &runtime.ValidationConfig{
			MinTaskConfidence:     0.95,
			RequirePromptPacket:   true,
			RequireCategory:       true,
			RequireExpectedOutput: true,
		}
	}

	for _, task := range tasks {
		if task.Confidence < validationCfg.MinTaskConfidence {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      fmt.Sprintf("confidence %.2f below minimum %.2f - task must be split further", task.Confidence, validationCfg.MinTaskConfidence),
				Severity:   "high",
			})
		}
		if validationCfg.RequirePromptPacket && task.PromptPacket == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "empty prompt packet - task has no instructions",
				Severity:   "critical",
			})
		}
		if validationCfg.RequireCategory && task.Category == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing category - needed for routing to appropriate model",
				Severity:   "medium",
			})
		}
		if validationCfg.RequireExpectedOutput && task.ExpectedOutput == "" {
			errors = append(errors, ValidationError{
				TaskNumber: task.TaskNumber,
				Issue:      "missing expected output - supervisor cannot verify completion",
				Severity:   "medium",
			})
		}
	}

	if len(errors) > 0 {
		return &ValidationFailedError{Errors: errors}
	}
	return nil
}

func createTasksFromApprovedPlan(ctx context.Context, database *db.DB, plan map[string]any, validationCfg *runtime.ValidationConfig, repoPath string) error {
	planID, _ := plan["id"].(string)
	planPath, _ := plan["plan_path"].(string)

	if planPath == "" {
		return fmt.Errorf("plan has no plan_path")
	}

	fullPath := filepath.Join(repoPath, planPath)

	planContent, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read plan file %s: %w", fullPath, err)
	}

	tasks, err := parseTasksFromPlanMarkdown(string(planContent))
	if err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no valid tasks found in plan")
	}

	log.Printf("[createTasksFromApprovedPlan] Found %d tasks in plan %s", len(tasks), truncateID(planID))

	if validationErr := validateTasks(tasks, validationCfg); validationErr != nil {
		log.Printf("[createTasksFromApprovedPlan] Validation failed for plan %s: %v", truncateID(planID), validationErr)
		return validationErr
	}

	createdCount := 0
	for _, task := range tasks {
		routingFlag := "web"
		if task.RequiresCodebase {
			routingFlag = "internal"
		}

		taskID, err := database.RPC(ctx, "create_task_with_packet", map[string]any{
			"p_plan_id":             planID,
			"p_task_number":         task.TaskNumber,
			"p_title":               task.Title,
			"p_type":                task.Type,
			"p_status":              "pending",
			"p_priority":            5,
			"p_confidence":          task.Confidence,
			"p_category":            task.Category,
			"p_routing_flag":        routingFlag,
			"p_routing_flag_reason": fmt.Sprintf("From plan: %s", planPath),
			"p_dependencies":        task.Dependencies,
			"p_prompt":              task.PromptPacket,
			"p_expected_output":     task.ExpectedOutput,
			"p_context":             map[string]any{"source": "plan_approval"},
		})
		if err != nil {
			log.Printf("[createTasksFromApprovedPlan] Failed to create task %s: %v", task.TaskNumber, err)
			continue
		}

		var taskIDStr string
		if len(taskID) > 0 {
			json.Unmarshal(taskID, &taskIDStr)
		}
		log.Printf("[createTasksFromApprovedPlan] Created task %s: %s (id: %s)", task.TaskNumber, task.Title, truncateID(taskIDStr))
		createdCount++
	}

	log.Printf("[createTasksFromApprovedPlan] Created %d/%d tasks for plan %s", createdCount, len(tasks), truncateID(planID))
	return nil
}

func parseTasksFromPlanMarkdown(content string) ([]TaskData, error) {
	var tasks []TaskData

	taskHeaderRegex := regexp.MustCompile(`(?m)^### (T\d+):`)
	matches := taskHeaderRegex.FindAllStringSubmatchIndex(content, -1)

	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		start := match[0]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		section := content[start:end]
		task, err := parseTaskSection(section)
		if err != nil {
			log.Printf("[parseTasksFromPlanMarkdown] Failed to parse task section: %v", err)
			continue
		}

		if task.TaskNumber != "" && task.Title != "" && task.PromptPacket != "" {
			tasks = append(tasks, task)
		} else {
			log.Printf("[parseTasksFromPlanMarkdown] Task %s incomplete: title=%q prompt_len=%d", task.TaskNumber, task.Title, len(task.PromptPacket))
		}
	}

	return tasks, nil
}

func parseTaskSection(section string) (TaskData, error) {
	var task TaskData
	task.Type = "feature"
	task.Category = "coding"

	headerEnd := strings.Index(section, "\n")
	if headerEnd == -1 {
		return task, fmt.Errorf("no newline in section")
	}

	header := section[3:headerEnd]
	body := section[headerEnd+1:]

	parts := strings.SplitN(header, ":", 2)
	if len(parts) < 2 {
		return task, fmt.Errorf("invalid header format: %s", header)
	}

	task.TaskNumber = strings.TrimSpace(parts[0])
	task.Title = strings.TrimSpace(parts[1])

	confidenceMatch := regexp.MustCompile(`\*\*Confidence:\*\*\s*([\d.]+)`).FindStringSubmatch(body)
	if len(confidenceMatch) > 1 {
		task.Confidence, _ = strconv.ParseFloat(confidenceMatch[1], 64)
	}

	depsMatch := regexp.MustCompile(`\*\*Dependencies:\*\*\s*(.+)`).FindStringSubmatch(body)
	if len(depsMatch) > 1 {
		depsStr := strings.TrimSpace(depsMatch[1])
		if depsStr != "none" && depsStr != "-" {
			task.Dependencies = strings.Fields(depsStr)
		}
	}

	categoryMatch := regexp.MustCompile(`\*\*Category:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(categoryMatch) > 1 {
		task.Category = strings.TrimSpace(categoryMatch[1])
	}

	typeMatch := regexp.MustCompile(`\*\*Type:\*\*\s*(\w+)`).FindStringSubmatch(body)
	if len(typeMatch) > 1 {
		task.Type = strings.TrimSpace(typeMatch[1])
	}

	codebaseMatch := regexp.MustCompile(`\*\*Requires Codebase:\*\*\s*(true|false)`).FindStringSubmatch(body)
	if len(codebaseMatch) > 1 {
		task.RequiresCodebase = strings.ToLower(codebaseMatch[1]) == "true"
	}

	// Parse Prompt Packet - supports both code block and plain text formats
	ppStart := strings.Index(body, "#### Prompt Packet")
	if ppStart != -1 {
		ppContent := body[ppStart+19:]
		ppContent = strings.TrimSpace(ppContent)

		codeStart := strings.Index(ppContent, "```")
		if codeStart != -1 && codeStart < 10 {
			ppContent = ppContent[codeStart+3:]
			if strings.HasPrefix(ppContent, "json") || strings.HasPrefix(ppContent, "markdown") {
				ppContent = ppContent[strings.Index(ppContent, "\n")+1:]
			} else if strings.HasPrefix(ppContent, "\n") {
				ppContent = ppContent[1:]
			}
			ppEnd := strings.Index(ppContent, "\n```")
			if ppEnd != -1 {
				task.PromptPacket = strings.TrimSpace(ppContent[:ppEnd])
			}
		} else {
			nextSection := strings.Index(ppContent, "\n#### ")
			if nextSection != -1 {
				task.PromptPacket = strings.TrimSpace(ppContent[:nextSection])
			} else {
				task.PromptPacket = strings.TrimSpace(ppContent)
			}
		}
	}

	// Parse Expected Output - supports both code block and plain text formats
	eoStart := strings.Index(body, "#### Expected Output")
	if eoStart != -1 {
		eoContent := body[eoStart+19:]
		eoContent = strings.TrimSpace(eoContent)

		codeStart := strings.Index(eoContent, "```")
		if codeStart != -1 && codeStart < 10 {
			eoContent = eoContent[codeStart+3:]
			if strings.HasPrefix(eoContent, "json") || strings.HasPrefix(eoContent, "markdown") {
				eoContent = eoContent[strings.Index(eoContent, "\n")+1:]
			} else if strings.HasPrefix(eoContent, "\n") {
				eoContent = eoContent[1:]
			}
			eoEnd := strings.Index(eoContent, "\n```")
			if eoEnd != -1 {
				task.ExpectedOutput = strings.TrimSpace(eoContent[:eoEnd])
			}
		} else {
			nextSection := strings.Index(eoContent, "\n#### ")
			if nextSection != -1 {
				task.ExpectedOutput = strings.TrimSpace(eoContent[:nextSection])
			} else {
				task.ExpectedOutput = strings.TrimSpace(eoContent)
			}
		}
	}

	return task, nil
}
