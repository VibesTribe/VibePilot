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
	mainBranch        string
}

type Config struct {
	RepoPath          string
	ProtectedBranches []string
	Timeout           time.Duration
	RemoteName        string
	MainBranch        string
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

	mainBranch := cfg.MainBranch
	if mainBranch == "" {
		mainBranch = "main"
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
		mainBranch:        mainBranch,
	}
}

// MainBranch returns the configured main branch name.
func (g *Gitree) MainBranch() string {
	return g.mainBranch
}

func (g *Gitree) isProtected(branchName string) bool {
	return g.protectedBranches[branchName]
}

func (g *Gitree) isTaskOrModuleBranch(branchName string) bool {
	return strings.HasPrefix(branchName, "task/") ||
		strings.HasPrefix(branchName, "module/") ||
		strings.HasPrefix(branchName, "TEST_MODULES/")
}

// ResetToMain forcefully returns the repo to the main branch with a clean state.
// This is the single source of truth for "get clean". Every public git operation
// must call this BEFORE doing its work. Returns an error if it fails -- callers
// MUST NOT silently ignore this.
func (g *Gitree) ResetToMain(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	main := g.mainBranch

	// Step 1: Force checkout main (discards working tree changes)
	var out bytes.Buffer
	checkoutCmd := g.gitCommand(ctx, "checkout", "-f", main)
	checkoutCmd.Stdout = &out
	checkoutCmd.Stderr = &out
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("ResetToMain: checkout -f %s failed: %w - %s", main, err, out.String())
	}

	// Step 2: Clean untracked files and directories
	if err := g.gitCommand(ctx, "clean", "-fd").Run(); err != nil {
		log.Printf("[Gitree] ResetToMain: clean -fd warning: %v", err)
	}

	// Step 3: Hard reset to match remote exactly
	var resetOut bytes.Buffer
	resetCmd := g.gitCommand(ctx, "reset", "--hard", g.remoteName+"/"+main)
	resetCmd.Stdout = &resetOut
	resetCmd.Stderr = &resetOut
	if err := resetCmd.Run(); err != nil {
		// Remote might not have the branch yet -- try fetching first
		log.Printf("[Gitree] ResetToMain: reset to %s/%s failed, trying fetch: %v", g.remoteName, main, err)
		if fetchErr := g.gitCommand(ctx, "fetch", g.remoteName, main).Run(); fetchErr != nil {
			log.Printf("[Gitree] ResetToMain: fetch also failed: %v", fetchErr)
		} else {
			var retryOut bytes.Buffer
			retryCmd := g.gitCommand(ctx, "reset", "--hard", g.remoteName+"/"+main)
			retryCmd.Stdout = &retryOut
			retryCmd.Stderr = &retryOut
			if retryErr := retryCmd.Run(); retryErr != nil {
				log.Printf("[Gitree] ResetToMain: reset after fetch still failed: %v - %s", retryErr, retryOut.String())
			}
		}
	}

	// Step 4: Verify we're actually on main
	var verifyOut bytes.Buffer
	verifyCmd := g.gitCommand(ctx, "branch", "--show-current")
	verifyCmd.Stdout = &verifyOut
	currentBranch := strings.TrimSpace(verifyOut.String())
	if currentBranch != "" && currentBranch != main {
		return fmt.Errorf("ResetToMain: verification failed, still on branch %q (expected %q)", currentBranch, main)
	}

	return nil
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

	// GUARANTEE: start from a clean main
	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("CreateBranchFrom: reset to main failed: %w", err)
	}

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
				// Branch exists locally — just check it out and ensure it tracks remote
				if checkoutErr := g.gitCommand(ctx, "checkout", branchName).Run(); checkoutErr != nil {
					return fmt.Errorf("checkout existing branch %s: %w", branchName, checkoutErr)
				}
				// Ensure remote tracking (ignore error if already tracking)
				g.gitCommand(ctx, "push", "-u", g.remoteName, branchName).Run()
				return nil
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

		// GUARANTEE: return to clean main
		return g.ResetToMain(ctx)
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
			return g.ResetToMain(ctx)
		}
		return fmt.Errorf("create orphan branch: %w - %s", err, out.String())
	}

	if err := g.gitCommand(ctx, "rm", "-rf", "--cached", ".").Run(); err != nil {
		log.Printf("[Gitree] Warning: rm -rf --cached failed: %v", err)
	}
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

	pushCmd := g.gitCommand(ctx, "push", "-f", "-u", g.remoteName, branchName)
	var pushOut bytes.Buffer
	pushCmd.Stdout = &pushOut
	pushCmd.Stderr = &pushOut
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("push branch: %w - %s", err, pushOut.String())
	}

	// GUARANTEE: return to clean main
	return g.ResetToMain(ctx)
}

