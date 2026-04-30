package gitree

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ManagedRepo handles the governor's own git clone, completely independent
// from any external clone or editing directory. The governor owns this repo
// from cradle to grave -- it creates it, resets it, and cleans it up.
//
// On startup, if the directory doesn't exist, it clones from GitHub.
// If it exists, it fetches and hard-resets to main. No shared state with
// anything else on the system.
type ManagedRepo struct {
	gitree       *Gitree
	repoURL      string // e.g. https://github.com/VibesTribe/VibePilot
	localPath    string // e.g. /home/vibes/.governor/repos/VibesTribe-VibePilot
	githubToken  string
	gitUserEmail string
	gitUserName  string
}

// ManagedRepoConfig configures a self-bootstrapping git repo.
type ManagedRepoConfig struct {
	// GitHubOwner and GitHubRepo identify the remote (e.g. "VibesTribe", "VibePilot").
	GitHubOwner string
	GitHubRepo  string
	// GitHubToken is used for authenticated clone/push.
	GitHubToken string
	// DataDir is the governor's data directory (e.g. /home/vibes/.governor).
	// The repo will be cloned under DataDir/repos/{owner}-{repo}/.
	DataDir string
	// MainBranch defaults to "main".
	MainBranch string
	// ProtectedBranches are branches that cannot be deleted or overwritten.
	ProtectedBranches []string
	// Timeout for git operations.
	Timeout time.Duration
	// GitUserEmail and GitUserName for commits. Defaults to governor@vibepilot.dev / VibePilot Governor.
	GitUserEmail string
	GitUserName  string
	// WorktreeBasePath is where task worktrees are created.
	// Defaults to DataDir/worktrees/{owner}-{repo}/.
	WorktreeBasePath string
}

// NewManagedRepo creates or reinitializes the governor's own clone.
// This is the ONLY way the governor should interact with a project's git repo.
func NewManagedRepo(ctx context.Context, cfg ManagedRepoConfig) (*ManagedRepo, error) {
	if cfg.GitHubOwner == "" || cfg.GitHubRepo == "" {
		return nil, fmt.Errorf("ManagedRepo: GitHubOwner and GitHubRepo are required")
	}

	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, ".governor")
	}
	if cfg.MainBranch == "" {
		cfg.MainBranch = "main"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	repoSlug := cfg.GitHubOwner + "-" + cfg.GitHubRepo
	localPath := filepath.Join(cfg.DataDir, "repos", repoSlug)

	// Build authenticated clone URL
	// GitHub PAT format: https://USERNAME:TOKEN@github.com/owner/repo.git
	// Using just https://TOKEN@ treats it as a username and prompts for password
	var repoURL string
	if cfg.GitHubToken != "" {
		repoURL = fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", cfg.GitHubOwner, cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo)
	} else {
		repoURL = fmt.Sprintf("https://github.com/%s/%s.git", cfg.GitHubOwner, cfg.GitHubRepo)
	}

	mr := &ManagedRepo{
		repoURL:     repoURL,
		localPath:   localPath,
		githubToken:  cfg.GitHubToken,
		gitUserEmail: cfg.GitUserEmail,
		gitUserName:  cfg.GitUserName,
	}

	// Bootstrap: clone if needed, fetch+reset if exists
	if err := mr.bootstrap(ctx, cfg); err != nil {
		return nil, fmt.Errorf("ManagedRepo bootstrap failed: %w", err)
	}

	// Create the Gitree instance that all handlers will use
	gitreeCfg := &Config{
		RepoPath:          localPath,
		ProtectedBranches: cfg.ProtectedBranches,
		Timeout:           cfg.Timeout,
		RemoteName:        "origin",
		MainBranch:        cfg.MainBranch,
	}
	mr.gitree = New(gitreeCfg)

	// Configure git credentials in the local clone for push operations
	mr.configureAuth(ctx)

	return mr, nil
}

// Gitree returns the underlying Gitree for use by handlers, tools, etc.
func (mr *ManagedRepo) Gitree() *Gitree {
	return mr.gitree
}

// LocalPath returns the filesystem path of the managed clone.
func (mr *ManagedRepo) LocalPath() string {
	return mr.localPath
}

// WorktreeBasePath returns the default worktree directory for this repo.
func (mr *ManagedRepo) WorktreeBasePath() string {
	// Derive from localPath: /home/vibes/.governor/repos/VibesTribe-VibePilot -> /home/vibes/.governor/worktrees/VibesTribe-VibePilot
	dataDir := filepath.Dir(filepath.Dir(mr.localPath)) // go up from repos/VibesTribe-VibePilot to .governor
	repoName := filepath.Base(mr.localPath)             // VibesTribe-VibePilot
	return filepath.Join(dataDir, "worktrees", repoName)
}

// Reset ensures the managed repo is on main and up to date with remote.
// Called at startup and can be called at any time to ensure clean state.
func (mr *ManagedRepo) Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Fetch latest
	cmd := exec.CommandContext(ctx, "git", "fetch", "--all")
	cmd.Dir = mr.localPath
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[ManagedRepo] fetch warning: %s", string(out))
	}

	// Hard reset to main via gitree
	return mr.gitree.ResetToMain(ctx)
}

