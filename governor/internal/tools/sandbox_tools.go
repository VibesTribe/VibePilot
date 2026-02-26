package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultSandboxTimeoutSecs   = 60
	DefaultLintTimeoutSecs      = 60
	DefaultTypecheckTimeoutSecs = 120
)

type SandboxTestTool struct {
	repoPath       string
	sandboxDir     string
	timeoutSecs    int
	defaultTestCmd string
}

type SandboxConfig struct {
	RepoPath       string
	SandboxDir     string
	TimeoutSecs    int
	DefaultTestCmd string
}

func NewSandboxTestTool(repoPath string, timeoutSecs int) *SandboxTestTool {
	return NewSandboxTestToolWithConfig(&SandboxConfig{
		RepoPath:    repoPath,
		TimeoutSecs: timeoutSecs,
	})
}

func NewSandboxTestToolWithConfig(cfg *SandboxConfig) *SandboxTestTool {
	if cfg == nil {
		cfg = &SandboxConfig{}
	}

	timeout := cfg.TimeoutSecs
	if timeout <= 0 {
		timeout = DefaultSandboxTimeoutSecs
	}

	sandboxDir := cfg.SandboxDir
	if sandboxDir == "" {
		sandboxDir = filepath.Join(os.TempDir(), "vibepilot-sandbox")
	}

	defaultCmd := cfg.DefaultTestCmd
	if defaultCmd == "" {
		defaultCmd = "npm test"
	}

	return &SandboxTestTool{
		repoPath:       cfg.RepoPath,
		sandboxDir:     sandboxDir,
		timeoutSecs:    timeout,
		defaultTestCmd: defaultCmd,
	}
}

func (t *SandboxTestTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	files, ok := args["files"].([]any)
	if !ok || len(files) == 0 {
		return nil, fmt.Errorf("files parameter required (array of {path, content})")
	}

	testCommand, _ := args["test_command"].(string)
	if testCommand == "" {
		testCommand = t.defaultTestCmd
	}

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	sandboxPath := filepath.Join(t.sandboxDir, runID)

	if err := os.MkdirAll(sandboxPath, 0755); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("create sandbox: %v", err),
		})
	}
	defer os.RemoveAll(sandboxPath)

	var createdFiles []string
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		path, _ := fileMap["path"].(string)
		content, _ := fileMap["content"].(string)

		if path == "" {
			continue
		}

		if strings.Contains(path, "..") {
			continue
		}

		fullPath := filepath.Join(sandboxPath, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
		createdFiles = append(createdFiles, path)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.timeoutSecs)*time.Second)
	defer cancel()

	parts := strings.Fields(testCommand)
	if len(parts) == 0 {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   "invalid test command",
		})
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = sandboxPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	success := err == nil
	return json.Marshal(map[string]any{
		"success":   success,
		"exit_code": exitCode,
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"files":     createdFiles,
		"command":   testCommand,
	})
}

type RunLintTool struct {
	repoPath    string
	timeoutSecs int
}

func NewRunLintTool(repoPath string) *RunLintTool {
	return &RunLintTool{repoPath: repoPath, timeoutSecs: DefaultLintTimeoutSecs}
}

func NewRunLintToolWithTimeout(repoPath string, timeoutSecs int) *RunLintTool {
	if timeoutSecs <= 0 {
		timeoutSecs = DefaultLintTimeoutSecs
	}
	return &RunLintTool{repoPath: repoPath, timeoutSecs: timeoutSecs}
}

func (t *RunLintTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.timeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "run", "lint")
	if _, err := os.Stat(filepath.Join(t.repoPath, "package.json")); os.IsNotExist(err) {
		cmd = exec.CommandContext(ctx, "go", "vet", "./...")
	}
	cmd.Dir = filepath.Join(t.repoPath, path)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	success := err == nil

	return json.Marshal(map[string]any{
		"success": success,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
		"path":    path,
	})
}

type RunTypecheckTool struct {
	repoPath    string
	timeoutSecs int
}

func NewRunTypecheckTool(repoPath string) *RunTypecheckTool {
	return &RunTypecheckTool{repoPath: repoPath, timeoutSecs: DefaultTypecheckTimeoutSecs}
}

func NewRunTypecheckToolWithTimeout(repoPath string, timeoutSecs int) *RunTypecheckTool {
	if timeoutSecs <= 0 {
		timeoutSecs = DefaultTypecheckTimeoutSecs
	}
	return &RunTypecheckTool{repoPath: repoPath, timeoutSecs: timeoutSecs}
}

func (t *RunTypecheckTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.timeoutSecs)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "run", "typecheck")
	if _, err := os.Stat(filepath.Join(t.repoPath, "package.json")); os.IsNotExist(err) {
		cmd = exec.CommandContext(ctx, "go", "build", "./...")
	}
	cmd.Dir = filepath.Join(t.repoPath, path)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	success := err == nil

	return json.Marshal(map[string]any{
		"success": success,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
		"path":    path,
	})
}