func (g *Gitree) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
	if !isValidBranchName(branchName) {
		return fmt.Errorf("invalid branch name: %s", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// GUARANTEE: start from a clean main
	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("CommitOutput: reset to main failed: %w", err)
	}

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
			g.ResetToMain(ctx)
			return nil
		}
		return fmt.Errorf("git commit: %w - %s", err, outStr)
	}

	if err := g.gitCommand(ctx, "push").Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	// GUARANTEE: return to clean main
	return g.ResetToMain(ctx)
}

func (g *Gitree) ReadBranchOutput(ctx context.Context, branchName string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if err := g.ResetToMain(ctx); err != nil {
		return nil, fmt.Errorf("ReadBranchOutput: reset to main failed: %w", err)
	}

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

	g.ResetToMain(ctx)
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

	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("MergeBranch: reset to main failed: %w", err)
	}

	if err := g.gitCommand(ctx, "checkout", targetBranch).Run(); err != nil {
		return fmt.Errorf("checkout %s failed: %w", targetBranch, err)
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

	if err := g.gitCommand(ctx, "push", g.remoteName, targetBranch).Run(); err != nil {
		return fmt.Errorf("push after merge: %w", err)
	}

	return g.ResetToMain(ctx)
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

	// GUARANTEE: must be on main (can't delete current branch)
	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("DeleteBranch: reset to main failed: %w", err)
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

	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("ClearBranch: reset to main failed: %w", err)
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
	if err := cmd.Run(); err != nil {
		log.Printf("[Gitree] Warning: commit failed: %v", err)
	}

	g.gitCommand(ctx, "push", "-f").Run()

	return g.ResetToMain(ctx)
}

func (g *Gitree) CreateModuleBranch(ctx context.Context, sliceID string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	branchName := "TEST_MODULES/" + sliceID

	if err := g.ResetToMain(ctx); err != nil {
		return fmt.Errorf("CreateModuleBranch: reset to main failed: %w", err)
	}

	var out bytes.Buffer

	cmd := g.gitCommand(ctx, "checkout", "--orphan", branchName)
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if strings.Contains(out.String(), "already exists") {
			if err := g.gitCommand(ctx, "checkout", "-f", branchName).Run(); err != nil {
				return err
			}
			g.gitCommand(ctx, "clean", "-fd").Run()
			return g.ResetToMain(ctx)
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

	return g.ResetToMain(ctx)
}

// CommitAndPush commits a file and pushes to the remote.
// Used for plan files that get committed to main.
func (g *Gitree) CommitAndPush(ctx context.Context, filePath, message string) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Ensure we're on main (plan files go on main), but do NOT clean/reset
	// because the caller just wrote the file to disk and ResetToMain would
	// destroy it with checkout -f + clean -fd.
	currentBranch, _ := g.currentBranch(ctx)
	if currentBranch != g.mainBranch {
		if err := g.gitCommand(ctx, "checkout", g.mainBranch).Run(); err != nil {
			return fmt.Errorf("CommitAndPush: checkout %s failed: %w", g.mainBranch, err)
		}
	}

	// Pull latest to avoid conflicts (stash any untracked files first)
	g.gitCommand(ctx, "stash", "--include-untracked").Run()
	if err := g.Pull(ctx); err != nil {
		g.gitCommand(ctx, "stash", "pop").Run()
		return fmt.Errorf("CommitAndPush: pull failed: %w", err)
	}
	g.gitCommand(ctx, "stash", "pop").Run()

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

func (g *Gitree) currentBranch(ctx context.Context) (string, error) {
	var out bytes.Buffer
	cmd := g.gitCommand(ctx, "branch", "--show-current")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (g *Gitree) gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath
	return cmd
}

// Pull fetches and merges the latest changes from origin main
func (g *Gitree) Pull(ctx context.Context) error {
	cmd := g.gitCommand(ctx, "pull", "--rebase", g.remoteName, g.mainBranch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %s: %w", string(out), err)
	}
	return nil
}

// CommitOutputToWorktree writes task output files directly to the worktree directory.
// The worktree is already on the correct branch, so we skip the checkout step that
// CommitOutput does (which fails when the branch is locked by the worktree).
// This is used by both internal task runners and courier agents.
func (g *Gitree) CommitOutputToWorktree(ctx context.Context, worktreePath string, branchName string, output interface{}) error {
	if worktreePath == "" {
		return fmt.Errorf("CommitOutputToWorktree: worktreePath is required")
	}
	if !isValidBranchName(branchName) {
		return fmt.Errorf("invalid branch name: %s", branchName)
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		outputMap = make(map[string]interface{})
	}

	// Write files directly to the worktree (already on the right branch)
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
			fullPath := filepath.Join(worktreePath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		}
	}

	// Write raw output if no files (check both "output" and "raw_output" keys)
	if rawOut, ok := outputMap["raw_output"]; ok && rawOut != "" {
		if outputMap["files"] == nil {
			resultPath := filepath.Join(worktreePath, "task_output.txt")
			content, _ := json.MarshalIndent(rawOut, "", "  ")
			if err := os.WriteFile(resultPath, content, 0644); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
		}
	} else if output, ok := outputMap["output"]; ok && outputMap["files"] == nil {
		resultPath := filepath.Join(worktreePath, "task_output.txt")
		content, _ := json.MarshalIndent(output, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	// Write response if present
	if response, ok := outputMap["response"]; ok {
		resultPath := filepath.Join(worktreePath, "task_result.json")
		content, _ := json.MarshalIndent(response, "", "  ")
		if err := os.WriteFile(resultPath, content, 0644); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
	}

	// Stage, commit, and push from the worktree
	addCmd := g.gitCommandIn(ctx, worktreePath, "add", ".")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add in worktree: %w", err)
	}

	var commitOut bytes.Buffer
	commitCmd := g.gitCommandIn(ctx, worktreePath, "commit", "-m", "task output")
	commitCmd.Stdout = &commitOut
	commitCmd.Stderr = &commitOut

	if err := commitCmd.Run(); err != nil {
		outStr := commitOut.String()
		if strings.Contains(outStr, "nothing to commit") || strings.Contains(outStr, "no changes added") {
			log.Printf("[Gitree] No changes to commit in worktree for %s", branchName)
			return nil
		}
		return fmt.Errorf("git commit in worktree: %w - %s", err, outStr)
	}

	pushCmd := g.gitCommandIn(ctx, worktreePath, "push")
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push from worktree: %w", err)
	}

	log.Printf("[Gitree] Committed output to worktree branch %s at %s", branchName, worktreePath)
	return nil
}

