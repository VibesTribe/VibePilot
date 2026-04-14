package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vibepilot/governor/internal/runtime"
)

// MCPToolExecutor wraps an MCP client tool call as a runtime.ToolExecutor.
// This lets MCP tools be registered in the existing ToolRegistry so agents
// can call them through the same interface as built-in tools.
type MCPToolExecutor struct {
	registry *Registry
	toolName string
}

// NewMCPToolExecutor creates a ToolExecutor for an MCP tool.
func NewMCPToolExecutor(registry *Registry, toolName string) *MCPToolExecutor {
	return &MCPToolExecutor{
		registry: registry,
		toolName: toolName,
	}
}

// Execute implements runtime.ToolExecutor by calling the MCP tool.
func (e *MCPToolExecutor) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	result, err := e.registry.CallTool(ctx, e.toolName, args)
	if err != nil {
		return nil, fmt.Errorf("MCP tool %s: %w", e.toolName, err)
	}
	return result, nil
}

// RegisterToolsInRegistry discovers all MCP tools and registers them in the
// VibePilot tool registry so agents can call them alongside built-in tools.
func (r *Registry) RegisterToolsInRegistry(toolRegistry *runtime.ToolRegistry) {
	tools := r.ListTools()
	for _, binding := range tools {
		executor := NewMCPToolExecutor(r, binding.Tool.Name)
		toolRegistry.Register("mcp_"+binding.Tool.Name, executor)
	}
}
