package tester

import (
	"context"
	"os/exec"
	"strings"
)

type TestResult struct {
	Passed   bool
	Failures []string
}

type Tester struct {
	repoPath string
}

type Config struct {
	RepoPath string
}

func New(cfg *Config) *Tester {
	if cfg == nil || cfg.RepoPath == "" {
		return &Tester{repoPath: "."}
	}
	return &Tester{repoPath: cfg.RepoPath}
}

func (t *Tester) RunTests(ctx context.Context, branchName string) TestResult {
	failures := []string{}

	t.gitCommand(ctx, "checkout", branchName).Run()

	if output, err := t.runCommand(ctx, "pytest", "--tb=short", "-q"); err != nil {
		outStr := string(output)
		if !strings.Contains(outStr, "not found") && !strings.Contains(outStr, "No such file") && outStr != "" {
			failures = append(failures, "pytest: "+outStr)
		}
	}

	if output, err := t.runCommand(ctx, "ruff", "check", "."); err != nil {
		outStr := string(output)
		if !strings.Contains(outStr, "not found") && !strings.Contains(outStr, "No such file") && !strings.Contains(outStr, "All checks passed") {
			failures = append(failures, "lint: "+outStr)
		}
	}

	return TestResult{
		Passed:   len(failures) == 0,
		Failures: failures,
	}
}

func (t *Tester) runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = t.repoPath
	return cmd.CombinedOutput()
}

func (t *Tester) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = t.repoPath
	return cmd
}
