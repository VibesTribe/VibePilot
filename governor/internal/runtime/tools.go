package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ToolResult struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ToolExecutor interface {
	Execute(ctx context.Context, name string, args map[string]any) (json.RawMessage, error)
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

func (r *ToolRegistry) Execute(ctx context.Context, agentID, toolName string, args map[string]any) ToolResult {
	if !r.config.AgentHasTool(agentID, toolName) {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %s not available to agent %s", toolName, agentID),
		}
	}

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

	result, err := executor.Execute(ctx, toolName, args)
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

var toolCallPattern = regexp.MustCompile(`(?i)TOOL:\s*([a-z_][a-z0-9_]*)\s*(\{[\s\S]*?\})?`)

func ParseToolCalls(output string) []ToolCall {
	var calls []ToolCall

	matches := toolCallPattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		call := ToolCall{
			Name: strings.TrimSpace(match[1]),
		}

		if len(match) > 2 && match[2] != "" {
			var args map[string]any
			if err := json.Unmarshal([]byte(match[2]), &args); err == nil {
				call.Args = args
			}
		}

		if call.Args == nil {
			call.Args = make(map[string]any)
		}

		calls = append(calls, call)
	}

	return calls
}

type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func (c ToolCall) String() string {
	argsJSON, _ := json.Marshal(c.Args)
	return fmt.Sprintf("TOOL: %s %s", c.Name, string(argsJSON))
}

func FormatToolResults(results map[string]ToolResult) string {
	var sb strings.Builder
	sb.WriteString("TOOL RESULTS:\n")
	for name, result := range results {
		if result.Success {
			sb.WriteString(fmt.Sprintf("- %s: SUCCESS %s\n", name, string(result.Result)))
		} else {
			sb.WriteString(fmt.Sprintf("- %s: ERROR %s\n", name, result.Error))
		}
	}
	return sb.String()
}
