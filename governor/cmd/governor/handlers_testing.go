package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
)

type TestingHandler struct {
	database    *db.DB
	factory     *runtime.SessionFactory
	pool        *runtime.AgentPool
	connRouter  *runtime.Router
	git         *gitree.Gitree
	cfg         *runtime.Config
	worktreeMgr *gitree.WorktreeManager
}

func NewTestingHandler(
	database *db.DB,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	cfg *runtime.Config,
	worktreeMgr *gitree.WorktreeManager,
) *TestingHandler {
	return &TestingHandler{
		database:    database,
		factory:     factory,
		pool:        pool,
		connRouter:  connRouter,
		git:         git,
		cfg:         cfg,
		worktreeMgr: worktreeMgr,
	}
}

func (h *TestingHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventTaskTesting, h.handleTaskTesting)
}

// ============================================================================
// TESTING: run go test directly, then merge or fail
// ============================================================================

func (h *TestingHandler) handleTaskTesting(event runtime.Event) {
	ctx := context.Background()

	var task map[string]any
	if err := json.Unmarshal(event.Record, &task); err != nil {
		log.Printf("[Testing] Parse error: %v", err)
		return
	}

	taskID := getString(task, "id")
	taskNumber := getString(task, "task_number")
	sliceID := getStringOr(task, "slice_id", "testing")
	branchName := getStringOr(task, "branch_name", "")

	if taskID == "" {
		return
	}

	if branchName == "" {
		branchName = h.buildBranchName(sliceID, taskNumber, taskID)
	}

	// Claim for testing
	testerID := fmt.Sprintf("tester:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "claim_for_review", map[string]any{
		"p_task_id":     taskID,
		"p_reviewer_id": testerID,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[Testing] Task %s already being tested", truncateID(taskID))
		return
	}

	log.Printf("[Testing] Running tests for task %s (branch: %s)", truncateID(taskID), branchName)

	changedPackages := h.extractTestPackages(task)

	start := time.Now()
	passed, testOutput, err := h.runTests(ctx, branchName, changedPackages)
	duration := time.Since(start).Seconds()

	if err != nil {
		log.Printf("[Testing] Test execution error for %s: %v — output: %s", truncateID(taskID), err, truncateOutput(testOutput))
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "available",
			"p_failure_reason": fmt.Sprintf("test_execution_error: %v\n%s", err, truncateOutput(testOutput)),
		})
		return
	}

	if passed {
		log.Printf("[Testing] Task %s tests PASSED in %.1fs", truncateID(taskID), duration)

		// TASK IS COMPLETE. Period. Testing passed = success.
		// Record completion with model tracking data. This is the source of truth
		// for model success/failure stats. Merge is a separate concern below.
		completionResult := map[string]any{
			"status":         "complete",
			"test_passed":    true,
			"test_duration":  duration,
			"files_created":  h.extractFilesFromTask(task),
		}
		completionJSON, _ := json.Marshal(completionResult)
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": "merged",
			"p_result":     string(completionJSON),
		})
		log.Printf("[Testing] Task %s → COMPLETE (testing passed)", truncateID(taskID))

		// Unlock dependents now — task is done regardless of merge
		// The DB unlock_dependent_tasks RPC searches by UUID, but our deps store
		// task numbers (e.g. "T001"). Use Go-native unlock that matches by number.
		unlockDependentsByTaskNumber(ctx, h.database, taskNumber)

		// Auto-merge to module branch (best effort, does not affect completion)
		targetBranch := h.getTargetBranch(sliceID)

		// Shadow merge check
		if h.worktreeMgr != nil {
			shadowResult, shadowErr := h.worktreeMgr.ShadowMerge(ctx, branchName, targetBranch)
			if shadowErr != nil {
				log.Printf("[Testing] Shadow merge check failed for %s: %v (proceeding anyway)", branchName, shadowErr)
			} else if shadowResult != nil && shadowResult.HasConflicts {
				log.Printf("[Testing] Shadow merge found conflicts for %s: %v (task still complete)", branchName, shadowResult.ConflictFiles)
				// Task stays merged. Merge conflicts handled separately.
				// Do NOT change task status — completion is already recorded.
				return
			}
		}

		if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
			// Merge failed but TASK IS STILL COMPLETE. Log and move on.
			log.Printf("[Testing] Merge to %s failed for task %s (task still complete): %v", targetBranch, truncateID(taskID), err)
		} else {
			// Merge success — cleanup
			log.Printf("[Testing] Task %s code merged to %s", truncateID(taskID), targetBranch)
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)

			// Check if all tasks for this slice are now merged → merge module to testing
			h.tryMergeModuleToTesting(ctx, taskID, sliceID, targetBranch)
		}
	} else {
		// Tests failed → keep branch and worktree for iterative fixing
		// The executor will resume on this same branch to fix the specific failures
		log.Printf("[Testing] Task %s tests FAILED in %.1fs:\n%s", truncateID(taskID), duration, truncateOutput(testOutput))

		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "available",
			"p_failure_reason": "test_failed:\n" + testOutput,
		})
		log.Printf("[Testing] Task %s → available (branch %s preserved for fix)", truncateID(taskID), branchName)
	}
}