// CleanStaleBranches removes all local branches that aren't main/master.
// Called at startup to ensure no leftover task branches from previous runs.
func (mr *ManagedRepo) CleanStaleBranches(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get all local branches
	cmd := exec.CommandContext(ctx, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	cmd.Dir = mr.localPath
	out, err := cmd.Output()
	if err != nil {
		return
	}

	mainBranch := mr.gitree.MainBranch()
	for _, branch := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		branch = strings.TrimSpace(branch)
		if branch == "" || branch == mainBranch {
			continue
		}
		delCmd := exec.CommandContext(ctx, "git", "branch", "-D", branch)
		delCmd.Dir = mr.localPath
		if err := delCmd.Run(); err != nil {
			log.Printf("[ManagedRepo] Warning: could not delete stale branch %s: %v", branch, err)
		} else {
			log.Printf("[ManagedRepo] Cleaned stale branch: %s", branch)
		}
	}
}

// CleanStaleWorktrees removes all worktrees under this repo's worktree directory.
func (mr *ManagedRepo) CleanStaleWorktrees(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Remove all worktrees via git
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = mr.localPath
	cmd.Run()

	// List remaining worktrees and remove any under our base path
	listCmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	listCmd.Dir = mr.localPath
	out, _ := listCmd.Output()

	basePath := mr.WorktreeBasePath()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			wtPath := strings.TrimPrefix(line, "worktree ")
			if strings.HasPrefix(wtPath, basePath) && wtPath != mr.localPath {
				rmCmd := exec.CommandContext(ctx, "git", "worktree", "remove", wtPath, "--force")
				rmCmd.Dir = mr.localPath
				rmCmd.Run()
				log.Printf("[ManagedRepo] Cleaned stale worktree: %s", wtPath)
			}
		}
	}

	// Prune again after removal
	exec.CommandContext(ctx, "git", "worktree", "prune").Run()
}

// bootstrap clones the repo if it doesn't exist, or fetches+resets if it does.
func (mr *ManagedRepo) bootstrap(ctx context.Context, cfg ManagedRepoConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if _, err := os.Stat(filepath.Join(mr.localPath, ".git")); err == nil {
		// Repo exists -- fetch and verify it's the right remote
		log.Printf("[ManagedRepo] Existing clone at %s, fetching latest...", mr.localPath)

		// Verify remote URL matches
		remoteCmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
		remoteCmd.Dir = mr.localPath
		remoteOut, _ := remoteCmd.Output()
		currentRemote := strings.TrimSpace(string(remoteOut))

		// If remote changed, update it
		expectedRemote := mr.repoURL
		// Compare without token for logging
		if currentRemote != expectedRemote && !strings.Contains(currentRemote, cfg.GitHubOwner+"/"+cfg.GitHubRepo) {
			log.Printf("[ManagedRepo] Remote changed, updating from %s to %s", currentRemote, cfg.GitHubOwner+"/"+cfg.GitHubRepo)
			setCmd := exec.CommandContext(ctx, "git", "remote", "set-url", "origin", mr.repoURL)
			setCmd.Dir = mr.localPath
			if err := setCmd.Run(); err != nil {
				return fmt.Errorf("update remote URL: %w", err)
			}
		}

		// Fetch all
		fetchCmd := exec.CommandContext(ctx, "git", "fetch", "--all")
		fetchCmd.Dir = mr.localPath
		if out, err := fetchCmd.CombinedOutput(); err != nil {
			log.Printf("[ManagedRepo] Fetch warning: %s", string(out))
		}

		// Hard reset to origin/main so the working tree reflects the latest commit.
		// Without this, fetch updates refs but the working tree stays stale —
		// symlinks, config, and prompts can be out of date.
		mainBranch := cfg.MainBranch
		if mainBranch == "" {
			mainBranch = "main"
		}
		resetCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "origin/"+mainBranch)
		resetCmd.Dir = mr.localPath
		if out, err := resetCmd.CombinedOutput(); err != nil {
			log.Printf("[ManagedRepo] Reset warning: %s", string(out))
		} else {
			log.Printf("[ManagedRepo] Reset to origin/%s", mainBranch)
		}

		return nil
	}

	// Need to clone
	log.Printf("[ManagedRepo] Cloning %s/%s into %s...", cfg.GitHubOwner, cfg.GitHubRepo, mr.localPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(mr.localPath), 0755); err != nil {
		return fmt.Errorf("create repo parent dir: %w", err)
	}

	cloneCmd := exec.CommandContext(ctx, "git", "clone", mr.repoURL, mr.localPath)
	var cloneOut strings.Builder
	cloneCmd.Stdout = &cloneOut
	cloneCmd.Stderr = &cloneOut
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("git clone %s/%s: %w - %s", cfg.GitHubOwner, cfg.GitHubRepo, err, cloneOut.String())
	}

	log.Printf("[ManagedRepo] Clone complete")
	return nil
}

// configureAuth sets up git credentials in the local clone for push operations.
func (mr *ManagedRepo) configureAuth(ctx context.Context) {
	if mr.githubToken == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Set the authenticated remote URL for push
	// (fetch can work without auth for public repos, but push always needs it)
	cmd := exec.CommandContext(ctx, "git", "remote", "set-url", "origin", mr.repoURL)
	cmd.Dir = mr.localPath
	cmd.Run()

	// Configure git user for commits (required for push)
	email := mr.gitUserEmail
	if email == "" {
		email = "governor@vibepilot.dev"
	}
	name := mr.gitUserName
	if name == "" {
		name = "VibePilot Governor"
	}
	exec.CommandContext(ctx, "git", "config", "user.email", email).Run()
	exec.CommandContext(ctx, "git", "config", "user.name", name).Run()
}
