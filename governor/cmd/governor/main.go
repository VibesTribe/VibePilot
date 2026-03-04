package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	setupMaintenanceHandlers(ctx, router, factory, pool, database, cfg, connRouter)
	setupTestingHandlers(ctx, router, factory, pool, database, cfg, connRouter, git)
}
            })
            log.Printf("[EventResearchCouncil] Research suggestion %s blocked by council", truncateID(suggestionID))

        case "revision_needed":
            _, _ = database.RPC(ctx, "update_research_suggestion_status", map[string]any{
                "p_id":     suggestionID
                "p_status": "pending_human"
                "p_review_notes": map[string]any{
                    "council_consensus": consensus
                    "reviews":           validReviews
                }
            })
            log.Printf("[EventResearchCouncil] Research suggestion %s needs human review", truncateID(suggestionID))
        }
    }

}
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