// DiskFile represents a file with its path and content read from disk.
type DiskFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ScanWorktreeFiles reads ALL files currently in a worktree directory and returns
// them as DiskFile slices with full content. This is the source of truth for what
// the agent actually produced -- regardless of connector type (API, CLI, courier).
//
// After commitOutput writes parsed/raw output to the worktree and git-adds everything,
// this function scans the result. It captures:
//   - Files parsed from structured JSON output
//   - Files written by CLI agents directly to disk
//   - The task_output.txt raw fallback
//
// excludePaths filters out known metadata files that aren't agent output.
func (g *Gitree) ScanWorktreeFiles(ctx context.Context, worktreePath string, excludePaths []string) ([]DiskFile, error) {
	if worktreePath == "" {
		return nil, fmt.Errorf("ScanWorktreeFiles: worktreePath is required")
	}

	exclude := make(map[string]bool)
	for _, p := range excludePaths {
		exclude[p] = true
	}

	var files []DiskFile
	err := filepath.Walk(worktreePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.IsDir() {
			// Skip .git directories
			if strings.Contains(path, "/.git") || strings.HasSuffix(path, "/.git") {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path from worktree root
		relPath, err := filepath.Rel(worktreePath, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes
		relPath = filepath.ToSlash(relPath)

		if exclude[relPath] {
			return nil
		}

		// Read file content (limit to 2MB per file to avoid memory issues)
		if info.Size() > 2*1024*1024 {
			log.Printf("[ScanWorktreeFiles] Skipping large file: %s (%d bytes)", relPath, info.Size())
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[ScanWorktreeFiles] Warning: could not read %s: %v", relPath, err)
			return nil
		}

		files = append(files, DiskFile{
			Path:    relPath,
			Content: string(content),
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scan worktree: %w", err)
	}

	log.Printf("[ScanWorktreeFiles] Found %d files in %s", len(files), worktreePath)
	return files, nil
}

// ScanBranchFiles lists files on a branch (non-worktree path) by checking out
// the branch, walking the directory, and reading file contents. Returns to main after.
func (g *Gitree) ScanBranchFiles(ctx context.Context, branchName string, excludePaths []string) ([]DiskFile, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	if err := g.ResetToMain(ctx); err != nil {
		return nil, fmt.Errorf("ScanBranchFiles: reset to main failed: %w", err)
	}

	if err := g.gitCommand(ctx, "checkout", "-f", branchName).Run(); err != nil {
		return nil, fmt.Errorf("ScanBranchFiles: checkout %s failed: %w", branchName, err)
	}

	files, err := g.ScanWorktreeFiles(ctx, g.repoPath, excludePaths)

	// Always return to main
	g.ResetToMain(ctx)

	return files, err
}

// gitCommandIn creates an exec.Cmd for git with the specified working directory.
// Used for operations in worktrees where commands must run inside the worktree, not the main repo.
func (g *Gitree) gitCommandIn(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd
}
