package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vibepilot/governor/internal/gitree"
)

type GitCreateBranchTool struct {
	git *gitree.Gitree
}

func NewGitCreateBranchTool(git *gitree.Gitree) *GitCreateBranchTool {
	return &GitCreateBranchTool{git: git}
}

func (t *GitCreateBranchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name parameter required")
	}

	if err := t.git.CreateBranch(ctx, name); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"branch":  name,
		"message": fmt.Sprintf("Branch %s created", name),
	})
}

type GitReadBranchTool struct {
	git *gitree.Gitree
}

func NewGitReadBranchTool(git *gitree.Gitree) *GitReadBranchTool {
	return &GitReadBranchTool{git: git}
}

func (t *GitReadBranchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	branch, ok := args["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("branch parameter required")
	}

	files, err := t.git.ReadBranchOutput(ctx, branch)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"branch":  branch,
		"files":   files,
	})
}

type GitCommitTool struct {
	git *gitree.Gitree
}

func NewGitCommitTool(git *gitree.Gitree) *GitCommitTool {
	return &GitCommitTool{git: git}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	branch, _ := args["branch"].(string)
	output, _ := args["output"]

	if branch == "" {
		return nil, fmt.Errorf("branch parameter required")
	}

	if err := t.git.CommitOutput(ctx, branch, output); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"branch":  branch,
		"message": fmt.Sprintf("Committed to branch %s", branch),
	})
}

type GitMergeTool struct {
	git *gitree.Gitree
}

func NewGitMergeTool(git *gitree.Gitree) *GitMergeTool {
	return &GitMergeTool{git: git}
}

func (t *GitMergeTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	source, ok := args["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source parameter required")
	}
	target, ok := args["target"].(string)
	if !ok {
		return nil, fmt.Errorf("target parameter required")
	}

	if err := t.git.MergeBranch(ctx, source, target); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"source":  source,
		"target":  target,
		"message": fmt.Sprintf("Merged %s into %s", source, target),
	})
}

type GitDeleteBranchTool struct {
	git *gitree.Gitree
}

func NewGitDeleteBranchTool(git *gitree.Gitree) *GitDeleteBranchTool {
	return &GitDeleteBranchTool{git: git}
}

func (t *GitDeleteBranchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name parameter required")
	}

	if err := t.git.DeleteBranch(ctx, name); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"branch":  name,
		"message": fmt.Sprintf("Branch %s deleted", name),
	})
}

type GitClearBranchTool struct {
	git *gitree.Gitree
}

func NewGitClearBranchTool(git *gitree.Gitree) *GitClearBranchTool {
	return &GitClearBranchTool{git: git}
}

func (t *GitClearBranchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name parameter required")
	}

	if err := t.git.ClearBranch(ctx, name); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"branch":  name,
		"message": fmt.Sprintf("Branch %s cleared", name),
	})
}
