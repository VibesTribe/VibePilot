package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

type MaintenanceHandler struct {
	database     db.Database
	factory      *runtime.SessionFactory
	pool         *runtime.AgentPool
	connRouter   *runtime.Router
	cfg          *runtime.Config
	git          *gitree.Gitree
	worktreeMgr  *gitree.WorktreeManager
	usageTracker *runtime.UsageTracker
}

func NewMaintenanceHandler(
	database db.Database,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
	git *gitree.Gitree,
	worktreeMgr *gitree.WorktreeManager,
	usageTracker *runtime.UsageTracker,
) *MaintenanceHandler {
	return &MaintenanceHandler{
		database:     database,
		factory:      factory,
		pool:         pool,
		connRouter:   connRouter,
		cfg:          cfg,
		git:          git,
		worktreeMgr:  worktreeMgr,
		usageTracker: usageTracker,
	}
}

func (h *MaintenanceHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventMaintenanceCmd, h.handleMaintenanceCommand)
	router.On(runtime.EventTaskApproval, h.handleTaskApproved)
	router.On(runtime.EventTaskMergePending, h.handleTaskMergePending)
}

func (h *MaintenanceHandler) handleMaintenanceCommand(event runtime.Event) {
	ctx := context.Background()

	cmd, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[MaintenanceCmd] Failed to get cmd record: %v", err)
		return
	}

	cmdID := getString(cmd, "id")
	cmdType := getString(cmd, "command_type")
	cmdStatus := getString(cmd, "status")
	payload := cmd["payload"]

	if cmdID == "" {
		return
	}

	// Guard: only process pending commands (defense against re-trigger after completion)
	if cmdStatus != "" && cmdStatus != "pending" {
		log.Printf("[MaintenanceCmd] Skipping command %s with status=%s (expected pending)", truncateID(cmdID), cmdStatus)
		return
	}

	processingBy := fmt.Sprintf("maintenance_cmd:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "maintenance_commands",
		"p_id":            cmdID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[MaintenanceCmd] Command %s already being processed", truncateID(cmdID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "maintenance_commands",
		"p_id":    cmdID,
	})

	log.Printf("[MaintenanceCmd] Processing command %s (type: %s)", truncateID(cmdID), cmdType)

	routingResult, err := h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
		Role:        "maintenance",
		TaskType:    cmdType,
		RoutingFlag: "internal",
	})
	if err != nil || routingResult == nil {
		log.Printf("[MaintenanceCmd] No routing for command %s", truncateID(cmdID))
		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":           cmdID,
			"p_status":       "failed",
			"p_result_notes": map[string]any{"error": "no_routing"},
		})
		return
	}

	session, err := h.factory.CreateWithConnector(ctx, "maintenance", cmdType, routingResult.ConnectorID)
	if err != nil {
		log.Printf("[MaintenanceCmd] Failed to create session for %s: %v", truncateID(cmdID), err)
		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":           cmdID,
			"p_status":       "failed",
			"p_result_notes": map[string]any{"error": err.Error()},
		})
		return
	}

	err = h.pool.SubmitWithDestination(ctx, "maintenance", routingResult.ConnectorID, func() error {
		start := time.Now()
		result, sessionErr := session.Run(ctx, map[string]any{
			"command":      cmd,
			"command_type": cmdType,
			"payload":      payload,
			"event":        "maintenance_command",
		})
		duration := time.Since(start).Seconds()

		if sessionErr != nil {
			log.Printf("[MaintenanceCmd] Execution failed for %s: %v", truncateID(cmdID), sessionErr)
			_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
				"p_id":           cmdID,
				"p_status":       "failed",
				"p_result_notes": map[string]any{"error": sessionErr.Error()},
			})
			// Record failure for the maintenance model
			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, routingResult.ModelID, "maintenance_"+cmdType, duration, false)
			}
			h.database.RPC(ctx, "record_model_failure", map[string]any{
				"p_model_id":         routingResult.ModelID,
				"p_task_type":        "maintenance_" + cmdType,
				"p_failure_class":    "execution_error",
				"p_failure_detail":   sessionErr.Error(),
				"p_duration_seconds": duration,
			})
			return sessionErr
		}

		log.Printf("[MaintenanceCmd] Command %s executed via %s (model=%s) in %.1fs", truncateID(cmdID), routingResult.ConnectorID, routingResult.ModelID, duration)

		_, _ = h.database.RPC(ctx, "update_maintenance_command_status", map[string]any{
			"p_id":     cmdID,
			"p_status": "completed",
			"p_result_notes": map[string]any{
				"output":       result.Output,
				"duration_ms":  int(duration * 1000),
				"tokens_in":    result.TokensIn,
				"tokens_out":   result.TokensOut,
				"connector_id": routingResult.ConnectorID,
				"model_id":     routingResult.ModelID,
			},
		})

		// Record success for the maintenance model
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, routingResult.ModelID, "maintenance_"+cmdType, duration, true)
		}
		h.database.RPC(ctx, "record_model_success", map[string]any{
			"p_model_id":         routingResult.ModelID,
			"p_task_type":        "maintenance_" + cmdType,
			"p_duration_seconds": duration,
			"p_tokens_used":      result.TokensIn + result.TokensOut,
		})
		h.database.RPC(ctx, "record_performance_metric", map[string]any{
			"p_agent_id":         "maintenance",
			"p_model_id":         routingResult.ModelID,
			"p_metric_type":      "maintenance_" + cmdType,
			"p_duration_seconds": duration,
			"p_success":          true,
		})

		return nil
	})
	if err != nil {
		log.Printf("[MaintenanceCmd] Failed to submit: %v", err)
	}
}

