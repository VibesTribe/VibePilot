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
)

type Gitree struct {
	repoPath string
}

type Config struct {
	RepoPath string
}

func New(cfg *Config) *Gitree {
	if cfg == nil || cfg.RepoPath == "" {
		cwd, _ := os.Getwd()
		return &Gitree{repoPath: cwd}
	}
	return &Gitree{repoPath: cfg.RepoPath}
}

func (g *Gitree) CreateBranch(ctx context.Context, branchName string) error {
	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "checkout", "-b", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return g.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create branch: %w - %s", err, out.String())
	}
	return g.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (g *Gitree) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
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

func (g *Gitree) ReadBranchOutput(ctx context.Context, branchName string, baseBranch string) ([]string, error) {
	if baseBranch == "" {
		baseBranch = "main"
	}

	var files []string
	cmd := g.gitCommand(ctx, "diff", "--name-only", baseBranch+"..."+branchName)
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

func (g *Gitree) MergeBranch(ctx context.Context, sourceBranch, targetBranch string) error {
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
	g.gitCommand(ctx, "branch", "-d", branchName).Run()
	g.gitCommand(ctx, "push", "origin", "--delete", branchName).Run()
	return nil
}

func (g *Gitree) ClearBranch(ctx context.Context, branchName string, baseBranch string) error {
	if baseBranch == "" {
		baseBranch = "main"
	}

	if err := g.gitCommand(ctx, "checkout", branchName).Run(); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "reset", "--hard", baseBranch)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reset branch: %w - %s", err, out.String())
	}

	return g.gitCommand(ctx, "push", "-f").Run()
}

func (g *Gitree) CreateModuleBranch(ctx context.Context, sliceID string) error {
	branchName := "module/" + sliceID
	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "checkout", "-b", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			return g.gitCommand(ctx, "checkout", branchName).Run()
		}
		return fmt.Errorf("create module branch: %w - %s", err, out.String())
	}
	return g.gitCommand(ctx, "push", "-u", "origin", branchName).Run()
}

func (g *Gitree) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath
	return cmd
}
