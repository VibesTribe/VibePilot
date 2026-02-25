package tester

import (
	"context"
	"log"
	"os/exec"
	"strings"
)

type TestResult struct {
	Passed        bool
	Failures      []string
	LearnedTests  int
	LearnedCaught []string
}

type RuleProvider interface {
	GetTesterRules(ctx context.Context, appliesTo string, limit int) ([]TesterRule, error)
	RecordTesterRuleCaughtBug(ctx context.Context, ruleID string) error
	RecordTesterRuleFalsePositive(ctx context.Context, ruleID string) error
}

type TesterRule struct {
	ID             string
	AppliesTo      string
	TestType       string
	TestCommand    string
	TriggerPattern string
	Priority       int
	CaughtBugs     int
	FalsePositives int
}

type Tester struct {
	repoPath string
	rules    RuleProvider
	maxRules int
}

type Config struct {
	RepoPath string
	MaxRules int
}

func New(cfg *Config) *Tester {
	if cfg == nil {
		cfg = &Config{}
	}
	maxRules := cfg.MaxRules
	if maxRules <= 0 {
		maxRules = 10
	}
	return &Tester{
		repoPath: cfg.RepoPath,
		maxRules: maxRules,
	}
}

func (t *Tester) SetRuleProvider(provider RuleProvider) {
	t.rules = provider
}

func (t *Tester) RunTests(ctx context.Context, branchName string) TestResult {
	return t.RunTestsWithType(ctx, branchName, "")
}

func (t *Tester) RunTestsWithType(ctx context.Context, branchName string, taskType string) TestResult {
	result := TestResult{
		Passed: true,
	}

	t.gitCommand(ctx, "checkout", branchName).Run()

	if output, err := t.runCommand(ctx, "pytest", "--tb=short", "-q"); err != nil {
		result.Passed = false
		result.Failures = append(result.Failures, "pytest: "+t.trimOutput(string(output)))
	}

	if output, err := t.runCommand(ctx, "ruff", "check", "."); err != nil {
		if !strings.Contains(string(output), "All checks passed") {
			result.Passed = false
			result.Failures = append(result.Failures, "lint: "+t.trimOutput(string(output)))
		}
	}

	if t.rules != nil {
		t.runLearnedTests(ctx, taskType, &result)
	}

	return result
}

func (t *Tester) runLearnedTests(ctx context.Context, taskType string, result *TestResult) {
	rules, err := t.rules.GetTesterRules(ctx, taskType, t.maxRules)
	if err != nil {
		log.Printf("Tester: failed to get learned rules: %v", err)
		return
	}

	for _, rule := range rules {
		parts := strings.Fields(rule.TestCommand)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		output, err := t.runCommand(ctx, cmd, args...)
		if err != nil {
			result.Passed = false
			failure := rule.TestType + ": " + t.trimOutput(string(output))
			result.Failures = append(result.Failures, failure)
			result.LearnedCaught = append(result.LearnedCaught, rule.ID)

			if t.rules != nil {
				if err := t.rules.RecordTesterRuleCaughtBug(ctx, rule.ID); err != nil {
					log.Printf("Tester: failed to record caught bug: %v", err)
				}
			}
		} else {
			if t.rules != nil && rule.FalsePositives > rule.CaughtBugs {
				if err := t.rules.RecordTesterRuleFalsePositive(ctx, rule.ID); err != nil {
					log.Printf("Tester: failed to record false positive: %v", err)
				}
			}
		}
		result.LearnedTests++
	}
}

func (t *Tester) trimOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) > 500 {
		return output[:497] + "..."
	}
	return output
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
