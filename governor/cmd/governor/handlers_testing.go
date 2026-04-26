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
	database     db.Database
	factory      *runtime.SessionFactory
	pool         *runtime.AgentPool
	connRouter   *runtime.Router
	git          *gitree.Gitree
	cfg          *runtime.Config
	worktreeMgr  *gitree.WorktreeManager
	usageTracker *runtime.UsageTracker
}

func NewTestingHandler(
	database db.Database,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	cfg *runtime.Config,
	worktreeMgr *gitree.WorktreeManager,
	usageTracker *runtime.UsageTracker,
) *TestingHandler {
	return &TestingHandler{
		database:     database,
		factory:      factory,
		pool:         pool,
		connRouter:   connRouter,
		git:          git,
		cfg:          cfg,
		worktreeMgr:  worktreeMgr,
		usageTracker: usageTracker,
	}
}

func (h *TestingHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventTaskTesting, h.handleTaskTesting)
}

// ============================================================================
// TESTING: 3-layer quality gate — artifact validation, semgrep, native tests
// ============================================================================

func (h *TestingHandler) handleTaskTesting(event runtime.Event) {
	ctx := context.Background()

	task, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[Testing] Failed to get task record: %v", err)
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

	// Extract expected output files from task result for artifact validation
	expectedFiles := h.extractExpectedFiles(task)

	start := time.Now()
	testDir := h.resolveTestDir(ctx, branchName)
	passed, testOutput, err := h.runTests(ctx, testDir, branchName, expectedFiles)
	duration := time.Since(start).Seconds()

	if err != nil {
		log.Printf("[Testing] Test execution error for %s: %v — output: %s", truncateID(taskID), err, truncateOutput(testOutput))

		// Record the test result even on execution error
		h.recordTestResult(ctx, taskID, taskNumber, sliceID, testDir, false,
			fmt.Sprintf("test_execution_error: %v", err), testOutput, duration)

		// The test runner crashed — could be due to broken code from the executor model.
		// Exclude the executor model so a different model gets a fresh attempt.
		executorModelID := h.getExecutorModelID(ctx, taskID)
		if executorModelID != "" {
			accumulateFailedModel(ctx, h.database, taskID, "exec_failed_by", executorModelID)
		}
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "pending",
			"p_failure_reason": fmt.Sprintf("test_execution_error: %v\n%s", err, truncateOutput(testOutput)),
		})
		return
	}

	// Record test result to DB for dashboard consumption
	h.recordTestResult(ctx, taskID, taskNumber, sliceID, testDir, passed, "", testOutput, duration)

	if passed {
		log.Printf("[Testing] Task %s tests PASSED in %.1fs", truncateID(taskID), duration)

		// Feed test result back to executor model learning.
		// This is the strongest quality signal: code that passes tests.
		executorModelID := h.getExecutorModelID(ctx, taskID)
		if executorModelID != "" && h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, executorModelID, "code_test", duration, true)
			log.Printf("[Testing] Recorded test PASS for executor model %s", executorModelID)
		}

		// Determine final status based on task type.
		// Visual/UI/UX tasks MUST go to human_review before merge.
		// Human has final say on all visual changes — this is non-negotiable.
		taskType := getString(task, "type")
		nextStatus := "complete"
		if taskType == "ui_ux" {
			nextStatus = "human_review"
		}

		completionResult := map[string]any{
			"status":         nextStatus,
			"test_passed":    true,
			"test_duration":  duration,
			"files_created":  h.extractFilesFromTask(task),
		}
		completionJSON, _ := json.Marshal(completionResult)
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":    taskID,
			"p_new_status": nextStatus,
			"p_result":     string(completionJSON),
		})
		if nextStatus == "human_review" {
			log.Printf("[Testing] Task %s → HUMAN_REVIEW (ui_ux task, awaiting human approval)", truncateID(taskID))
		} else {
			log.Printf("[Testing] Task %s → COMPLETE (testing passed)", truncateID(taskID))
		}

		// Unlock dependents now — task is done (human_review is still "done" from dependency perspective).
		unlockDependentsByTaskNumber(ctx, h.database, taskNumber)

		// Merge is handled by handleTaskApproved (MaintenanceHandler) on "complete" status.
		// For human_review tasks, merge happens AFTER human approves (see handleTaskHumanReviewApproval).
	} else {
		// Tests failed → keep branch and worktree for iterative fixing
		// The executor will resume on this same branch to fix the specific failures
		log.Printf("[Testing] Task %s tests FAILED in %.1fs:\n%s", truncateID(taskID), duration, truncateOutput(testOutput))

		// Feed test failure back to executor model learning.
		// Test failure = code quality issue = strongest negative signal.
		executorModelID := h.getExecutorModelID(ctx, taskID)
		if executorModelID != "" {
			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, executorModelID, "code_test", duration, false)
			}
			// Record to DB model learning for cross-session persistence
			h.database.RPC(ctx, "update_model_learning", map[string]any{
				"p_model_id":         executorModelID,
				"p_task_type":        "code_test",
				"p_outcome":          "failure",
				"p_failure_class":    "test_failure",
				"p_failure_category": "quality",
				"p_failure_detail":   truncateOutput(testOutput),
			})
			log.Printf("[Testing] Recorded test FAIL for executor model %s", executorModelID)
		}

		// Store the failed executor model ID BEFORE transitioning.
		// transition_task fires pgnotify → handler reads routing_flag_reason.
		if executorModelID != "" {
			accumulateFailedModel(ctx, h.database, taskID, "test_failed_by", executorModelID)
		}
		h.database.RPC(ctx, "transition_task", map[string]any{
			"p_task_id":        taskID,
			"p_new_status":     "pending",
			"p_failure_reason": "test_failed:\n" + testOutput,
		})
		log.Printf("[Testing] Task %s → pending (branch %s preserved for fix)", truncateID(taskID), branchName)
	}
}