// runTests executes go build + go test on the task branch.
// Returns (passed, output, error).
func (h *TestingHandler) runTests(ctx context.Context, branchName string, changedPackages []string) (bool, string, error) {
	repoPath := h.cfg.GetRepoPath()
	testDir := ""

	// Method 1: Check git worktree list
	if h.worktreeMgr != nil {
		worktrees, err := h.worktreeMgr.ListWorktrees(ctx)
		if err == nil {
			for _, wt := range worktrees {
				if strings.Contains(wt.BranchName, branchName) || strings.Contains(wt.Path, branchName) {
					testDir = wt.Path
					governorDir := filepath.Join(wt.Path, "governor")
					if _, err := os.Stat(filepath.Join(governorDir, "go.mod")); err == nil {
						testDir = governorDir
					}
					log.Printf("[Testing] Using git worktree at %s", testDir)
					break
				}
			}
		}
	}

	// Method 2: Scan VibePilot-work for directories with .go files
	if testDir == "" {
		workBase := filepath.Join(filepath.Dir(repoPath), "VibePilot-work")
		entries, err := os.ReadDir(workBase)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				dirPath := filepath.Join(workBase, e.Name())
				// Check for go.mod in governor/ subdir or direct
				govMod := filepath.Join(dirPath, "governor", "go.mod")
				directMod := filepath.Join(dirPath, "go.mod")
				if _, err := os.Stat(govMod); err == nil {
					testDir = filepath.Join(dirPath, "governor")
					log.Printf("[Testing] Using filesystem worktree at %s", testDir)
					break
				} else if _, err := os.Stat(directMod); err == nil {
					testDir = dirPath
					log.Printf("[Testing] Using filesystem worktree at %s", testDir)
					break
				}
			}
		}
	}

	// Method 3: Fallback to main repo
	if testDir == "" {
		testDir = repoPath
		log.Printf("[Testing] No worktree found, using main repo at %s", testDir)
	}

	// Timeout
	testCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	goBin := "/home/vibes/go/bin/go"
	if _, err := exec.LookPath("go"); err == nil {
		goBin = "go"
	}
	env := append(os.Environ(), "PATH="+os.Getenv("PATH")+":/home/vibes/go/bin:/usr/local/go/bin")

	// Phase 1: go build ./...
	buildCmd := exec.CommandContext(testCtx, goBin, "build", "./...")
	buildCmd.Dir = testDir
	buildCmd.Env = env
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	log.Printf("[Testing] Phase 1: go build ./... (dir: %s)", testDir)
	if err := buildCmd.Run(); err != nil {
		return false, buildOut.String(), fmt.Errorf("build failed")
	}

	// Phase 2: go test on changed packages (or ./... as fallback)
	testTargets := changedPackages
	if len(testTargets) == 0 {
		testTargets = []string{"./..."}
	}
	args := []string{"test", "-v", "-count=1"}
	args = append(args, testTargets...)
	cmd := exec.CommandContext(testCtx, goBin, args...)
	cmd.Dir = testDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[Testing] Phase 2: go test -v -count=1 %s (dir: %s)", strings.Join(testTargets, " "), testDir)
	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- STDERR ---\n" + stderr.String()
	}

	if testCtx.Err() == context.DeadlineExceeded {
		return false, output, fmt.Errorf("test execution timed out after 2m")
	}

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, output, nil
		}
		return false, output, err
	}

	return true, output, nil
}

