package gitree

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DefaultGitTimeout = 60 * time.Second

var branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_/.-]+$`)

func isValidBranchName(name string) bool {
	if name == "" || len(name) > 250 {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.Contains(name, "//") || strings.Contains(name, "..") {
		return false
	}
	return branchNameRegex.MatchString(name)
}

type Gitree struct {
	repoPath          string
	protectedBranches map[string]bool
	timeout           time.Duration
	remoteName        string
}

type Config struct {
	RepoPath          string
	ProtectedBranches []string
	Timeout           time.Duration
	RemoteName        string
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

	remoteName := cfg.RemoteName
	if remoteName == "" {
		remoteName = "origin"
	}

	protected := make(map[string]bool)
	for _, branch := range cfg.ProtectedBranches {
		protected[strings.TrimSpace(branch)] = true
	}

	return &Gitree{
		repoPath:          repoPath,
		protectedBranches: protected,
		timeout:           timeout,
		remoteName:        remoteName,
	}
}

func (g *Gitree) isProtected(branchName string) bool {
	return g.protectedBranches[branchName]
}

func (g *Gitree) isTaskOrModuleBranch(branchName string) bool {
	return strings.HasPrefix(branchName, "task/") ||
		strings.HasPrefix(branchName, "module/") ||
		strings.HasPrefix(branchName, "TEST_MODULES/")
}

func (g *Gitree) CreateBranch(ctx context.Context, branchName string) error {
	return g.CreateBranchFrom(ctx, branchName, "")
}

func (g *Gitree) CreateBranchFrom(ctx context.Context, branchName, sourceBranch string) error {
	if !isValidBranchName(branchName) {
		return fmt.Errorf("invalid branch name: %s", branchName)
	}
	if g.isProtected(branchName) {
		return fmt.Errorf("cannot create branch: '%s' is protected", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	var out bytes.Buffer

	if sourceBranch != "" {
		if err := g.gitCommand(ctx, "fetch", g.remoteName, sourceBranch).Run(); err != nil {
			log.Printf("[Gitree] Warning: fetch %s failed: %v", sourceBranch, err)
		}

		checkoutCmd := g.gitCommand(ctx, "checkout", "-b", branchName, g.remoteName+"/"+sourceBranch)
		checkoutCmd.Stdout = &out
		checkoutCmd.Stderr = &out

		if err := checkoutCmd.Run(); err != nil {
			if strings.Contains(out.String(), "already exists") {
				return g.gitCommand(ctx, "checkout", branchName).Run()
			}
			return fmt.Errorf("create branch from %s: %w - %s", sourceBranch, err, out.String())
		}

		pushCmd := g.gitCommand(ctx, "push", "-u", g.remoteName, branchName)
		var pushOut bytes.Buffer
		pushCmd.Stdout = &pushOut
		pushCmd.Stderr = &pushOut
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("push branch: %w - %s", err, pushOut.String())
		}

		if err := g.gitCommand(ctx, "checkout", "-f", "main").Run(); err != nil {
			log.Printf("[Gitree] Warning: checkout main failed: %v", err)
		}

		return nil
	}

	cmd := g.gitCommand(ctx, "checkout", "--orphan", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			if err := g.gitCommand(ctx, "checkout", "-f", branchName).Run(); err != nil {
				return err
			}
			g.gitCommand(ctx, "clean", "-fd").Run()
			return nil
		}
		return fmt.Errorf("create orphan branch: %w - %s", err, out.String())
	}

	if err := g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run(); err != nil {
		log.Printf("[Gitree] Warning: rm -rf --cached failed: %v", err)
	}

	// Critical: clean working directory after orphan branch creation
	if err := g.gitCommand(ctx, "clean", "-fd").Run(); err != nil {
		log.Printf("[Gitree] Warning: clean -fd failed: %v", err)
	}

	commitCmd := g.gitCommand(ctx, "commit", "--allow-empty", "-m", "Initialize task branch")
	var commitOut bytes.Buffer
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("initial commit: %w - %s", err, commitOut.String())
	}

	// Force push orphan branch (may replace existing remote branch)
	pushCmd := g.gitCommand(ctx, "push", "-f", "-u", g.remoteName, branchName)
	var pushOut bytes.Buffer
	pushCmd.Stdout = &pushOut
	pushCmd.Stderr = &pushOut
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("push branch: %w - %s", err, pushOut.String())
	}

	if err := g.gitCommand(ctx, "checkout", "main").Run(); err != nil {
		log.Printf("[Gitree] Warning: checkout main failed: %v", err)
	}

	return nil
}

func (g *Gitree) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	if !isValidBranchName(branchName) {
		return fmt.Errorf("invalid branch name: %s", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Force checkout to handle uncommitted changes
	var checkoutOut bytes.Buffer
	checkoutCmd := g.gitCommand(ctx, "checkout", "-f", branchName)
	checkoutCmd.Stdout = &checkoutOut
	checkoutCmd.Stderr = &checkoutOut
	if err := checkoutCmd.Run(); err != nil {
		if strings.Contains(checkoutOut.String(), "did not match") || strings.Contains(checkoutOut.String(), "not found") {
			fetchCmd := g.gitCommand(ctx, "fetch", g.remoteName, branchName)
			if fetchErr := fetchCmd.Run(); fetchErr == nil {
				trackCmd := g.gitCommand(ctx, "checkout", "-f", "--track", "origin/"+branchName)
				var trackOut bytes.Buffer
				trackCmd.Stdout = &trackOut
				trackCmd.Stderr = &trackOut
				if trackErr := trackCmd.Run(); trackErr != nil {
					return fmt.Errorf("checkout tracking branch: %w - %s", trackErr, trackOut.String())
				}
			} else {
				return fmt.Errorf("checkout branch (not found locally or remotely): %w - %s", err, checkoutOut.String())
			}
		} else {
			return fmt.Errorf("checkout branch: %w - %s", err, checkoutOut.String())
		}
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
			log.Printf("[Gitree] No changes to commit for %s - task may already be complete", branchName)
			return nil
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
	if !isValidBranchName(sourceBranch) {
		return fmt.Errorf("invalid source branch name: %s", sourceBranch)
	}
	if !isValidBranchName(targetBranch) {
		return fmt.Errorf("invalid target branch name: %s", targetBranch)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout*2)
	defer cancel()

	if g.isProtected(targetBranch) && !g.isTaskOrModuleBranch(sourceBranch) {
		return fmt.Errorf("cannot merge '%s' to protected branch '%s': only module/* branches can merge to protected branches", sourceBranch, targetBranch)
	}

	if err := g.gitCommand(ctx, "checkout", targetBranch).Run(); err != nil {
		log.Printf("[Gitree] Warning: checkout %s failed: %v", targetBranch, err)
	}
	if err := g.gitCommand(ctx, "pull", g.remoteName, targetBranch).Run(); err != nil {
		log.Printf("[Gitree] Warning: pull %s failed: %v", targetBranch, err)
	}

	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "merge", sourceBranch, "--allow-unrelated-histories", "-m", fmt.Sprintf("Merge %s", sourceBranch))
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge: %w - %s", err, out.String())
	}

	return g.gitCommand(ctx, "push", g.remoteName, targetBranch).Run()
}

func (g *Gitree) DeleteBranch(ctx context.Context, branchName string) error {
	if !isValidBranchName(branchName) {
		return fmt.Errorf("invalid branch name: %s", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if g.isProtected(branchName) {
		return fmt.Errorf("cannot delete protected branch: %s", branchName)
	}

	// Ensure we're not on the branch we're trying to delete
	if err := g.gitCommand(ctx, "checkout", "main").Run(); err != nil {
		log.Printf("[Gitree] Warning: checkout main before delete failed: %v", err)
	}

	if err := g.gitCommand(ctx, "branch", "-D", branchName).Run(); err != nil {
		log.Printf("[Gitree] Warning: branch -D %s failed: %v", branchName, err)
	}
	if err := g.gitCommand(ctx, "push", g.remoteName, "--delete", branchName).Run(); err != nil {
		log.Printf("[Gitree] Warning: push --delete %s failed: %v", branchName, err)
	}
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

	if err := g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run(); err != nil {
		log.Printf("[Gitree] Warning: rm -rf --cached failed: %v", err)
	}
	if err := g.gitCommand(ctx, "clean", "-fd").Run(); err != nil {
		log.Printf("[Gitree] Warning: clean -fd failed: %v", err)
	}

	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "commit", "--allow-empty", "-m", "clear for retry")
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		log.Printf("[Gitree] Warning: commit failed: %v", err)
	}

	return g.gitCommand(ctx, "push", "-f").Run()
}

func (g *Gitree) CreateModuleBranch(ctx context.Context, sliceID string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	branchName := "TEST_MODULES/" + sliceID
	var out bytes.Buffer

	if err := g.gitCommand(ctx, "checkout", "-f", "main").Run(); err != nil {
		log.Printf("[Gitree] Warning: checkout main failed: %v", err)
	}

	cmd := g.gitCommand(ctx, "checkout", "--orphan", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			if err := g.gitCommand(ctx, "checkout", "-f", branchName).Run(); err != nil {
				return err
			}
			g.gitCommand(ctx, "clean", "-fd").Run()
			return nil
		}
		return fmt.Errorf("create module branch: %w - %s", err, out.String())
	}

	g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run()
	g.gitCommand(ctx, "clean", "-fd").Run()

	commitCmd := g.gitCommand(ctx, "commit", "--allow-empty", "-m", "Initialize TEST_MODULES/"+sliceID)
	var commitOut bytes.Buffer
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut
	if err := commitCmd.Run(); err != nil {
		log.Printf("[Gitree] Warning: initial commit failed: %v", err)
	}

	if err := g.gitCommand(ctx, "push", "-f", "-u", g.remoteName, branchName).Run(); err != nil {
		return fmt.Errorf("push module branch: %w", err)
	}

	if err := g.gitCommand(ctx, "checkout", "-f", "main").Run(); err != nil {
		log.Printf("[Gitree] Warning: checkout main failed: %v", err)
	}

	return nil
}

func (g *Gitree) CommitAndPush(ctx context.Context, filePath, message string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Pull latest before committing to avoid push rejection when remote is ahead
	// (e.g. dev copy pushed changes, or another pipeline run committed files).
	if err := g.Pull(ctx); err != nil {
		log.Printf("[Gitree] CommitAndPush: pull failed (continuing anyway): %v", err)
		// Non-fatal: we might still be able to push if we're up to date
	}

	if err := g.gitCommand(ctx, "add", filePath).Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	var out bytes.Buffer
	commitCmd := g.gitCommand(ctx, "commit", "-m", message)
	commitCmd.Stdout = &out
	commitCmd.Stderr = &out

	if err := commitCmd.Run(); err != nil {
		outStr := out.String()
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "no changes added") {
			return nil
		}
		return fmt.Errorf("git commit: %w - %s", err, outStr)
	}

	if err := g.gitCommand(ctx, "push", g.remoteName).Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func (g *Gitree) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath
	return cmd
}

// Pull fetches and merges the latest changes from origin main
func (g *Gitree) Pull(ctx context.Context) error {
	cmd := g.gitCommand(ctx, "pull", "--rebase", "origin", "main")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %s: %w", string(out), err)
	}
	return nil
}
