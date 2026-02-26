package tools

import (
	"net/http"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/vault"
)

const (
	DefaultHTTPTimeoutSecs     = 30
	DefaultMaxIdleConns        = 10
	DefaultIdleConnTimeoutSecs = 30
)

type Dependencies struct {
	DB       *db.DB
	Git      *gitree.Gitree
	Vault    *vault.Vault
	RepoPath string
	Config   *runtime.Config
}

var sharedHTTPClient *http.Client

func init() {
	sharedHTTPClient = &http.Client{
		Timeout: DefaultHTTPTimeoutSecs * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       DefaultMaxIdleConns,
			IdleConnTimeout:    DefaultIdleConnTimeoutSecs * time.Second,
			DisableCompression: false,
		},
	}
}

func RegisterAll(registry *runtime.ToolRegistry, deps *Dependencies) {
	if deps.Git != nil {
		registry.Register("git_create_branch", NewGitCreateBranchTool(deps.Git))
		registry.Register("git_read_branch", NewGitReadBranchTool(deps.Git))
		registry.Register("git_commit", NewGitCommitTool(deps.Git))
		registry.Register("git_merge", NewGitMergeTool(deps.Git))
		registry.Register("git_delete_branch", NewGitDeleteBranchTool(deps.Git))
		registry.Register("git_clear_branch", NewGitClearBranchTool(deps.Git))
	}

	if deps.DB != nil {
		registry.Register("db_query", NewDBQueryTool(deps.DB))
		registry.Register("db_update", NewDBUpdateTool(deps.DB))
		registry.Register("db_insert", NewDBInsertTool(deps.DB))
		registry.Register("db_rpc", NewDBRPCTool(deps.DB))
		registry.Register("command_maintenance", NewMaintenanceCommandTool(deps.DB))
	}

	if deps.Vault != nil {
		registry.Register("vault_get", NewVaultGetTool(deps.Vault))
	}

	sandboxTimeout := DefaultSandboxTimeoutSecs
	if deps.Config != nil && deps.Config.GetSandboxConfig().TimeoutSeconds > 0 {
		sandboxTimeout = deps.Config.GetSandboxConfig().TimeoutSeconds
	}

	if deps.RepoPath != "" {
		registry.Register("file_read", NewFileReadTool(deps.RepoPath))
		registry.Register("file_write", NewFileWriteTool(deps.RepoPath))
		registry.Register("file_delete", NewFileDeleteTool(deps.RepoPath))
		registry.Register("sandbox_test", NewSandboxTestTool(deps.RepoPath, sandboxTimeout))
		registry.Register("run_lint", NewRunLintTool(deps.RepoPath))
		registry.Register("run_typecheck", NewRunTypecheckTool(deps.RepoPath))
	}

	var allowlist []string
	if deps.Config != nil {
		allowlist = deps.Config.GetHTTPAllowlist()
	}
	registry.Register("web_search", NewWebSearchTool(allowlist))
	registry.Register("web_fetch", NewWebFetchTool(allowlist))
}