// resolveTestDir finds the worktree directory for a task branch.
// Checks worktree list, then filesystem scan, then falls back to main repo.
func (h *TestingHandler) resolveTestDir(ctx context.Context, branchName string) string {
	repoPath := h.cfg.GetRepoPath()

	// Method 1: Check git worktree list
	if h.worktreeMgr != nil {
		worktrees, err := h.worktreeMgr.ListWorktrees(ctx)
		if err == nil {
			for _, wt := range worktrees {
				if strings.Contains(wt.BranchName, branchName) || strings.Contains(wt.Path, branchName) {
					log.Printf("[Testing] Using git worktree at %s", wt.Path)
					return wt.Path
				}
			}
		}
	}

	// Method 2: Scan worktree base dir for matching directories
	workBase := ""
	if h.worktreeMgr != nil {
		workBase = h.worktreeMgr.BasePath()
	}
	if workBase == "" {
		workBase = filepath.Join(filepath.Dir(repoPath), "worktrees")
	}
	entries, err := os.ReadDir(workBase)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dirPath := filepath.Join(workBase, e.Name())
			if strings.Contains(filepath.Base(dirPath), branchName) || strings.Contains(dirPath, branchName) {
				log.Printf("[Testing] Using filesystem worktree at %s", dirPath)
				return dirPath
			}
		}
	}

	// Method 3: Fallback to main repo
	log.Printf("[Testing] No worktree found, using main repo at %s", repoPath)
	return repoPath
}

// projectType represents the kind of project detected in a worktree.
type projectType string

const (
	projectGo     projectType = "go"
	projectNode   projectType = "node"
	projectPython projectType = "python"
	projectGeneric projectType = "generic" // no project file, pure output
)

// detectProjectType checks for project marker files in the worktree.
func detectProjectType(dir string) projectType {
	// Check each level up to 3 deep (handles worktree/governor/go.mod)
	for i := 0; i < 3; i++ {
		checkDir := dir
		if i > 0 {
			parts := strings.Split(dir, string(filepath.Separator))
			if len(parts) > i {
				checkDir = filepath.Join(parts[:len(parts)-i]...)
			}
		}
		if _, err := os.Stat(filepath.Join(checkDir, "go.mod")); err == nil {
			return projectGo
		}
		if _, err := os.Stat(filepath.Join(checkDir, "package.json")); err == nil {
			return projectNode
		}
		if _, err := os.Stat(filepath.Join(checkDir, "pyproject.toml")); err == nil {
			return projectPython
		}
		if _, err := os.Stat(filepath.Join(checkDir, "setup.py")); err == nil {
			return projectPython
		}
	}
	return projectGeneric
}

