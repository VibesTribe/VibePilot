package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TestRunner struct {
	stateMachine *StateMachine
	repoPath     string
	sandboxDir   string
	timeoutSecs  int
}

type TestResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
	Coverage *float64 `json:"coverage,omitempty"`
	Duration float64  `json:"duration_seconds"`
	Output   string   `json:"output,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type TestConfig struct {
	Command     string `json:"command"`
	TimeoutSecs int    `json:"timeout_seconds"`
	Coverage    bool   `json:"coverage"`
	FailFast    bool   `json:"fail_fast"`
}

func NewTestRunner(sm *StateMachine, repoPath string, sandboxDir string, timeoutSecs int) *TestRunner {
	return &TestRunner{
		stateMachine: sm,
		repoPath:     repoPath,
		sandboxDir:   sandboxDir,
		timeoutSecs:  timeoutSecs,
	}
}

func (tr *TestRunner) RunTests(ctx context.Context, taskID string, config TestConfig) (*TestResult, error) {
	start := time.Now()

	tr.stateMachine.Emit(ctx, Event{
		Type:      EventTaskTestStarted,
		TaskID:    taskID,
		Timestamp: start,
	})

	result, err := tr.executeTests(ctx, config)
	duration := time.Since(start).Seconds()

	if err != nil {
		tr.stateMachine.Emit(ctx, Event{
			Type:      EventTaskTestCompleted,
			TaskID:    taskID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"passed":   false,
				"error":    err.Error(),
				"duration": duration,
			},
		})
		return nil, fmt.Errorf("execute tests: %w", err)
	}

	result.Duration = duration

	tr.stateMachine.Emit(ctx, Event{
		Type:      EventTaskTestCompleted,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"passed":   result.Passed,
			"failures": result.Failures,
			"duration": duration,
		},
	})

	return result, nil
}

func (tr *TestRunner) executeTests(ctx context.Context, config TestConfig) (*TestResult, error) {
	timeout := tr.timeoutSecs
	if config.TimeoutSecs > 0 {
		timeout = config.TimeoutSecs
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := config.Command
	if cmd == "" {
		cmd = tr.detectTestCommand()
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty test command")
	}

	execCmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	execCmd.Dir = tr.repoPath

	output, err := execCmd.CombinedOutput()
	result := &TestResult{
		Output: string(output),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("test timeout after %d seconds", timeout)
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Passed = false
			result.Failures = tr.parseTestFailures(string(output), exitErr.ExitCode())
			return result, nil
		}
		return nil, fmt.Errorf("run tests: %w", err)
	}

	result.Passed = true
	return result, nil
}

func (tr *TestRunner) detectTestCommand() string {
	packageJSON := filepath.Join(tr.repoPath, "package.json")
	if _, err := exec.LookPath("npm"); err == nil {
		if _, err := os.Stat(packageJSON); err == nil {
			if tr.hasScript("test") {
				return "npm test"
			}
		}
	}

	if _, err := exec.LookPath("go"); err == nil {
		if _, err := os.Stat(filepath.Join(tr.repoPath, "go.mod")); err == nil {
			return "go test ./..."
		}
	}

	if _, err := exec.LookPath("pytest"); err == nil {
		if _, err := os.Stat(filepath.Join(tr.repoPath, "pytest.ini")); err == nil {
			return "pytest"
		}
	}

	if _, err := exec.LookPath("cargo"); err == nil {
		if _, err := os.Stat(filepath.Join(tr.repoPath, "Cargo.toml")); err == nil {
			return "cargo test"
		}
	}

	return "npm test"
}

func (tr *TestRunner) hasScript(script string) bool {
	cmd := exec.Command("npm", "run", "--silent", "--", script)
	cmd.Dir = tr.repoPath
	cmd.Env = append(cmd.Env, "npm_config_loglevel=silent")
	return cmd.Run() == nil
}

func (tr *TestRunner) parseTestFailures(output string, exitCode int) []string {
	var failures []string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "FAIL") ||
			strings.Contains(line, "Error:") ||
			strings.Contains(line, "FAILED") ||
			strings.Contains(line, "✗") ||
			strings.Contains(line, "×") {
			failures = append(failures, line)
		}
	}

	if len(failures) == 0 && exitCode != 0 {
		failures = append(failures, fmt.Sprintf("Tests failed with exit code %d", exitCode))
	}

	if len(failures) > 5 {
		failures = failures[:5]
		failures = append(failures, "... (truncated)")
	}

	return failures
}

func (tr *TestRunner) RunLint(ctx context.Context, taskID string, config TestConfig) (*TestResult, error) {
	start := time.Now()

	timeout := tr.timeoutSecs
	if config.TimeoutSecs > 0 {
		timeout = config.TimeoutSecs
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := "npm run lint"
	if _, err := os.Stat(filepath.Join(tr.repoPath, "package.json")); os.IsNotExist(err) {
		cmd = "go vet ./..."
	}

	parts := strings.Fields(cmd)
	execCmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	execCmd.Dir = tr.repoPath

	output, err := execCmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	result := &TestResult{
		Duration: duration,
		Output:   string(output),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("lint timeout after %d seconds", timeout)
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Passed = false
			result.Failures = tr.parseTestFailures(string(output), exitErr.ExitCode())
			return result, nil
		}
		return nil, fmt.Errorf("run lint: %w", err)
	}

	result.Passed = true
	return result, nil
}

func (tr *TestRunner) RunTypecheck(ctx context.Context, taskID string, config TestConfig) (*TestResult, error) {
	start := time.Now()

	timeout := tr.timeoutSecs
	if config.TimeoutSecs > 0 {
		timeout = config.TimeoutSecs
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := "npm run typecheck"
	if _, err := os.Stat(filepath.Join(tr.repoPath, "package.json")); os.IsNotExist(err) {
		cmd = "go build ./..."
	}

	parts := strings.Fields(cmd)
	execCmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	execCmd.Dir = tr.repoPath

	output, err := execCmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	result := &TestResult{
		Duration: duration,
		Output:   string(output),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("typecheck timeout after %d seconds", timeout)
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Passed = false
			result.Failures = tr.parseTestFailures(string(output), exitErr.ExitCode())
			return result, nil
		}
		return nil, fmt.Errorf("run typecheck: %w", err)
	}

	result.Passed = true
	return result, nil
}

func (tr *TestRunner) ToJSON(result *TestResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
