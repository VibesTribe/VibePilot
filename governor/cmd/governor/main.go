package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
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

		branchName := fmt.Sprintf("task/%s", taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("task/%s", truncateID(taskID))
		}

		if err := git.CreateBranch(ctx, branchName); err != nil {
			log.Printf("[EventTaskAvailable] Failed to create branch %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskAvailable] Created branch %s for task %s", branchName, truncateID(taskID))
		}

		_, err := database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "in_progress",
		})
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to update status to in_progress: %v", err)
		}

		destID := selectDestination("task_runner", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskAvailable] No destination available for task %s", truncateID(taskID))
			return
		}

		session, err := factory.CreateWithContext(ctx, "task_runner", taskType)
		if err != nil {
			log.Printf("[EventTaskAvailable] Failed to create task_runner session: %v", err)
			return
		}

		err = pool.SubmitWithDestination(ctx, sliceID, destID, func() error {
			result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_available"})
			if err != nil {
				log.Printf("[EventTaskAvailable] Task runner failed for %s: %v", truncateID(taskID), err)
				return err
			}

			runnerOutput := map[string]any{
				"raw_output": result.Output,
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

			log.Printf("[EventTaskAvailable] Task %s output committed, status=review", truncateID(taskID))
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
		taskNumber, _ := task["task_number"].(string)
		modelID, _ := task["model_id"].(string)
		sliceID, _ := task["slice_id"].(string)
		if sliceID == "" {
			sliceID = "complete"
		}

		branchName := fmt.Sprintf("task/%s", taskNumber)
		if taskNumber == "" {
			branchName = fmt.Sprintf("task/%s", truncateID(taskID))
		}

		destID := selectDestination("supervisor", taskID, taskType)
		if destID == "" {
			log.Printf("[EventTaskCompleted] No destination available for task %s", truncateID(taskID))
			recordModelFailure(ctx, database, modelID, taskID, "no_destination")
			return
		}

		session, err := factory.CreateWithContext(ctx, "supervisor", taskType)
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

		output := map[string]any{
			"output":     result.Output,
			"model_id":   modelID,
			"task_id":    taskID,
			"duration":   duration,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
		}
		if err := git.CommitOutput(ctx, branchName, output); err != nil {
			log.Printf("[EventTaskCompleted] Failed to commit output to %s: %v", branchName, err)
		} else {
			log.Printf("[EventTaskCompleted] Committed output to %s", branchName)
		}

		_, err = database.RPC(ctx, "update_task_status", map[string]any{
			"p_task_id": taskID,
			"p_status":  "review",
		})
		if err != nil {
			log.Printf("[EventTaskCompleted] Failed to update task status to review: %v", err)
		}

		recordModelSuccess(ctx, database, modelID, taskType, duration)
		log.Printf("[EventTaskCompleted] Task %s output committed, status=review", truncateID(taskID))
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
				branchName := "docs/plans"
				files := []interface{}{
					map[string]interface{}{"path": plannerOutput.PlanPath, "content": plannerOutput.PlanContent},
				}
				output := map[string]interface{}{"files": files}
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
				if err := createTasksFromApprovedPlan(ctx, database, plan); err != nil {
					log.Printf("[EventPlanReview] Failed to create tasks: %v", err)
				}
			case "needs_revision":
				newStatus = "revision_needed"
				concernsJSON, _ := json.Marshal(review.Concerns)
				_, err := database.RPC(ctx, "record_planner_revision", map[string]any{
					"p_plan_id":                planID,
					"p_concerns":               concernsJSON,
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
	if len(output) > 5000 {
		return output[:5000] + "..."
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

func createTasksFromApprovedPlan(ctx context.Context, database *db.DB, plan map[string]any) error {
	planID, _ := plan["id"].(string)
	planPath, _ := plan["plan_path"].(string)

	if planPath == "" {
		return fmt.Errorf("plan has no plan_path")
	}

	repoPath := "/home/mjlockboxsocial/vibepilot"
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

	createdCount := 0
	for _, task := range tasks {
		if task.PromptPacket == "" {
			log.Printf("[createTasksFromApprovedPlan] WARNING: Task %s has empty prompt packet - skipping", task.TaskNumber)
			continue
		}

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

	sections := strings.Split(content, "### ")
	for _, section := range sections {
		if !strings.HasPrefix(section, "T") {
			continue
		}

		task, err := parseTaskSection(section)
		if err != nil {
			log.Printf("[parseTasksFromPlanMarkdown] Failed to parse task section: %v", err)
			continue
		}

		if task.TaskNumber != "" && task.Title != "" && task.PromptPacket != "" {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func parseTaskSection(section string) (TaskData, error) {
	var task TaskData
	task.Type = "feature"
	task.Category = "coding"

	lines := strings.SplitN(section, "\n", 2)
	if len(lines) < 2 {
		return task, fmt.Errorf("section too short")
	}

	header := lines[0]
	body := lines[1]

	parts := strings.SplitN(header, ":", 2)
	if len(parts) < 2 {
		return task, fmt.Errorf("invalid header format")
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

	ppStart := strings.Index(body, "#### Prompt Packet\n```\n")
	if ppStart != -1 {
		ppContent := body[ppStart+len("#### Prompt Packet\n```\n"):]
		ppEnd := strings.Index(ppContent, "\n```\n")
		if ppEnd != -1 {
			task.PromptPacket = strings.TrimSpace(ppContent[:ppEnd])
		}
	}

	eoStart := strings.Index(body, "#### Expected Output\n```json\n")
	if eoStart != -1 {
		eoContent := body[eoStart+len("#### Expected Output\n```json\n"):]
		eoEnd := strings.Index(eoContent, "\n```\n")
		if eoEnd != -1 {
			task.ExpectedOutput = strings.TrimSpace(eoContent[:eoEnd])
		}
	}

	return task, nil
}