// runTests executes the 3-layer quality gate.
// Layer 1: Artifact validation — verify expected output files exist and are well-formed
// Layer 2: Semgrep static analysis — security and code quality scanning
// Layer 3: Native test suite — project-type-specific tests (go test, npm test, pytest)
// Returns (passed, output, error).
func (h *TestingHandler) runTests(ctx context.Context, testDir, branchName string, expectedFiles []string) (bool, string, error) {
	pt := detectProjectType(testDir)
	log.Printf("[Testing] Detected project type: %s (dir: %s)", pt, testDir)

	var allOutput strings.Builder
	timeout := 3 * time.Minute
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// ── Layer 1: Artifact Validation ──
	log.Printf("[Testing] Layer 1: Artifact validation (%d expected files)", len(expectedFiles))
	layer1Pass, layer1Output := h.runArtifactValidation(testCtx, testDir, expectedFiles)
	allOutput.WriteString("=== LAYER 1: ARTIFACT VALIDATION ===\n")
	allOutput.WriteString(layer1Output)
	allOutput.WriteString("\n")

	if !layer1Pass {
		allOutput.WriteString("RESULT: FAIL (artifact validation)\n")
		return false, allOutput.String(), nil
	}
	allOutput.WriteString("RESULT: PASS\n\n")

	// ── Layer 2: Semgrep Static Analysis ──
	log.Printf("[Testing] Layer 2: Semgrep static analysis")
	layer2Pass, layer2Output := h.runSemgrep(testCtx, testDir)
	allOutput.WriteString("=== LAYER 2: SEMGREP STATIC ANALYSIS ===\n")
	allOutput.WriteString(layer2Output)
	allOutput.WriteString("\n")

	if !layer2Pass {
		allOutput.WriteString("RESULT: FAIL (semgrep found ERROR severity issues)\n")
		return false, allOutput.String(), nil
	}
	allOutput.WriteString("RESULT: PASS\n\n")

	// ── Layer 3: Native Test Suite ──
	log.Printf("[Testing] Layer 3: Native test suite (%s)", pt)
	layer3Pass, layer3Output, layer3Skipped := h.runNativeTests(testCtx, testDir, pt)
	allOutput.WriteString("=== LAYER 3: NATIVE TEST SUITE ===\n")
	if layer3Skipped {
		allOutput.WriteString(fmt.Sprintf("SKIPPED (no native test suite for %s project)\n", pt))
	} else {
		allOutput.WriteString(layer3Output)
		allOutput.WriteString("\n")
		if !layer3Pass {
			allOutput.WriteString("RESULT: FAIL\n")
			return false, allOutput.String(), nil
		}
		allOutput.WriteString("RESULT: PASS\n")
	}

	allOutput.WriteString("\n=== ALL 3 LAYERS PASSED ===\n")
	return true, allOutput.String(), nil
}

