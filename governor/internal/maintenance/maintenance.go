package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Maintenance struct {
	repoPath string
}

type Config struct {
	RepoPath string
}

func New(cfg *Config) *Maintenance {
	if cfg == nil || cfg.RepoPath == "" {
		cwd, _ := os.Getwd()
		return &Maintenance{repoPath: cwd}
	}
	return &Maintenance{repoPath: cfg.RepoPath}
}

func (m *Maintenance) CreateBranch(ctx context.Context, branchName string) error {
	var out bytes.Buffer
	cmd := m.gitCommand(ctx, "checkout", "-b", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return m.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create branch: %w - %s", err, out.String())
	}
	return m.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (m *Maintenance) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	if err := m.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		outputMap = make(map[string]interface{})
	}

	if files, ok := outputMap["files"]; ok {
		filesList, ok := files.([]interface{})
		if !ok {
			return fmt.Errorf("files must be an array")
		}
		for _, f := range filesList {
			file, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			path, ok := file["path"].(string)
			if !ok {
				continue
			}
			content, ok := file["content"].(string)
			if !ok {
				continue
			}

			fullPath := filepath.Join(m.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		}
	}

	if output, ok := outputMap["output"]; ok && outputMap["files"] == nil {
		resultPath := filepath.Join(m.repoPath, "task_output.txt")
		content, _ := json.MarshalIndent(output, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	if response, ok := outputMap["response"]; ok {
		resultPath := filepath.Join(m.repoPath, "task_result.json")
		content, _ := json.MarshalIndent(response, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	m.gitCommand(ctx, "add", ".").Run()

	var commitOut bytes.Buffer
	commitCmd := m.gitCommand(ctx, "commit", "-m", "task output")
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		outStr := commitOut.String()
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "no changes added") {
			return fmt.Errorf("task produced no output: no files were created or modified")
		}
		return fmt.Errorf("git commit: %w - %s", err, outStr)
	}

	return m.gitCommand(ctx, "push").Run()
}

func (m *Maintenance) ReadBranchOutput(ctx context.Context, branchName string) ([]string, error) {
	var files []string

	cmd := m.gitCommand(ctx, "diff", "--name-only", "main..."+branchName)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

func (m *Maintenance) MergeBranch(ctx context.Context, sourceBranch, targetBranch string) error {
	m.gitCommand(ctx, "checkout", targetBranch).Run()
	m.gitCommand(ctx, "pull", "origin", targetBranch).Run()

	var out bytes.Buffer
	cmd := m.gitCommand(ctx, "merge", sourceBranch)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge: %w - %s", err, out.String())
	}

	return m.gitCommand(ctx, "push", "origin", targetBranch).Run()
}

func (m *Maintenance) DeleteBranch(ctx context.Context, branchName string) error {
	m.gitCommand(ctx, "branch", "-d", branchName).Run()
	m.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()
	return nil
}

func (m *Maintenance) ExecuteMerge(ctx context.Context, taskID, branchName string) error {
	if err := m.MergeBranch(ctx, branchName, "main"); err != nil {
		return err
	}

	m.DeleteBranch(ctx, branchName)
	return nil
}

func (m *Maintenance) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.repoPath
	return cmd
}
