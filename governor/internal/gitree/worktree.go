package gitree

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
// The worktree is a full checkout at basePath/taskID on a task/{id}-{slug} branch.
// branchName should be generated via TaskBranchName(taskID, slug).
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

	// Ensure main repo is on main before creating branch/worktree
	// (can't create worktree for a branch that's checked out in main repo)
	if err := wm.gitree.gitCommand(ctx, "checkout", "main").Run(); err != nil {
		log.Printf("[Worktrees] Warning: checkout main failed: %v", err)
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

	// Bootstrap: symlink shared resources (env, config, caches)
	if err := wm.BootstrapWorktree(ctx, worktreePath); err != nil {
		log.Printf("[Worktrees] Warning: bootstrap failed for %s: %v", taskID, err)
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

// TaskBranchName generates a standardized branch name for a task.
// Convention: task/{id}-{slug} (e.g. task/abc123-fix-auth-bug)
func TaskBranchName(taskID, slug string) string {
	if slug == "" {
		return "task/" + taskID
	}
	// Sanitize slug: lowercase, replace spaces/special with hyphens
	slug = strings.ToLower(slug)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	// Truncate to keep branch name reasonable
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return "task/" + taskID + "-" + slug
}

// ShadowMergeResult holds the result of a test merge.
type ShadowMergeResult struct {
	HasConflicts bool     `json:"has_conflicts"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
	CanAutoMerge  bool     `json:"can_auto_merge"`
}

// ShadowMerge performs a dry-run merge to detect conflicts before the real merge.
// This implements the "Parallel Quality Gates" pattern from the Gemini strategy.
func (wm *WorktreeManager) ShadowMerge(ctx context.Context, sourceBranch, targetBranch string) (*ShadowMergeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Fetch latest
	if err := wm.gitree.gitCommand(ctx, "fetch", wm.gitree.remoteName).Run(); err != nil {
		return nil, fmt.Errorf("fetch before shadow merge: %w", err)
	}

	// Use git merge-tree (or merge --no-commit --no-ff) to test
	// First, check for conflicts using git merge-tree (available in git 2.38+)
	cmd := wm.gitree.gitCommand(ctx, "merge-tree", wm.gitree.remoteName+"/"+targetBranch, wm.gitree.remoteName+"/"+sourceBranch)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// merge-tree might not exist in older git, fall back to diff check
		return wm.shadowMergeFallback(ctx, sourceBranch, targetBranch)
	}

	// merge-tree output contains "changed in both" for conflicts
	result := &ShadowMergeResult{
		HasConflicts:  strings.Contains(outputStr, "changed in both") || strings.Contains(outputStr, "CONFLICT"),
		CanAutoMerge:  true,
	}

	if result.HasConflicts {
		result.CanAutoMerge = false
		// Extract conflict file names
		for _, line := range strings.Split(outputStr, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "changed in both") || strings.Contains(line, "CONFLICT") {
				// Try to extract filename from the line
				parts := strings.Fields(line)
				if len(parts) > 0 {
					result.ConflictFiles = append(result.ConflictFiles, parts[len(parts)-1])
				}
			}
		}
	}

	return result, nil
}

// shadowMergeFallback uses git diff to check for potential conflicts
// when merge-tree is not available.
func (wm *WorktreeManager) shadowMergeFallback(ctx context.Context, sourceBranch, targetBranch string) (*ShadowMergeResult, error) {
	// Get list of files changed in source vs target
	cmd := wm.gitree.gitCommand(ctx, "diff", "--name-only", 
		wm.gitree.remoteName+"/"+targetBranch+"..."+wm.gitree.remoteName+"/"+sourceBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ShadowMergeResult{CanAutoMerge: true}, nil // Assume OK if we can't check
	}

	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	var conflicts []string

	for _, file := range changedFiles {
		if file == "" {
			continue
		}
		// Check if file was also modified on target
		diffCmd := wm.gitree.gitCommand(ctx, "log", "--oneline", "-1",
			wm.gitree.remoteName+"/"+targetBranch, "--", file)
		diffOut, _ := diffCmd.CombinedOutput()
		if len(strings.TrimSpace(string(diffOut))) > 0 {
			conflicts = append(conflicts, file)
		}
	}

	result := &ShadowMergeResult{
		HasConflicts:  len(conflicts) > 0,
		ConflictFiles: conflicts,
		CanAutoMerge:  len(conflicts) == 0,
	}

	return result, nil
}

// BootstrapWorktree symlinks shared resources into a new worktree.
// This implements the "Strategic Context Injection" pattern from Gemini.
// It ensures agents have API keys, caches, and config available immediately.
func (wm *WorktreeManager) BootstrapWorktree(ctx context.Context, worktreePath string) error {
	// Shared resources to symlink from main repo into worktree
	// These are the files agents need to do their work
	sharedFiles := []string{
		"governor/config/system.json",    // Runtime config (connectors, worktrees, MCP)
		"governor/config/models.json",    // Model definitions
		"governor/config/routing.json",   // Routing rules
		"governor/config/agents.json",    // Agent definitions
		"governor/config/connectors.json", // Connector configs
		"governor/config/categories.json", // Task categories
		"governor/config/tools.json",     // Tool definitions
		".hermes.md",                     // Enforcement rules (priority 1)
		".context/boot.md",               // Boot context
		".context/map.md",                // Go function signatures
		".context/index.db",              // Code symbol index
		".context/knowledge.db",          // Rules, prompts, configs, docs, schema
	}
	// Symlink entire directories
	sharedDirs := []string{
		"governor/config/prompts",        // Agent role prompt templates
		"governor/config/pipelines",      // Pipeline definitions
		".context/tools",                 // Build tools, tier0 rules
	}

	// Find main repo path (parent of worktrees)
	mainRepoPath := wm.gitree.repoPath

	for _, shared := range sharedFiles {
		source := filepath.Join(mainRepoPath, shared)
		target := filepath.Join(worktreePath, shared)

		// Skip if source doesn't exist
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		}

		// Skip if target already exists (don't overwrite)
		if _, err := os.Stat(target); err == nil {
			continue
		}

		// Ensure target directory exists
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			log.Printf("[Worktrees] Warning: mkdir for %s: %v", target, err)
			continue
		}

		// Create symlink
		if err := os.Symlink(source, target); err != nil {
			log.Printf("[Worktrees] Warning: symlink %s -> %s: %v", source, target, err)
		}
	}

	// Symlink shared directories (prompts, pipelines, tools)
	for _, shared := range sharedDirs {
		source := filepath.Join(mainRepoPath, shared)
		target := filepath.Join(worktreePath, shared)

		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			log.Printf("[Worktrees] Warning: mkdir for %s: %v", target, err)
			continue
		}
		if err := os.Symlink(source, target); err != nil {
			log.Printf("[Worktrees] Warning: symlink dir %s -> %s: %v", source, target, err)
		}
	}

	return nil
}