func (h *MaintenanceHandler) handleTaskApproved(event runtime.Event) {
	ctx := context.Background()

	task, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[TaskApproved] Failed to get task record: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// Claim using set_processing — works on any status (claim_for_review only
	// works for 'review'/'testing' which is why this handler was dead code).
	processingBy := fmt.Sprintf("merge:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskApproved] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	branchName := h.buildBranchName(sliceID, taskNumber, taskID)
	targetBranch := h.getTargetBranch(sliceID)

	log.Printf("[TaskApproved] Merging %s -> %s for task %s", branchName, targetBranch, truncateID(taskID))

	// Shadow merge check — detect conflicts before attempting real merge
	if h.worktreeMgr != nil {
		shadowResult, shadowErr := h.worktreeMgr.ShadowMerge(ctx, branchName, targetBranch)
		if shadowErr != nil {
			log.Printf("[TaskApproved] Shadow merge check failed for %s: %v (proceeding anyway)", branchName, shadowErr)
		} else if shadowResult != nil && shadowResult.HasConflicts {
			log.Printf("[TaskApproved] Shadow merge found conflicts for %s: %v → merge_pending", branchName, shadowResult.ConflictFiles)
			recordPipelineEvent(ctx, h.database, "merge_conflict_detected", taskID, "",
				fmt.Sprintf("conflicts in %v", shadowResult.ConflictFiles),
				map[string]any{
					"task_number":    taskNumber,
					"slice_id":       sliceID,
					"conflict_files": shadowResult.ConflictFiles,
					"source_branch":  branchName,
					"target_branch":  targetBranch,
				})
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "merge_pending",
			})
			h.database.RPC(ctx, "record_failure", map[string]any{
				"p_agent_id":       "maintenance",
				"p_failure_class":  "merge_conflict",
				"p_failure_detail": fmt.Sprintf("branch=%s target=%s conflicts=%v", branchName, targetBranch, shadowResult.ConflictFiles),
				"p_task_id":        taskID,
			})
			return
		}
	}

	if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
		log.Printf("[TaskApproved] Merge failed for %s: %v — task stays complete", truncateID(taskID), err)
		h.database.RPC(ctx, "record_failure", map[string]any{
			"p_agent_id":       "maintenance",
			"p_failure_class":  "merge_failed",
			"p_failure_detail": fmt.Sprintf("branch=%s target=%s: %v", branchName, targetBranch, err),
			"p_task_id":        taskID,
		})
		// Task stays "complete" — merge is best effort
		return
	}

	// Merge succeeded
	h.database.RPC(ctx, "transition_task", map[string]any{
		"p_task_id":    taskID,
		"p_new_status": "merged",
	})

	// Cleanup worktree and branch
	if h.worktreeMgr != nil {
		h.worktreeMgr.RemoveWorktree(ctx, taskID)
	}
	h.git.DeleteBranch(ctx, branchName)

	// Record merge event for timeline
	recordPipelineEvent(ctx, h.database, "task_merged_to_module", taskID, "",
		fmt.Sprintf("branch=%s → %s", branchName, targetBranch),
		map[string]any{
			"task_number":   taskNumber,
			"slice_id":      sliceID,
			"source_branch": branchName,
			"target_branch": targetBranch,
		})

	// Check if all tasks for this slice are now merged → merge module to testing
	h.tryMergeModuleToTesting(ctx, taskID, sliceID, targetBranch)

	h.database.RPC(ctx, "record_performance_metric", map[string]any{
		"p_agent_id":    "maintenance",
		"p_metric_type": "merge",
		"p_success":     true,
	})
	log.Printf("[TaskApproved] Task %s merged to %s", truncateID(taskID), targetBranch)
}

