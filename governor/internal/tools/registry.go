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
	httpTimeout := 30
	idleTimeout := 30
	maxIdleConns := 10

	sharedHTTPClient = &http.Client{
		Timeout: time.Duration(httpTimeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       maxIdleConns,
			IdleConnTimeout:    time.Duration(idleTimeout) * time.Second,
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

	sandboxTimeout := 60
	lintTimeout := 60
	typecheckTimeout := 120
	if deps.Config != nil {
		if deps.Config.GetSandboxConfig().TimeoutSeconds > 0 {
			sandboxTimeout = deps.Config.GetSandboxConfig().TimeoutSeconds
		}
		lintTimeout = deps.Config.GetLintTimeoutSecs()
		typecheckTimeout = deps.Config.GetTypecheckTimeoutSecs()
	}

	if deps.RepoPath != "" {
		registry.Register("file_read", NewFileReadTool(deps.RepoPath))
		registry.Register("file_write", NewFileWriteTool(deps.RepoPath))
		registry.Register("file_delete", NewFileDeleteTool(deps.RepoPath))
		registry.Register("sandbox_test", NewSandboxTestTool(deps.RepoPath, sandboxTimeout))
		registry.Register("run_lint", NewRunLintToolWithTimeout(deps.RepoPath, lintTimeout))
		registry.Register("run_typecheck", NewRunTypecheckToolWithTimeout(deps.RepoPath, typecheckTimeout))
	}

	var allowlist []string
	var webConfig *runtime.WebToolsConfig
	if deps.Config != nil {
		allowlist = deps.Config.GetHTTPAllowlist()
		webConfig = deps.Config.GetWebToolsConfig()
	}
	registry.Register("web_search", NewWebSearchTool(allowlist, webConfig))
	registry.Register("web_fetch", NewWebFetchTool(allowlist, webConfig))
}