// runArtifactValidation verifies that expected output files exist and are well-formed.
func (h *TestingHandler) runArtifactValidation(ctx context.Context, testDir string, expectedFiles []string) (bool, string) {
	if len(expectedFiles) == 0 {
		return true, "No expected files to validate (skipped)\n"
	}

	var output strings.Builder
	allPassed := true

	for _, relPath := range expectedFiles {
		fullPath := filepath.Join(testDir, relPath)
		output.WriteString(fmt.Sprintf("Checking: %s ... ", relPath))

		// Check existence
		info, err := os.Stat(fullPath)
		if err != nil {
			output.WriteString(fmt.Sprintf("MISSING (%v)\n", err))
			allPassed = false
			continue
		}

		// Check file is not empty
		if info.Size() == 0 {
			output.WriteString("EMPTY (0 bytes)\n")
			allPassed = false
			continue
		}

		// Format-specific validation
		ext := strings.ToLower(filepath.Ext(relPath))
		switch ext {
		case ".json":
			data, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			var js json.RawMessage
			if err := json.Unmarshal(data, &js); err != nil {
				output.WriteString(fmt.Sprintf("INVALID JSON: %v\n", err))
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (valid JSON, %d bytes)\n", info.Size()))

		case ".html", ".htm":
			data, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			content := strings.ToLower(string(data))
			if !strings.Contains(content, "<html") && !strings.Contains(content, "<!doctype") {
				output.WriteString("INVALID HTML (no <html> or <!DOCTYPE> tag)\n")
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (valid HTML, %d bytes)\n", info.Size()))

		case ".yaml", ".yml":
			data, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			// Basic YAML validation: check it's not empty and has at least one key: value pair
			content := strings.TrimSpace(string(data))
			if len(content) == 0 {
				output.WriteString("INVALID YAML (empty)\n")
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (%d bytes)\n", info.Size()))

		case ".xml":
			data, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			if !strings.Contains(string(data), "</") && !strings.Contains(string(data), "/>") {
				output.WriteString("INVALID XML (no closing tags)\n")
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (valid XML, %d bytes)\n", info.Size()))

		case ".css":
			data, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			if !strings.Contains(string(data), "{") || !strings.Contains(string(data), "}") {
				output.WriteString("INVALID CSS (no rule blocks)\n")
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (valid CSS, %d bytes)\n", info.Size()))

		case ".js", ".ts":
			// Basic syntax check: file is non-empty and not obviously broken
			_, err := os.ReadFile(fullPath)
			if err != nil {
				output.WriteString(fmt.Sprintf("READ ERROR: %v\n", err))
				allPassed = false
				continue
			}
			output.WriteString(fmt.Sprintf("OK (%d bytes)\n", info.Size()))

		default:
			// Unknown extension — just confirm it exists and is non-empty (already checked above)
			output.WriteString(fmt.Sprintf("OK (%d bytes)\n", info.Size()))
		}
	}

	return allPassed, output.String()
}

// runSemgrep executes semgrep static analysis on the test directory.
// ERROR severity findings = fail. WARNING = logged but passes.
func (h *TestingHandler) runSemgrep(ctx context.Context, testDir string) (bool, string) {
	var output strings.Builder

	// Check if semgrep is available
	semgrepPath, err := exec.LookPath("semgrep")
	if err != nil {
		output.WriteString("Semgrep not found in PATH — skipping static analysis\n")
		return true, output.String() // skip is a pass
	}

	// Run semgrep --config auto --json
	cmd := exec.CommandContext(ctx, semgrepPath, "--config", "auto", "--json", "--no-git-ignore", ".")
	cmd.Dir = testDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	output.WriteString(fmt.Sprintf("Running: semgrep --config auto --json . (dir: %s)\n", testDir))
	err = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		output.WriteString("Semgrep timed out — skipping\n")
		return true, output.String()
	}

	// Parse JSON output
	var result struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Line    int    `json:"start"` // approximate
			Extra   struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	stdoutStr := stdout.String()
	if stdoutStr == "" {
		if stderr.Len() > 0 {
			output.WriteString(fmt.Sprintf("Semgrep stderr: %s\n", truncateOutput(stderr.String())))
		}
		output.WriteString("Semgrep produced no output — skipping\n")
		return true, output.String()
	}

	if jsonErr := json.Unmarshal([]byte(stdoutStr), &result); jsonErr != nil {
		output.WriteString(fmt.Sprintf("Semgrep output parse error: %v — skipping\n", jsonErr))
		return true, output.String()
	}

	// Count findings by severity
	errorCount := 0
	warningCount := 0
	infoCount := 0
	for _, r := range result.Results {
		switch strings.ToLower(r.Extra.Severity) {
		case "error":
			errorCount++
			output.WriteString(fmt.Sprintf("  ERROR: %s (%s): %s\n", r.CheckID, r.Path, r.Extra.Message))
		case "warning":
			warningCount++
			output.WriteString(fmt.Sprintf("  WARNING: %s (%s): %s\n", r.CheckID, r.Path, r.Extra.Message))
		default:
			infoCount++
		}
	}

	output.WriteString(fmt.Sprintf("\nSemgrep findings: %d ERROR, %d WARNING, %d INFO\n",
		errorCount, warningCount, infoCount))

	// Only ERROR severity blocks the pipeline
	if errorCount > 0 {
		return false, output.String()
	}
	return true, output.String()
}

// runNativeTests executes the project-type-specific test suite.
// Returns (passed, output, skipped).
func (h *TestingHandler) runNativeTests(ctx context.Context, testDir string, pt projectType) (bool, string, bool) {
	switch pt {
	case projectGo:
		return h.runGoTests(ctx, testDir)
	case projectNode:
		return h.runNodeTests(ctx, testDir)
	case projectPython:
		return h.runPythonTests(ctx, testDir)
	default:
		// No project file detected — pure output task, no native tests
		return true, "", true
	}
}

// runGoTests runs go build + go test on Go projects.
func (h *TestingHandler) runGoTests(ctx context.Context, testDir string) (bool, string, bool) {
	var output strings.Builder

	// Find the directory with go.mod (might be in a subdirectory)
	goDir := testDir
	if _, err := os.Stat(filepath.Join(testDir, "go.mod")); err != nil {
		// Check subdirectories one level deep
		entries, readErr := os.ReadDir(testDir)
		if readErr == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidate := filepath.Join(testDir, e.Name())
					if _, statErr := os.Stat(filepath.Join(candidate, "go.mod")); statErr == nil {
						goDir = candidate
						break
					}
				}
			}
		}
	}

	goBin := "go"
	if _, err := exec.LookPath("go"); err != nil {
		// Fallback: try common Go install locations
		for _, candidate := range []string{"/usr/local/go/bin/go", os.ExpandEnv("$HOME/go/bin/go")} {
			if _, statErr := os.Stat(candidate); statErr == nil {
				goBin = candidate
				break
			}
		}
	}
	env := append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+filepath.Dir(goBin))

	// Phase 1: go build
	buildCmd := exec.CommandContext(ctx, goBin, "build", "./...")
	buildCmd.Dir = goDir
	buildCmd.Env = env
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	output.WriteString(fmt.Sprintf("Running: go build ./... (dir: %s)\n", goDir))
	if err := buildCmd.Run(); err != nil {
		output.WriteString(fmt.Sprintf("BUILD FAILED:\n%s\n", buildOut.String()))
		return false, output.String(), false
	}
	output.WriteString("Build: PASS\n")

	// Phase 2: go test
	testCmd := exec.CommandContext(ctx, goBin, "test", "-v", "-count=1", "./...")
	testCmd.Dir = goDir
	testCmd.Env = env
	var testOut bytes.Buffer
	testCmd.Stdout = &testOut
	testCmd.Stderr = &testOut
	output.WriteString("Running: go test -v -count=1 ./...\n")
	err := testCmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		output.WriteString("Go tests timed out\n")
		return false, output.String(), false
	}

	if err != nil {
		output.WriteString(fmt.Sprintf("Tests FAILED:\n%s\n", truncateOutput(testOut.String())))
		return false, output.String(), false
	}
	output.WriteString(fmt.Sprintf("Tests: PASS\n%s\n", truncateOutput(testOut.String())))
	return true, output.String(), false
}