func (h *TestingHandler) buildBranchName(sliceID, taskNumber, taskID string) string {
	prefix := h.cfg.GetTaskBranchPrefix()
	if prefix == "" {
		prefix = "task/"
	}

	if sliceID != "" && taskNumber != "" {
		return prefix + sliceID + "/" + taskNumber
	}

	if taskNumber != "" {
		return prefix + taskNumber
	}
	return prefix + truncateID(taskID)
}

func (h *TestingHandler) getTargetBranch(sliceID string) string {
	if sliceID == "" || sliceID == "default" || sliceID == "testing" || sliceID == "review" {
		sliceID = "general"
	}
	return "TEST_MODULES/" + sliceID
}

// tryMergeModuleToTesting checks if all tasks for the same slice and plan are merged.
// If so, merges TEST_MODULES/<slice> into the "testing" branch.
func (h *TestingHandler) tryMergeModuleToTesting(ctx context.Context, taskID, sliceID, moduleBranch string) {
	planID := ""
	// Query this task to get its plan_id
	taskData, err := h.database.Query(ctx, "tasks", map[string]any{"id": "eq." + taskID, "select": "plan_id"})
	if err != nil {
		log.Printf("[Testing] Module merge check: failed to query task plan_id: %v", err)
		return
	}
	var tasks []map[string]any
	if err := json.Unmarshal(taskData, &tasks); err == nil && len(tasks) > 0 {
		if pid, ok := tasks[0]["plan_id"].(string); ok {
			planID = pid
		}
	}
	if planID == "" {
		log.Printf("[Testing] Module merge check: no plan_id for task %s, skipping", truncateID(taskID))
		return
	}

	// Count tasks in this slice+plan that are NOT yet merged or complete
	// Check for any status other than merged/complete/escalated
	filters := map[string]any{
		"plan_id": "eq." + planID,
		"slice_id": "eq." + sliceID,
		"select":   "id,status",
	}
	siblingData, err := h.database.Query(ctx, "tasks", filters)
	if err != nil {
		log.Printf("[Testing] Module merge check: failed to query siblings: %v", err)
		return
	}
	var siblings []map[string]any
	if err := json.Unmarshal(siblingData, &siblings); err != nil {
		log.Printf("[Testing] Module merge check: failed to parse siblings: %v", err)
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
		log.Printf("[Testing] Module %s has %d tasks remaining (plan %s), skipping module merge", sliceID, remaining, truncateID(planID))
		return
	}

	// All tasks in this module are done → merge TEST_MODULES/<slice> into testing
	log.Printf("[Testing] All tasks complete for module %s (plan %s) → merging %s to testing", sliceID, truncateID(planID), moduleBranch)

	if err := h.git.MergeBranch(ctx, moduleBranch, "testing"); err != nil {
		log.Printf("[Testing] Module-to-testing merge FAILED for %s: %v", moduleBranch, err)
	} else {
		log.Printf("[Testing] Module %s successfully merged to testing branch", sliceID)
		// Step 3: Check if ALL modules for this plan are merged to testing → merge testing to main
		h.tryMergeTestingToMain(ctx, planID)
	}
}

