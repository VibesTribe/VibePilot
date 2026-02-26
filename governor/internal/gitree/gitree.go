package gitree

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

const DefaultGitTimeout = 60 * time.Second

type Gitree struct {
	repoPath          string
	protectedBranches map[string]bool
	timeout           time.Duration
}

type Config struct {
	RepoPath          string
	ProtectedBranches []string
	Timeout           time.Duration
}

func New(cfg *Config) *Gitree {
	if cfg == nil {
		cfg = &Config{}
	}

	repoPath := cfg.RepoPath
	if repoPath == "" {
		cwd, _ := os.Getwd()
		repoPath = cwd
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultGitTimeout
	}

	protected := make(map[string]bool)
	for _, branch := range cfg.ProtectedBranches {
		protected[strings.TrimSpace(branch)] = true
	}

	return &Gitree{
		repoPath:          repoPath,
		protectedBranches: protected,
		timeout:           timeout,
	}
}

func (g *Gitree) isProtected(branchName string) bool {
	return g.protectedBranches[branchName]
}

func (g *Gitree) isTaskOrModuleBranch(branchName string) bool {
	return strings.HasPrefix(branchName, "task/") || strings.HasPrefix(branchName, "module/")
}

func (g *Gitree) CreateBranch(ctx context.Context, branchName string) error {
	if g.isProtected(branchName) {
		return fmt.Errorf("cannot create branch: '%s' is protected", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	var out bytes.Buffer

	cmd := g.gitCommand(ctx, "checkout", "--orphan", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return g.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create orphan branch: %w - %s", err, out.String())
	}

	g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run()

	return g.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (g *Gitree) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if err := g.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
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

			fullPath := filepath.Join(g.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		}
	}

	if output, ok := outputMap["output"]; ok && outputMap["files"] == nil {
		resultPath := filepath.Join(g.repoPath, "task_output.txt")
		content, _ := json.MarshalIndent(output, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	if response, ok := outputMap["response"]; ok {
		resultPath := filepath.Join(g.repoPath, "task_result.json")
		content, _ := json.MarshalIndent(response, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	if err := g.gitCommand(ctx, "add", ".").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	var commitOut bytes.Buffer
	commitCmd := g.gitCommand(ctx, "commit", "-m", "task output")
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		outStr := commitOut.String()
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "no changes added") {
			return fmt.Errorf("task produced no output: no files were created or modified")
		}
		return fmt.Errorf("git commit: %w - %s", err, outStr)
	}

	return g.gitCommand(ctx, "push").Run()
}

func (g *Gitree) ReadBranchOutput(ctx context.Context, branchName string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if err := g.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return nil, fmt.Errorf("checkout branch: %w", err)
	}

	var files []string
	cmd := g.gitCommand(ctx, "ls-tree", "-r", "--name-only", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-tree: %w", err)
	}

	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

func (g *Gitree) MergeBranch(ctx context.Context, sourceBranch, targetBranch string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout*2)
	defer cancel()

	if g.isProtected(targetBranch) && !g.isTaskOrModuleBranch(sourceBranch) {
		return fmt.Errorf("cannot merge '%s' to protected branch '%s': only module/* branches can merge to protected branches", sourceBranch, targetBranch)
	}

	g.gitCommand(ctx, "checkout", targetBranch).Run()
	g.gitCommand(ctx, "pull", "origin", targetBranch).Run()

	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "merge", sourceBranch)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge: %w - %s", err, out.String())
	}

	return g.gitCommand(ctx, "push", "origin", targetBranch).Run()
}

func (g *Gitree) DeleteBranch(ctx context.Context, branchName string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if g.isProtected(branchName) {
		return fmt.Errorf("cannot delete protected branch: %s", branchName)
	}

	g.gitCommand(ctx, "branch", "-D", branchName).Run()
	g.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()
	return nil
}

func (g *Gitree) ClearBranch(ctx context.Context, branchName string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if g.isProtected(branchName) {
		return fmt.Errorf("cannot clear protected branch: %s", branchName)
	}
	if !g.isTaskOrModuleBranch(branchName) {
		return fmt.Errorf("can only clear task/* or module/* branches, got: %s", branchName)
	}

	if err := g.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run()
	g.gitCommand(ctx, "clean", "-fd").Run()

	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "commit", "--allow-empty", "-m", "clear for retry")
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run()

	return g.gitCommand(ctx, "push", "-f").Run()
}

func (g *Gitree) CreateModuleBranch(ctx context.Context, sliceID string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	branchName := "module/" + sliceID
	var out bytes.Buffer

	cmd := g.gitCommand(ctx, "checkout", "--orphan", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return g.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create module branch: %w - %s", err, out.String())
	}

	g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run()

	return g.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (g *Gitree) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath
	return cmd
}
