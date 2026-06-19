package gitree

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProjectResolver is the interface RepoManager needs to look up project configs.
// This avoids a circular dependency on the runtime package.
type ProjectResolver interface {
	GetProjectBySlug(ctx context.Context, slug string) (ProjectInfo, error)
}

// ProjectInfo is the minimal project data RepoManager needs to create a repo.
type ProjectInfo struct {
	Slug             string
	GitHubOwner      string
	GitHubRepo       string
	DefaultBranch    string
	ProtectedBranches []string
}

// RepoManager creates and caches ManagedRepo instances per project.
// The governor creates one at startup with the default (vibepilot) repo,
// and lazily creates repos for other projects when tasks arrive.
//
// Thread-safe: ManagedRepos are created once and cached. Multiple goroutines
// (parallel task workers) can safely call GetOrCreate concurrently.
type RepoManager struct {
	mu       sync.RWMutex
	repos    map[string]*ManagedRepo // keyed by project slug
	default_ *ManagedRepo            // the vibepilot repo, created at startup

	// Config for creating new ManagedRepos
	dataDir          string
	timeout          time.Duration
	gitUserEmail     string
	gitUserName      string
	getGitHubToken   func(ctx context.Context) (string, error)
}

// NewRepoManager creates a RepoManager with the given default repo.
// The default repo is the vibepilot repo created at startup.
func NewRepoManager(defaultRepo *ManagedRepo, opts RepoManagerOptions) *RepoManager {
	m := &RepoManager{
		repos:          make(map[string]*ManagedRepo),
		default_:       defaultRepo,
		dataDir:        opts.DataDir,
		timeout:        opts.Timeout,
		gitUserEmail:   opts.GitUserEmail,
		gitUserName:    opts.GitUserName,
		getGitHubToken: opts.GetGitHubToken,
	}
	// Cache the default repo under its slug for consistency
	if defaultRepo != nil {
		m.repos["vibepilot"] = defaultRepo
	}
	return m
}

// RepoManagerOptions holds the config values needed to create new ManagedRepos.
type RepoManagerOptions struct {
	DataDir        string
	Timeout        time.Duration
	GitUserEmail   string
	GitUserName    string
	GetGitHubToken func(ctx context.Context) (string, error)
}

// Default returns the default (vibepilot) ManagedRepo.
func (m *RepoManager) Default() *ManagedRepo {
	return m.default_
}

// GetOrCreate returns the ManagedRepo for the given project info, creating it
// if this is the first time we've seen this project. If the project slug is
// "vibepilot" or empty, returns the default repo.
func (m *RepoManager) GetOrCreate(ctx context.Context, info ProjectInfo) (*ManagedRepo, error) {
	// Fast path: vibepilot or empty → use default
	if info.Slug == "" || info.Slug == "vibepilot" {
		return m.default_, nil
	}

	// Check cache (read lock)
	m.mu.RLock()
	if repo, ok := m.repos[info.Slug]; ok {
		m.mu.RUnlock()
		return repo, nil
	}
	m.mu.RUnlock()

	// Slow path: create new repo (write lock)
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have created it)
	if repo, ok := m.repos[info.Slug]; ok {
		return repo, nil
	}

	// Get GitHub token for this repo
	token := ""
	if m.getGitHubToken != nil {
		t, err := m.getGitHubToken(ctx)
		if err == nil {
			token = t
		}
	}

	protectedBranches := info.ProtectedBranches
	if len(protectedBranches) == 0 {
		protectedBranches = []string{"main"}
	}

	mainBranch := info.DefaultBranch
	if mainBranch == "" {
		mainBranch = "main"
	}

	repo, err := NewManagedRepo(ctx, ManagedRepoConfig{
		GitHubOwner:       info.GitHubOwner,
		GitHubRepo:        info.GitHubRepo,
		GitHubToken:       token,
		DataDir:           m.dataDir,
		MainBranch:        mainBranch,
		ProtectedBranches: protectedBranches,
		Timeout:           m.timeout,
		GitUserEmail:      m.gitUserEmail,
		GitUserName:       m.gitUserName,
	})
	if err != nil {
		return nil, fmt.Errorf("create managed repo for project %s: %w", info.Slug, err)
	}

	log.Printf("[RepoManager] Created managed repo for project %s: %s/%s at %s",
		info.Slug, info.GitHubOwner, info.GitHubRepo, repo.LocalPath())

	m.repos[info.Slug] = repo
	return repo, nil
}

// HasProjectRepo checks whether a non-default repo has been created for the
// given project slug. Returns true for "vibepilot" (the default repo always exists).
func (m *RepoManager) HasProjectRepo(slug string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.repos[slug]
	return ok
}
