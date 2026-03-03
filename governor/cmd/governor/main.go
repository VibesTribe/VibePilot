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
	"strings"
	"sync"
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

	pollInterval := time.Duration(cfg.System.Runtime.EventPollIntervalMs) * time.Millisecond
	watcher := runtime.NewPollingWatcher(database, pollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recoveryCfg := getRecoveryConfig(cfg)
	runStartupRecovery(ctx, database, recoveryCfg)
	runCheckpointRecovery(ctx, database, cfg, checkpointMgr)

	watcher.SetConfig(cfg)

	connRouter := runtime.NewRouter(cfg, database)
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

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, connRouter, git, stateMachine, checkpointMgr, leakDetector)

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
	eventsCfg := cfg.System.Events

	selectDestination := func(agentID, taskID, taskType string) string {
		result, err := connRouter.SelectDestination(ctx, runtime.RoutingRequest{
			AgentID:  agentID,
			TaskID:   taskID,
			TaskType: taskType,
		})
		if err != nil || result == nil {
			log.Printf("[Router] No destination available for agent %s, using fallback", agentID)
			dests := connRouter.GetAvailableConnectors()
			if len(dests) > 0 {
				return dests[0]
			}
			return ""
		}
		return result.DestinationID
	}

	setupTaskHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, checkpointMgr, leakDetector)

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

		availableDests := connRouter.GetAvailableConnectors()
		internalDests := 0
		for _, d := range availableDests {
			category := cfg.GetConnectorCategory(d)
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
		availableDests := connRouter.GetAvailableConnectors()
		internalDests := 0
		for _, d := range availableDests {
			category := cfg.GetConnectorCategory(d)
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