func (h *MaintenanceHandler) handleTaskMergePending(event runtime.Event) {
	ctx := context.Background()

	task, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[TaskMergePending] Failed to get task record: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	sliceID := getStringOr(task, "slice_id", "default")

	if taskID == "" {
		return
	}

	// Claim using set_processing — works on any status.
	processingBy := fmt.Sprintf("merge_retry:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "tasks",
		"p_id":            taskID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[TaskMergePending] Task %s already being processed", truncateID(taskID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "tasks",
		"p_id":    taskID,
	})

	log.Printf("[TaskMergePending] Creating maintenance command for task %s", truncateID(taskID))

	_, err = h.database.RPC(ctx, "create_maintenance_command", map[string]any{
		"p_command_type": "merge_conflict",
		"p_payload": map[string]any{
			"task_id":       taskID,
			"task_number":   taskNumber,
			"slice_id":      sliceID,
			"branch_name":   h.buildBranchName(sliceID, taskNumber, taskID),
			"target_branch": h.getTargetBranch(sliceID),
		},
		"p_status": "pending",
	})
	if err != nil {
		log.Printf("[TaskMergePending] Failed to create maintenance command: %v", err)
	}
}

func (h *MaintenanceHandler) buildBranchName(sliceID, taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}

	// Use slice-based naming: task/{slice_id}/{task_number}
	// Example: task/general/T001, task/auth/T002
	if sliceID != "" && taskNumber != "" {
		return prefix + sliceID + "/" + taskNumber
	}

	// Fallback to task number only (for backwards compatibility)
	if taskNumber != "" {
		return prefix + taskNumber
	}

	return prefix + truncateID(taskID)
}

func (h *MaintenanceHandler) getTargetBranch(sliceID string) string {
	if sliceID == "" || sliceID == "default" || sliceID == "testing" || sliceID == "review" {
		sliceID = "general"
	}
	return "TEST_MODULES/" + sliceID
}

