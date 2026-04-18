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

	start := time.Now()
	passed, testOutput, err := h.runTests(ctx, branchName)
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

		// Complete
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": "complete",
		})

		// Auto-merge to module branch
		targetBranch := h.getTargetBranch(sliceID)

		// Shadow merge check
		if h.worktreeMgr != nil {
			shadowResult, shadowErr := h.worktreeMgr.ShadowMerge(ctx, branchName, targetBranch)
			if shadowErr != nil {
				log.Printf("[Testing] Shadow merge check failed for %s: %v (proceeding anyway)", branchName, shadowErr)
			} else if shadowResult != nil && shadowResult.HasConflicts {
				log.Printf("[Testing] Shadow merge found conflicts in %s: %v", branchName, shadowResult.ConflictFiles)
				h.database.RPC(ctx, "transition_task", map[string]any{
					"p_task_id":        taskID,
					"p_new_status":     "merge_pending",
					"p_failure_reason": fmt.Sprintf("merge conflicts: %v", shadowResult.ConflictFiles),
				})
				return
			}
		}

		if err := h.git.MergeBranch(ctx, branchName, targetBranch); err != nil {
			log.Printf("[Testing] Merge failed for %s: %v", branchName, err)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":        taskID,
				"p_new_status":     "merge_pending",
				"p_failure_reason": "merge_failed: " + err.Error(),
			})
		} else {
			// Merge success — cleanup
			if h.worktreeMgr != nil {
				h.worktreeMgr.RemoveWorktree(ctx, taskID)
			}
			h.git.DeleteBranch(ctx, branchName)
			h.database.RPC(ctx, "transition_task", map[string]any{
				"p_task_id":    taskID,
				"p_new_status": "merged",
			})
			h.database.RPC(ctx, "unlock_dependent_tasks", map[string]any{
				"p_completed_task_id": taskID,
			})
			log.Printf("[Testing] Task %s → merged to %s", truncateID(taskID), targetBranch)

			// Check if all tasks for this slice are now merged → merge module to testing
			h.tryMergeModuleToTesting(ctx, taskID, sliceID, targetBranch)
		}
	} else {
		// Tests failed → back to available with test output as feedback
		log.Printf("[Testing] Task %s tests FAILED in %.1fs:\n%s", truncateID(taskID), duration, truncateOutput(testOutput))

		if h.worktreeMgr != nil {
			h.worktreeMgr.RemoveWorktree(ctx, taskID)
		}
		h.git.DeleteBranch(ctx, branchName)
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "available",
			"p_failure_reason": "test_failed:\n" + testOutput,
		})
	}
}

// runTests executes `go test ./...` on the task branch.
// Returns (passed, output, error).
func (h *TestingHandler) runTests(ctx context.Context, branchName string) (bool, string, error) {
	repoPath := h.cfg.GetRepoPath()

	// Try to find an existing worktree for this branch
	testDir := repoPath
	if h.worktreeMgr != nil {
		worktrees, err := h.worktreeMgr.ListWorktrees(ctx)
		if err == nil {
			for _, wt := range worktrees {
				if strings.Contains(wt.BranchName, branchName) || strings.Contains(wt.Path, branchName) {
				testDir = wt.Path
				// Worktrees clone the repo root, but go.mod lives in governor/ subdirectory
				governorDir := filepath.Join(wt.Path, "governor")
				if _, err := os.Stat(filepath.Join(governorDir, "go.mod")); err == nil {
					testDir = governorDir
				}
				log.Printf("[Testing] Using worktree at %s", testDir)
					break
				}
			}
		}
	}

	// If no worktree found, checkout the branch in the main repo temporarily
	needsCheckout := testDir == repoPath
	if needsCheckout {
		// Save current branch
		headCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
		headCmd.Dir = repoPath
		var headOut bytes.Buffer
		headCmd.Stdout = &headOut
		if err := headCmd.Run(); err != nil {
			return false, "", fmt.Errorf("failed to get current branch: %w", err)
		}
		originalBranch := strings.TrimSpace(headOut.String())

		// Checkout the task branch
		log.Printf("[Testing] Checking out %s to run tests (was on %s)", branchName, originalBranch)
		checkoutCmd := exec.CommandContext(ctx, "git", "checkout", "-f", branchName)
		checkoutCmd.Dir = repoPath
		if out, err := checkoutCmd.CombinedOutput(); err != nil {
			return false, string(out), fmt.Errorf("checkout %s failed: %w", branchName, err)
		}

		// Defer checkout back
		defer func() {
			backCmd := exec.CommandContext(ctx, "git", "checkout", "-f", originalBranch)
			backCmd.Dir = repoPath
			if out, err := backCmd.CombinedOutput(); err != nil {
				log.Printf("[Testing] Warning: checkout back to %s failed: %v %s", originalBranch, err, string(out))
			}
		}()
	}

	// Run go test with a 2-minute timeout
	testCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Resolve go binary path — systemd services may not have go in PATH
	goBin := "/home/vibes/go/bin/go"
	if _, err := exec.LookPath("go"); err == nil {
		goBin = "go"
	}

	cmd := exec.CommandContext(testCtx, goBin, "test", "-v", "-count=1", "./...")
	cmd.Dir = testDir
	// Ensure PATH includes common go locations
	cmd.Env = append(os.Environ(),
		"PATH="+os.Getenv("PATH")+":/home/vibes/go/bin:/usr/local/go/bin",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[Testing] Running: go test -v -count=1 ./... (dir: %s)", testDir)
	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- STDERR ---\n" + stderr.String()
	}

	if testCtx.Err() == context.DeadlineExceeded {
		return false, output, fmt.Errorf("test execution timed out after 2m")
	}

	if err != nil {
		// ExitError means tests compiled but failed — that's a normal test failure
		if _, ok := err.(*exec.ExitError); ok {
			return false, output, nil
		}
		// Other error (compile failure, etc)
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