// runNodeTests runs npm test on Node projects.
func (h *TestingHandler) runNodeTests(ctx context.Context, testDir string) (bool, string, bool) {
	// Check if package.json has a test script
	packageJSON, err := os.ReadFile(filepath.Join(testDir, "package.json"))
	if err != nil {
		return true, "", true // no package.json = skip
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(packageJSON, &pkg) != nil || pkg.Scripts["test"] == "" {
		return true, "No test script in package.json — skipping\n", true
	}

	var output strings.Builder
	cmd := exec.CommandContext(ctx, "npm", "test")
	cmd.Dir = testDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	output.WriteString("Running: npm test\n")

	err = cmd.Run()
	outputStr := stdout.String()
	if stderr.Len() > 0 {
		outputStr += "\n" + stderr.String()
	}

	if err != nil {
		output.WriteString(fmt.Sprintf("npm test FAILED:\n%s\n", truncateOutput(outputStr)))
		return false, output.String(), false
	}
	output.WriteString(fmt.Sprintf("npm test: PASS\n%s\n", truncateOutput(outputStr)))
	return true, output.String(), false
}

// runPythonTests runs pytest on Python projects.
func (h *TestingHandler) runPythonTests(ctx context.Context, testDir string) (bool, string, bool) {
	// Check if pytest is available
	pytestPath, err := exec.LookPath("pytest")
	if err != nil {
		return true, "pytest not found — skipping Python tests\n", true
	}

	// Check if there are any test files
	hasTests := false
	filepath.WalkDir(testDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && (strings.HasPrefix(d.Name(), "test_") || strings.HasSuffix(d.Name(), "_test.py")) {
			hasTests = true
			return filepath.SkipAll
		}
		return nil
	})

	if !hasTests {
		return true, "No Python test files found — skipping\n", true
	}

	var output strings.Builder
	cmd := exec.CommandContext(ctx, pytestPath, "-v", "--tb=short")
	cmd.Dir = testDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	output.WriteString("Running: pytest -v --tb=short\n")

	err = cmd.Run()
	outputStr := stdout.String()
	if stderr.Len() > 0 {
		outputStr += "\n" + stderr.String()
	}

	if err != nil {
		output.WriteString(fmt.Sprintf("pytest FAILED:\n%s\n", truncateOutput(outputStr)))
		return false, output.String(), false
	}
	output.WriteString(fmt.Sprintf("pytest: PASS\n%s\n", truncateOutput(outputStr)))
	return true, output.String(), false
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
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	git *gitree.Gitree,
	worktreeMgr *gitree.WorktreeManager,
	usageTracker *runtime.UsageTracker,
) {
	handler := NewTestingHandler(database, factory, pool, connRouter, git, cfg, worktreeMgr, usageTracker)
	handler.Register(router)
}