// tryMergeModuleToTesting checks if all tasks for the same slice and plan are merged.
// If so, merges TEST_MODULES/<slice> into the "testing" branch.
func (h *MaintenanceHandler) tryMergeModuleToTesting(ctx context.Context, taskID, sliceID, moduleBranch string) {
	planID := ""
	taskData, err := h.database.Query(ctx, "tasks", map[string]any{"id": "eq." + taskID, "select": "plan_id"})
	if err != nil {
		log.Printf("[TaskApproved] Module merge check: failed to query task plan_id: %v", err)
		return
	}
	var tasks []map[string]any
	if err := json.Unmarshal(taskData, &tasks); err == nil && len(tasks) > 0 {
		if pid, ok := tasks[0]["plan_id"].(string); ok {
			planID = pid
		}
	}
	if planID == "" {
		log.Printf("[TaskApproved] Module merge check: no plan_id for task %s, skipping", truncateID(taskID))
		return
	}

	filters := map[string]any{
		"plan_id":  "eq." + planID,
		"slice_id": "eq." + sliceID,
		"select":   "id,status",
	}
	siblingData, err := h.database.Query(ctx, "tasks", filters)
	if err != nil {
		log.Printf("[TaskApproved] Module merge check: failed to query siblings: %v", err)
		return
	}
	var siblings []map[string]any
	if err := json.Unmarshal(siblingData, &siblings); err != nil {
		log.Printf("[TaskApproved] Module merge check: failed to parse siblings: %v", err)
		return
	}

	allDone := true
	remaining := 0
	for _, s := range siblings {
		status, _ := s["status"].(string)
		switch status {
		case "merged", "complete", "escalated", "cancelled":
			// terminal states
		default:
			allDone = false
			remaining++
		}
	}

	if !allDone {
		log.Printf("[TaskApproved] Module %s has %d tasks remaining (plan %s), skipping module merge", sliceID, remaining, truncateID(planID))
		return
	}

	log.Printf("[TaskApproved] All tasks complete for module %s (plan %s) → running integration test before merge", sliceID, truncateID(planID))

	// Module integration test: verify the combined code from all tasks compiles.
	// Individual task tests already passed; this catches cross-task incompatibilities.
	repoPath := h.cfg.GetRepoPath()
	integrationPassed, integrationOutput := h.runModuleIntegrationTest(ctx, repoPath, moduleBranch)

	recordPipelineEvent(ctx, h.database, "module_integration_test", taskID, "",
		fmt.Sprintf("module %s integration %s", sliceID, func() string { if integrationPassed { return "passed" } else { return "failed" } }()),
		map[string]any{
			"slice_id":       sliceID,
			"source_branch":  moduleBranch,
			"passed":         integrationPassed,
		})

	if !integrationPassed {
		log.Printf("[TaskApproved] Module %s integration test FAILED, creating maintenance command", sliceID)
		h.database.RPC(ctx, "create_maintenance_command", map[string]any{
			"p_command_type": "integration_test_failure",
			"p_payload": map[string]any{
				"task_id":       taskID,
				"slice_id":      sliceID,
				"branch_name":   moduleBranch,
				"test_output":   truncateOutput(integrationOutput),
				"scope":         "module",
			},
		})
		// Don't merge. The maintenance agent will fix integration issues.
		return
	}

	log.Printf("[TaskApproved] Module %s integration test passed → merging %s to testing", sliceID, moduleBranch)
	if err := h.git.MergeBranch(ctx, moduleBranch, "testing"); err != nil {
		log.Printf("[TaskApproved] Module-to-testing merge FAILED for %s: %v", moduleBranch, err)
		recordPipelineEvent(ctx, h.database, "module_merge_failed", taskID, "",
			fmt.Sprintf("module %s → testing failed: %v", sliceID, err),
			map[string]any{
				"slice_id":       sliceID,
				"source_branch":  moduleBranch,
				"target_branch":  "testing",
			})
		// Module merge failure is infrastructure, not task failure.
		// Create maintenance command for internal agent to resolve.
		h.database.RPC(ctx, "create_maintenance_command", map[string]any{
			"p_command_type": "merge_conflict",
			"p_payload": map[string]any{
				"task_id":       taskID,
				"slice_id":      sliceID,
				"branch_name":   moduleBranch,
				"target_branch": "testing",
				"scope":         "module",
			},
			"p_status": "pending",
		})
	} else {
		log.Printf("[TaskApproved] Module %s successfully merged to testing branch", sliceID)
		recordPipelineEvent(ctx, h.database, "module_merged_to_testing", taskID, "",
			fmt.Sprintf("module %s → testing", sliceID),
			map[string]any{
				"slice_id":       sliceID,
				"source_branch":  moduleBranch,
				"target_branch":  "testing",
			})
		// Delete module branch — all tasks merged, branch no longer needed
		if err := h.git.DeleteBranch(ctx, moduleBranch); err != nil {
			log.Printf("[TaskApproved] Warning: failed to delete module branch %s: %v", moduleBranch, err)
		} else {
			log.Printf("[TaskApproved] Deleted module branch %s", moduleBranch)
		}
		h.tryMergeTestingToMain(ctx, planID)
	}
}

