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

type SandboxTestTool struct {
	repoPath    string
	sandboxDir  string
	timeoutSecs int
}

func NewSandboxTestTool(repoPath string, timeoutSecs int) *SandboxTestTool {
	if timeoutSecs == 0 {
		timeoutSecs = 60
	}
	return &SandboxTestTool{
		repoPath:    repoPath,
		sandboxDir:  filepath.Join(os.TempDir(), "vibepilot-sandbox"),
		timeoutSecs: timeoutSecs,
	}
}

func (t *SandboxTestTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	files, ok := args["files"].([]any)
	if !ok || len(files) == 0 {
		return nil, fmt.Errorf("files parameter required (array of {path, content})")
	}

	testCommand, _ := args["test_command"].(string)
	if testCommand == "" {
		testCommand = "npm test"
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

type RunLINTTool struct {
	repoPath string
}

func NewRunLintTool(repoPath string) *RunLINTTool {
	return &RunLINTTool{repoPath: repoPath}
}

func (t *RunLINTTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
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
	repoPath string
}

func NewRunTypecheckTool(repoPath string) *RunTypecheckTool {
	return &RunTypecheckTool{repoPath: repoPath}
}

func (t *RunTypecheckTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
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
