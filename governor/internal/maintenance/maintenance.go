package maintenance

import (
	"bytes"
	"context"
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
	if err := m.gitCommand(ctx, "checkout", "-b", branchName).Run(); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return m.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create branch: %w", err)
	}
	return m.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (m *Maintenance) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	if err := m.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	if files, ok := output.(map[string]interface{})["files"]; ok {
		for _, f := range files.([]interface{}) {
			file := f.(map[string]interface{})
			path := file["path"].(string)
			content := file["content"].(string)

			fullPath := filepath.Join(m.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		}
	}

	if err := m.gitCommand(ctx, "add", ".").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	var commitOut bytes.Buffer
	commitCmd := m.gitCommand(ctx, "commit", "-m", "task output")
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		if strings.Contains(commitOut.String(), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %w - %s", err, commitOut.String())
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
	if err := m.gitCommand(ctx, "checkout", targetBranch).Run(); err != nil {
		return fmt.Errorf("checkout target: %w", err)
	}

	if err := m.gitCommand(ctx, "pull", "origin", targetBranch).Run(); err != nil {
	}

	if err := m.gitCommand(ctx, "merge", sourceBranch).Run(); err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	return m.gitCommand(ctx, "push", "origin", targetBranch).Run()
}

func (m *Maintenance) DeleteBranch(ctx context.Context, branchName string) error {
	m.gitCommand(ctx, "branch", "-d", branchName).Run()
	return m.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()
}

func (m *Maintenance) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.repoPath
	return cmd
}
