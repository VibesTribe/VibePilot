package tools

import (
	"context"
	"encoding/json"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/vault"
)

type Dependencies struct {
	DB       *db.DB
	Git      *gitree.Gitree
	Vault    *vault.Vault
	RepoPath string
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
		registry.Register("db_rpc", NewDBRPCTool(deps.DB))
		registry.Register("command_maintenance", NewMaintenanceCommandTool(deps.DB))
	}

	if deps.Vault != nil {
		registry.Register("vault_get", NewVaultGetTool(deps.Vault))
	}

	if deps.RepoPath != "" {
		registry.Register("file_read", NewFileReadTool(deps.RepoPath))
		registry.Register("file_write", NewFileWriteTool(deps.RepoPath))
		registry.Register("file_delete", NewFileDeleteTool(deps.RepoPath))
		registry.Register("sandbox_test", NewSandboxTestTool(deps.RepoPath, 60))
		registry.Register("run_lint", NewRunLintTool(deps.RepoPath))
		registry.Register("run_typecheck", NewRunTypecheckTool(deps.RepoPath))
	}

	registry.Register("web_search", NewWebSearchTool())
	registry.Register("web_fetch", NewWebFetchTool())
}

type toolAdapter struct {
	execute func(ctx context.Context, args map[string]any) (json.RawMessage, error)
}

func (a *toolAdapter) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	return a.execute(ctx, args)
}