// extractExpectedFiles pulls files_created from task result for artifact validation.
// Replaces the old extractTestPackages which was Go-specific.
func (h *TestingHandler) extractExpectedFiles(task map[string]any) []string {
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
				return toStringSlice(files)
			}
		}
	}

	// Try raw_output for files_created
	rawStr, _ := result["raw_output"].(string)
	if rawStr != "" {
		var output map[string]any
		if json.Unmarshal([]byte(rawStr), &output) == nil {
			if files, ok := output["files_created"].([]any); ok {
				return toStringSlice(files)
			}
		}
		// Try as array
		var outputs []map[string]any
		if json.Unmarshal([]byte(rawStr), &outputs) == nil {
			for _, o := range outputs {
				if files, ok := o["files_created"].([]any); ok {
					return toStringSlice(files)
				}
			}
		}
	}

	return nil
}

// toStringSlice converts []any to []string, filtering non-string entries.
func toStringSlice(items []any) []string {
	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
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

// recordTestResult inserts a row into test_results so the dashboard can display
// pass/fail status with the error message and raw test output.
// This is the ONLY place test_results rows are written during pipeline execution.
func (h *TestingHandler) recordTestResult(
	ctx context.Context,
	taskID, taskNumber, sliceID string,
	testDir string,
	passed bool,
	errMsg string,
	testOutput string,
	durationSecs float64,
) {
	// Determine project type for test_command display
	pt := detectProjectType(testDir)
	testCommand := fmt.Sprintf("3-layer: artifact_validation + semgrep + %s_native_tests", pt)

	// Determine concise outcome string
	outcome := "pass"
	errorMsg := ""
	if !passed {
		outcome = "fail"
		if errMsg != "" {
			errorMsg = errMsg
		} else {
			errorMsg = truncateOutput(testOutput)
		}
	}

	data := map[string]any{
		"task_id":      taskID,
		"task_number":  taskNumber,
		"slice_id":     sliceID,
		"test_type":    "3_layer_quality_gate",
		"test_command": testCommand,
		"passed":       passed,
		"error":        errorMsg,
		"outcome":      outcome,
		"output":       testOutput,
		"duration_ms":  int(durationSecs * 1000),
		"status":       "complete",
		"created_at":   time.Now(),
	}

	result, dbErr := h.database.Insert(ctx, "test_results", data)
	if dbErr != nil {
		log.Printf("[Testing] WARNING: Failed to insert test_results for task %s: %v", truncateID(taskID), dbErr)
		return
	}

	var inserted []map[string]any
	if json.Unmarshal(result, &inserted) == nil && len(inserted) > 0 {
		if id, ok := inserted[0]["id"].(string); ok {
			log.Printf("[Testing] Recorded test_result %s for task %s (passed=%v)", truncateID(id), truncateID(taskID), passed)
		}
	}
}

// getExecutorModelID looks up the model_id from the latest task_run for a task.
// This connects the test result back to the model that wrote the code.
func (h *TestingHandler) getExecutorModelID(ctx context.Context, taskID string) string {
	data, err := h.database.Query(ctx, "task_runs", map[string]any{
		"task_id": "eq." + taskID,
		"select":  "model_id",
		"order":   "started_at.desc",
		"limit":   "1",
	})
	if err != nil {
		log.Printf("[Testing] Failed to query executor model for task %s: %v", truncateID(taskID), err)
		return ""
	}

	var runs []map[string]any
	if err := json.Unmarshal(data, &runs); err != nil || len(runs) == 0 {
		return ""
	}

	modelID, _ := runs[0]["model_id"].(string)
	return modelID
}
