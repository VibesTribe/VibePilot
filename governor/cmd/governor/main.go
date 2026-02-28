package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	watcher := runtime.NewPollingWatcher(&dbQuerierAdapter{db: database, cfg: cfg}, pollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recoveryCfg := getRecoveryConfig(cfg)
	runStartupRecovery(ctx, database, recoveryCfg)

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

	setupEventHandlers(ctx, eventRouter, sessionFactory, pool, database, cfg, toolRegistry, destRouter, git)

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

func setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, destRouter *runtime.Router, git *gitree.Gitree) {
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
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "default"
		}

		destID := selectDestination("orchestrator", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskAvailable] No destination available for task %s", truncateID(taskID))
			return
		}

		branchName := fmt.Sprintf("task/%s", taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("task/%s", truncateID(taskID))
		}

		if err := git.CreateBranch(ctx, branchName); err != nil {
			log.Printf("[EventTaskAvailable] Failed to create branch %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskAvailable] Created branch %s for task %s", branchName, truncateID(taskID))
		}

		session, err := factory.Create("orchestrator")
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to create orchestrator session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_available"})
			if err != nil {
				log.Printf("[EventTaskAvailable] Orchestrator session failed for %s: %v", truncateID(taskID), err)
				return err
			}
			log.Printf("[EventTaskAvailable] Task %s routed via %s: %s", truncateID(taskID), destID, truncateOutput(result.Output))
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
		taskType, _ := task["type"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "review"
		}

		destID := selectDestination("supervisor", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskReview] No destination available for task %s", truncateID(taskID))
			return
		}

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
		if err != nil {
			log.Printf("[EventTaskReview] Failed to create supervisor session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
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
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "complete"
		}

		destID := selectDestination("supervisor", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskCompleted] No destination available for task %s", truncateID(taskID))
			recordModelFailure(ctx, database, modelID, taskID, "no_destination")
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			recordModelFailure(ctx, database, modelID, taskID, "session_create_failed")
			return
		}

		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{"task": task, "event": "task_completed"})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[EventTaskCompleted] Supervisor session failed for %s: %v", truncateID(taskID), sessionErr)
			recordModelFailure(ctx, database, modelID, taskID, "session_error")
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			log.Printf("[EventTaskCompleted] Task %s completed via %s: %s", truncateID(taskID), destID, truncateOutput(result.Output))
			recordModelSuccess(ctx, database, modelID, taskType, duration)
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
		destID := selectDestination("supervisor", planID, "plan_review")
		if destID == "" {
			log.Printf("[EventPlanCreated] No destination available for plan %s", truncateID(planID))
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "plan_created"})
			if err != nil {
				return err
			}
			log.Printf("[EventPlanCreated] Plan %s triaged via %s: %s", truncateID(planID), destID, truncateOutput(result.Output))
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
		destID := selectDestination("supervisor", planID, "council_done")
		if destID == "" {
			log.Printf("[EventCouncilDone] No destination available for plan %s", truncateID(planID))
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
			_, err := session.Run(ctx, map[string]any{"plan": plan, "event": "council_done"})
			if err != nil {
				return err
			}

			councilReviews, _ := plan["council_reviews"].([]interface{})
			approved := 0
			revisionNeeded := 0
			blocked := 0

			for _, r := range councilReviews {
				if rm, ok := r.(map[string]interface{}); ok {
					vote, _ := rm["vote"].(string)
					switch vote {
					case "APPROVED":
						approved++
					case "REVISION_NEEDED":
						revisionNeeded++
					case "BLOCKED":
						blocked++
					}
				}
			}

			var consensus string
			if approved == 3 {
				consensus = "approved"
			} else if blocked > 0 {
				consensus = "blocked"
			} else {
				consensus = "revision_needed"
			}

			log.Printf("[EventCouncilDone] Plan %s consensus: %s (approved=%d, revision=%d, blocked=%d)", truncateID(planID), consensus, approved, revisionNeeded, blocked)

			_, err = database.RPC(ctx, "set_council_consensus", map[string]any{
				"p_plan_id":   planID,
				"p_consensus": consensus,
			})
			if err != nil {
				log.Printf("[EventCouncilDone] Failed to set council consensus: %v", err)
			}

			if consensus == "revision_needed" || consensus == "blocked" {
				for _, r := range councilReviews {
					if rm, ok := r.(map[string]interface{}); ok {
						concerns, _ := rm["concerns"].([]interface{})
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
			}

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
		destID := selectDestination("maintenance", cmdID, "maintenance")
		if destID == "" {
			log.Printf("[EventMaintenanceCmd] No destination available for command %s", truncateID(cmdID))
			return
		}

		session, err := factory.Create("maintenance")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "maintenance", destID, func() error {
			result, err := session.Run(ctx, map[string]any{"command": cmd, "event": "maintenance_command"})
			if err != nil {
				return err
			}
			log.Printf("[EventMaintenanceCmd] Command %s executed via %s: %s", truncateID(cmdID), destID, truncateOutput(result.Output))
			return nil
		})
		if err != nil {
			log.Printf("[EventMaintenanceCmd] Failed to submit to pool: %v", err)
		}
	})

	router.On(runtime.EventResearchReady, func(event runtime.Event) {
		destID := selectDestination("supervisor", "", "research_review")
		if destID == "" {
			log.Printf("[EventResearchReady] No destination available")
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "research", destID, func() error {
			result, err := session.Run(ctx, map[string]any{"event": "research_ready", "record": string(event.Record)})
			if err != nil {
				return err
			}
			log.Printf("[EventResearchReady] Research reviewed via %s: %s", destID, truncateOutput(result.Output))
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
		destID := selectDestination("planner", planID, "planning")
		if destID == "" {
			log.Printf("[EventPRDReady] No destination available for plan %s", truncateID(planID))
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
			return
		}

		err = pool.SubmitWithDestination(ctx, "planning", destID, func() error {
			result, err := session.Run(ctx, map[string]any{"plan": plan, "event": "prd_ready"})
			if err != nil {
				log.Printf("[EventPRDReady] Planner session failed for %s: %v", truncateID(planID), err)
				return err
			}

			plannerOutput, parseErr := runtime.ParsePlannerOutput(result.Output)
			if parseErr != nil {
				log.Printf("[EventPRDReady] Failed to parse planner output: %v", parseErr)
				log.Printf("[EventPRDReady] Raw output: %s", truncateOutput(result.Output))
				return nil
			}

			log.Printf("[EventPRDReady] Plan %s created with %d tasks, status: %s", truncateID(planID), plannerOutput.TotalTasks, plannerOutput.Status)

			if plannerOutput.PlanPath != "" && plannerOutput.PlanContent != "" {
				branchName := "docs/plans"
				output := map[string]any{
					"files": []map[string]any{
						{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
					},
				}
				if err := git.CommitOutput(ctx, branchName, output); err != nil {
					log.Printf("[EventPRDReady] Failed to commit plan to GitHub: %v", err)
				}
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id":      planID,
				"p_status":       plannerOutput.Status,
				"p_review_notes": map[string]any{"plan_path": plannerOutput.PlanPath, "total_tasks": plannerOutput.TotalTasks},
			})
			if err != nil {
				log.Printf("[EventPRDReady] Failed to update plan status: %v", err)
			}

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
		destID := selectDestination("supervisor", planID, "plan_review")
		if destID == "" {
			log.Printf("[EventPlanReview] No destination available for plan %s", truncateID(planID))
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			log.Printf("[EventPlanReview] Failed to create supervisor session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, "plans", destID, func() error {
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
			switch review.Decision {
			case "approved":
				newStatus = "approved"
			case "council_review":
				newStatus = "council_review"
			default:
				newStatus = "revision_needed"
			}

			_, err = database.RPC(ctx, "update_plan_status", map[string]any{
				"p_plan_id": planID,
				"p_status":  newStatus,
				"p_review_notes": map[string]any{
					"complexity": review.Complexity,
					"reasoning":  review.Reasoning,
					"concerns":   review.Concerns,
					"task_count": review.TaskCount,
				},
			})
			if err != nil {
				log.Printf("[EventPlanReview] Failed to update plan status: %v", err)
			}

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
		taskNumber, _ := testResult["task_number"].(string)
		destID := selectDestination("supervisor", taskID, "test_review")
		if destID == "" {
			log.Printf("[EventTestResults] No destination available for task %s", truncateID(taskID))
			return
		}

		session, err := factory.Create("supervisor")
		if err != nil {
			return
		}

		err = pool.SubmitWithDestination(ctx, "testing", destID, func() error {
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
	if len(output) > 200 {
		return output[:200] + "..."
	}
	return output
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
