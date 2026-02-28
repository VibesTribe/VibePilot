package runtime

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolResult struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ToolExecutor interface {
	Execute(ctx context.Context, args map[string]any) (json.RawMessage, error)
}

type ToolRegistry struct {
	tools  map[string]ToolExecutor
	config *Config
}

func NewToolRegistry(cfg *Config) *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]ToolExecutor),
		config: cfg,
	}
}

func (r *ToolRegistry) Register(name string, executor ToolExecutor) {
	r.tools[name] = executor
}

func (r *ToolRegistry) HasTool(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *ToolRegistry) ListTools() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *ToolRegistry) Execute(ctx context.Context, toolName string, args map[string]any) ToolResult {
	toolCfg := r.config.GetTool(toolName)
	if toolCfg == nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %s not found in config", toolName),
		}
	}

	if err := validateToolArgs(toolCfg, args); err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid args: %v", err),
		}
	}

	executor, ok := r.tools[toolName]
	if !ok {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %s not registered", toolName),
		}
	}

	result, err := executor.Execute(ctx, args)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return ToolResult{
		Success: true,
		Result:  result,
	}
}

func validateToolArgs(tool *ToolConfig, args map[string]any) error {
	for paramName, paramCfg := range tool.Parameters {
		if paramCfg.Required {
			val, exists := args[paramName]
			if !exists || val == nil {
				return fmt.Errorf("required parameter %s missing", paramName)
			}
		}

		if val, exists := args[paramName]; exists && val != nil {
			if err := validateParamType(paramName, paramCfg.Type, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateParamType(name, expectedType string, value any) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s must be string", name)
		}
	case "integer", "int":
		switch value.(type) {
		case int, int32, int64, float64:
		default:
			return fmt.Errorf("parameter %s must be integer", name)
		}
	case "number", "float":
		switch value.(type) {
		case int, int32, int64, float32, float64:
		default:
			return fmt.Errorf("parameter %s must be number", name)
		}
	case "boolean", "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s must be boolean", name)
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("parameter %s must be array", name)
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("parameter %s must be object", name)
		}
	}
	return nil
}