// tryMergeTestingToMain checks if all modules for a plan are merged into testing.
// If so, merges the testing branch into main (the final integration step).
func (h *TestingHandler) tryMergeTestingToMain(ctx context.Context, planID string) {
	// Query all tasks for this plan to check module merge status
	taskData, err := h.database.Query(ctx, "tasks", map[string]any{
		"plan_id": "eq." + planID,
		"select":  "id,status,slice_id",
	})
	if err != nil {
		log.Printf("[Testing] Testing-to-main check: failed to query plan tasks: %v", err)
		return
	}
	var tasks []map[string]any
	if err := json.Unmarshal(taskData, &tasks); err != nil {
		log.Printf("[Testing] Testing-to-main check: failed to parse tasks: %v", err)
		return
	}

	// Group by slice_id to check each module's status
	moduleStatus := map[string]bool{} // sliceID → all tasks merged/done
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
			moduleStatus[sliceID] = false // at least one task not done
		}
		if _, exists := moduleStatus[sliceID]; !exists {
			moduleStatus[sliceID] = true // first task we see is terminal
		}
	}

	// Check if all modules are complete
	for slice, done := range moduleStatus {
		if !done {
			log.Printf("[Testing] Testing-to-main: module %s still has pending tasks (plan %s)", slice, truncateID(planID))
			return
		}
	}

	log.Printf("[Testing] All modules complete for plan %s → merging testing to main", truncateID(planID))
	if err := h.git.MergeBranch(ctx, "testing", "main"); err != nil {
		log.Printf("[Testing] Testing-to-main merge FAILED: %v", err)
	} else {
		log.Printf("[Testing] Plan %s fully integrated: testing → main merge complete", truncateID(planID))
	}
}

func setupTestingHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database *db.DB,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	worktreeMgr *gitree.WorktreeManager,
) {
	handler := NewTestingHandler(database, factory, pool, connRouter, git, cfg, worktreeMgr)
	handler.Register(router)
}

// extractTestPackages parses task result to find Go package paths for targeted testing.
// Falls back to empty slice (which triggers ./... in runTests).
func (h *TestingHandler) extractTestPackages(task map[string]any) []string {
	result, ok := task["result"].(map[string]any)
	if !ok {
		return nil
	}

	// Try expected_output.files_created first
	expectedStr, _ := result["expected_output"].(string)
	if expectedStr != "" {
		var expected map[string]any
		if json.Unmarshal([]byte(expectedStr), &expected) == nil {
			if files, ok := expected["files_created"].([]any); ok {
				return filesToPackagePaths(files)
			}
		}
	}

	// Try raw_output for files_created
	rawStr, _ := result["raw_output"].(string)
	if rawStr != "" {
		// Look for files_created in the JSON output
		var outputs []map[string]any
		// Try parsing as JSON array or object
		if json.Unmarshal([]byte(rawStr), &outputs) == nil {
			for _, o := range outputs {
				if files, ok := o["files_created"].([]any); ok {
					return filesToPackagePaths(files)
				}
			}
		}
		var output map[string]any
		if json.Unmarshal([]byte(rawStr), &output) == nil {
			if files, ok := output["files_created"].([]any); ok {
				return filesToPackagePaths(files)
			}
		}
	}

	return nil
}

// filesToPackagePaths converts file paths like "internal/hello/hello.go" to "./internal/hello/..."
func filesToPackagePaths(files []any) []string {
	seen := make(map[string]bool)
	var pkgs []string
	for _, f := range files {
		path, _ := f.(string)
		if path == "" {
			continue
		}
		// Extract directory from file path
		dir := filepath.Dir(path)
		// Only include .go-related paths
		if filepath.Ext(path) == ".go" || filepath.Ext(path) == "" {
			pkg := "./" + dir + "/..."
			if !seen[pkg] {
				seen[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
	}
	return pkgs
}

// extractFilesFromTask pulls files_created from task result for completion tracking
func (h *TestingHandler) extractFilesFromTask(task map[string]any) []string {
	result, ok := task["result"].(map[string]any)
	if !ok {
		return nil
	}
	// Try expected_output.files_created
	expectedStr, _ := result["expected_output"].(string)
	if expectedStr != "" {
		var expected map[string]any
		if json.Unmarshal([]byte(expectedStr), &expected) == nil {
			if files, ok := expected["files_created"].([]any); ok {
				var out []string
				for _, f := range files {
					if s, ok := f.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
		}
	}
	return nil
}
