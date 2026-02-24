package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type Role struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Prompt                 string   `json:"prompt"`
	Skills                 []string `json:"skills"`
	Tools                  []string `json:"tools"`
	RequiresDestination    string   `json:"requires_destination"`
	RequiresModelCap       []string `json:"requires_model_capability"`
	DefaultDestination     string   `json:"default_destination"`
	DefaultModel           string   `json:"default_model"`
	RequiresMultipleModels bool     `json:"requires_multiple_models"`
	ApprovalRequired       bool     `json:"approval_required"`
	Notes                  string   `json:"notes"`
}

type Agent struct {
	Role     *Role
	Prompt   string
	ModelID  string
	RunnerID string
	Context  map[string]interface{}
	Tools    []string
}

type Result struct {
	Success bool
	Output  interface{}
	Error   string
	Tokens  TokenUsage
}

type TokenUsage struct {
	Input  int
	Output int
	Total  int
}

type Runtime struct {
	roles    *RoleRegistry
	prompts  *PromptManager
	executor Executor
}

type Executor interface {
	Execute(ctx context.Context, role *Role, prompt string, context map[string]interface{}) (*Result, error)
}

func NewRuntime(rolesPath, promptsDir string, executor Executor) (*Runtime, error) {
	roles, err := LoadRoles(rolesPath)
	if err != nil {
		return nil, fmt.Errorf("load roles: %w", err)
	}

	prompts := NewPromptManager(promptsDir)

	return &Runtime{
		roles:    roles,
		prompts:  prompts,
		executor: executor,
	}, nil
}

func (r *Runtime) Execute(ctx context.Context, roleID string, context map[string]interface{}) (*Result, error) {
	role := r.roles.Get(roleID)
	if role == nil {
		return nil, fmt.Errorf("role not found: %s", roleID)
	}

	prompt, err := r.prompts.Get(role.Prompt)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	return r.executor.Execute(ctx, role, prompt, context)
}

func (r *Runtime) ExecuteParallel(ctx context.Context, requests []Request) ([]*Result, error) {
	results := make([]*Result, len(requests))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, req Request) {
			defer wg.Done()

			result, err := r.Execute(ctx, req.RoleID, req.Context)

			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			results[idx] = result
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()
	return results, firstErr
}

func (r *Runtime) ExecuteSequential(ctx context.Context, requests []Request) ([]*Result, error) {
	results := make([]*Result, len(requests))

	for i, req := range requests {
		result, err := r.Execute(ctx, req.RoleID, req.Context)
		if err != nil {
			return results[:i], err
		}
		results[i] = result
	}

	return results, nil
}

type Request struct {
	RoleID  string
	Context map[string]interface{}
}

func (r *Role) HasSkill(skillID string) bool {
	for _, s := range r.Skills {
		if s == skillID {
			return true
		}
	}
	return false
}

func (r *Role) HasTool(toolID string) bool {
	for _, t := range r.Tools {
		if t == toolID {
			return true
		}
	}
	return false
}

func ParseResultJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
