package gitree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeManager manages git worktrees for parallel agent execution.
// Each task gets its own directory so multiple agents can work simultaneously
// without overwriting each other's files.
type WorktreeManager struct {
	gitree    *Gitree
	basePath  string // e.g. ~/VibePilot-work/
}

// WorktreeInfo tracks an active worktree.
type WorktreeInfo struct {
	TaskID     string `json:"task_id"`
	BranchName string `json:"branch_name"`
	Path       string `json:"path"`
}

// NewWorktreeManager creates a worktree manager.
// basePath is where worktrees are created (e.g. /home/vibes/VibePilot-work/).
func NewWorktreeManager(g *Gitree, basePath string) *WorktreeManager {
	if basePath == "" {
		home, _ := os.UserHomeDir()
		basePath = filepath.Join(home, "VibePilot-work")
	}
	return &WorktreeManager{
		gitree:   g,
		basePath: basePath,
	}
}

// CreateWorktree creates a git worktree for a task.
// The worktree is a full checkout at basePath/taskID on the given branch.
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, taskID, branchName string) (*WorktreeInfo, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID required for worktree")
	}
	if !isValidBranchName(branchName) {
		return nil, fmt.Errorf("invalid branch name: %s", branchName)
	}

	worktreePath := filepath.Join(wm.basePath, taskID)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree directory exists, check if it's valid
		if wm.isValidWorktree(ctx, worktreePath) {
			return &WorktreeInfo{
				TaskID:     taskID,
				BranchName: branchName,
				Path:       worktreePath,
			}, nil
		}
		// Stale directory, remove it
		os.RemoveAll(worktreePath)
	}

	// Ensure base directory exists
	if err := os.MkdirAll(wm.basePath, 0755); err != nil {
		return nil, fmt.Errorf("create worktree base dir: %w", err)
	}

	// Create the branch if it doesn't exist
	if err := wm.gitree.CreateBranchFrom(ctx, branchName, "main"); err != nil {
		// Branch might already exist, that's OK
		if !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("create branch %s: %w", branchName, err)
		}
	}

	// Create worktree: git worktree add <path> <branch>
	cmd := wm.gitree.gitCommand(ctx, "worktree", "add", worktreePath, branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add %s %s: %w\n%s", worktreePath, branchName, err, string(output))
	}

	return &WorktreeInfo{
		TaskID:     taskID,
		BranchName: branchName,
		Path:       worktreePath,
	}, nil
}

// RemoveWorktree removes a worktree after task completion.
func (wm *WorktreeManager) RemoveWorktree(ctx context.Context, taskID string) error {
	worktreePath := filepath.Join(wm.basePath, taskID)

	// Check if it exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil // Already gone
	}

	// Remove via git worktree remove (cleans up .git/worktrees too)
	cmd := wm.gitree.gitCommand(ctx, "worktree", "remove", worktreePath, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Git might fail if there are uncommitted changes -- force cleanup
		os.RemoveAll(worktreePath)
		// Also prune stale worktree references
		pruneCmd := wm.gitree.gitCommand(ctx, "worktree", "prune")
		pruneCmd.CombinedOutput()
		return fmt.Errorf("git worktree remove: %w\n%s (cleaned up manually)", err, string(output))
	}

	return nil
}

// GetWorktreePath returns the filesystem path for a task's worktree.
func (wm *WorktreeManager) GetWorktreePath(taskID string) string {
	return filepath.Join(wm.basePath, taskID)
}

// ListWorktrees returns all active worktrees.
func (wm *WorktreeManager) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	cmd := wm.gitree.gitCommand(ctx, "worktree", "list", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	return wm.parseWorktreeList(string(output)), nil
}

// PruneWorktrees removes stale worktree references (directories deleted but git still tracks them).
func (wm *WorktreeManager) PruneWorktrees(ctx context.Context) error {
	cmd := wm.gitree.gitCommand(ctx, "worktree", "prune")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune: %w\n%s", err, string(output))
	}
	return nil
}

// CleanAllWorktrees removes all worktrees under the base path.
// Use with caution -- only when shutting down or resetting.
func (wm *WorktreeManager) CleanAllWorktrees(ctx context.Context) error {
	entries, err := os.ReadDir(wm.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worktreePath := filepath.Join(wm.basePath, entry.Name())
		cmd := wm.gitree.gitCommand(ctx, "worktree", "remove", worktreePath, "--force")
		cmd.CombinedOutput() // best effort
	}

	// Prune stale refs
	wm.PruneWorktrees(ctx)

	return nil
}

// isValidWorktree checks if a directory is a valid git worktree.
func (wm *WorktreeManager) isValidWorktree(ctx context.Context, path string) bool {
	// Check for .git file (worktrees have a .git FILE pointing back to main repo)
	gitFile := filepath.Join(path, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		return false
	}
	// In a worktree, .git is a regular file (not a directory)
	return !info.IsDir()
}

// parseWorktreeList parses `git worktree list --porcelain` output.
func (wm *WorktreeManager) parseWorktreeList(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	lines := strings.Split(output, "\n")

	var currentPath, currentBranch string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			currentBranch = strings.TrimPrefix(line, "branch ")
			// refs/heads/ prefix
			currentBranch = strings.TrimPrefix(currentBranch, "refs/heads/")
		} else if line == "" && currentPath != "" {
			// Only include worktrees under our base path
			if strings.HasPrefix(currentPath, wm.basePath) {
				taskID := filepath.Base(currentPath)
				worktrees = append(worktrees, WorktreeInfo{
					TaskID:     taskID,
					BranchName: currentBranch,
					Path:       currentPath,
				})
			}
			currentPath = ""
			currentBranch = ""
		}
	}

	return worktrees
}