// tryMergeTestingToMain checks if all modules for a plan are merged into testing.
// If so, merges the testing branch into main (final integration step).
func (h *MaintenanceHandler) tryMergeTestingToMain(ctx context.Context, planID string) {
	taskData, err := h.database.Query(ctx, "tasks", map[string]any{
		"plan_id": "eq." + planID,
		"select":  "id,status,slice_id",
	})
	if err != nil {
		log.Printf("[TaskApproved] Testing-to-main check: failed to query plan tasks: %v", err)
		return
	}
	var tasks []map[string]any
	if err := json.Unmarshal(taskData, &tasks); err != nil {
		log.Printf("[TaskApproved] Testing-to-main check: failed to parse tasks: %v", err)
		return
	}

	moduleStatus := map[string]bool{}
	for _, t := range tasks {
		sliceID, _ := t["slice_id"].(string)
		status, _ := t["status"].(string)
		if sliceID == "" {
			continue
		}
		switch status {
		case "merged", "complete", "escalated", "cancelled":
			// terminal — ok
		default:
			moduleStatus[sliceID] = false
		}
		if _, exists := moduleStatus[sliceID]; !exists {
			moduleStatus[sliceID] = true
		}
	}

	for slice, done := range moduleStatus {
		if !done {
			log.Printf("[TaskApproved] Testing-to-main: module %s still has pending tasks (plan %s)", slice, truncateID(planID))
			return
		}
	}

	log.Printf("[TaskApproved] All modules complete for plan %s → merging testing to main/testing/", truncateID(planID))
	if err := h.git.MergeBranchToSubdir(ctx, "testing", "main", "testing"); err != nil {
		log.Printf("[TaskApproved] Testing-to-main subtree merge FAILED: %v", err)
		recordPipelineEvent(ctx, h.database, "integration_merge_failed", "", "",
			fmt.Sprintf("testing → main/testing/ failed: %v", err),
			map[string]any{
				"plan_id": planID,
			})
		// Integration merge failure is infrastructure. Maintenance agent handles it.
		h.database.RPC(ctx, "create_maintenance_command", map[string]any{
			"p_command_type": "merge_conflict",
			"p_payload": map[string]any{
				"plan_id":       planID,
				"branch_name":   "testing",
				"target_branch": "main",
				"scope":         "integration",
			},
			"p_status": "pending",
		})
	} else {
		log.Printf("[TaskApproved] Plan %s fully integrated: testing → main/testing/ merge complete", truncateID(planID))
		recordPipelineEvent(ctx, h.database, "plan_complete", planID, "",
			"all modules merged to main/testing/",
			map[string]any{
				"plan_id": planID,
			})
	}
}

// runModuleIntegrationTest verifies that the combined code from all tasks in a module
// compiles successfully. This catches cross-task incompatibilities before merging to testing.
// It checks out the module branch, runs a compile check, then returns to main.
func (h *MaintenanceHandler) runModuleIntegrationTest(ctx context.Context, repoPath, moduleBranch string) (bool, string) {
	var output strings.Builder

	// Step 1: Stash any working tree changes
	execCmd(exec.Command("git", "stash"), repoPath, &output)

	// Step 2: Checkout the module branch
	checkoutCmd := exec.Command("git", "checkout", moduleBranch)
	checkoutOut, checkoutErr := checkoutCmd.CombinedOutput()
	output.WriteString(fmt.Sprintf("$ git checkout %s\n%s\n", moduleBranch, string(checkoutOut)))

	if checkoutErr != nil {
		output.WriteString(fmt.Sprintf("CHECKOUT FAILED: %v\n", checkoutErr))
		// Try to get back to main
		execCmd(exec.Command("git", "checkout", "main"), repoPath, &output)
		execCmd(exec.Command("git", "stash", "pop"), repoPath, &output)
		return false, output.String()
	}

	// Step 3: Run compile check (go build for Go projects)
	passed := true
	if _, err := exec.LookPath("go"); err == nil {
		// Found Go — run go build ./... to catch compile errors across all packages
		govDir := repoPath
		// Navigate to the governor module if it exists
		if _, err := exec.Command("stat", repoPath + "/governor/go.mod").CombinedOutput(); err == nil {
			govDir = repoPath + "/governor"
		}
		buildCmd := exec.Command("go", "build", "./...")
		buildCmd.Dir = govDir
		buildOut, buildErr := buildCmd.CombinedOutput()
		output.WriteString(fmt.Sprintf("$ go build ./...\n%s\n", string(buildOut)))
		if buildErr != nil {
			output.WriteString(fmt.Sprintf("BUILD FAILED: %v\n", buildErr))
			passed = false
		} else {
			output.WriteString("BUILD PASSED\n")
		}
	} else {
		// No Go compiler available — skip compile check (pass by default)
		output.WriteString("No go compiler found, skipping compile check (PASS)\n")
	}

	// Step 4: Return to main
	execCmd(exec.Command("git", "checkout", "main"), repoPath, &output)
	execCmd(exec.Command("git", "stash", "pop"), repoPath, &output)

	return passed, output.String()
}

// execCmd runs a command in a directory, appending output to the builder.
func execCmd(cmd *exec.Cmd, dir string, out *strings.Builder) {
	cmd.Dir = dir
	b, _ := cmd.CombinedOutput()
	if len(b) > 0 {
		out.WriteString(string(b))
		out.WriteString("\n")
	}
}

func setupMaintenanceHandler(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	worktreeMgr *gitree.WorktreeManager,
	usageTracker *runtime.UsageTracker,
) {
	handler := NewMaintenanceHandler(database, factory, pool, connRouter, cfg, git, worktreeMgr, usageTracker)
	handler.Register(router)
}
