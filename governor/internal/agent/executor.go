package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/pool"
	"github.com/vibepilot/governor/internal/security"
)

type AgentExecutor struct {
	db           *db.DB
	pool         *pool.Pool
	leakDetector *security.LeakDetector
	timeoutSec   int
}

type ExecutorConfig struct {
	TimeoutSec int
}

func NewExecutor(database *db.DB, p *pool.Pool, cfg *ExecutorConfig) *AgentExecutor {
	timeout := 300
	if cfg != nil && cfg.TimeoutSec > 0 {
		timeout = cfg.TimeoutSec
	}

	return &AgentExecutor{
		db:           database,
		pool:         p,
		leakDetector: security.NewLeakDetector(),
		timeoutSec:   timeout,
	}
}

func (e *AgentExecutor) Execute(ctx context.Context, role *Role, prompt string, context map[string]interface{}) (*Result, error) {
	routing := "internal"
	if role.RequiresDestination != "" {
		routing = role.RequiresDestination
	}

	taskType := role.ID

	selection, err := e.pool.SelectBestWithTracking(ctx, routing, taskType, nil)
	if err != nil {
		return nil, fmt.Errorf("select runner: %w", err)
	}

	if selection.Runner == nil {
		return nil, fmt.Errorf("no runner available for role %s", role.ID)
	}

	runner := selection.Runner
	log.Printf("AgentExecutor: role=%s runner=%s model=%s", role.ID, runner.ID[:8], runner.ModelID)

	fullPrompt := e.buildPrompt(role, prompt, context)

	output, tokensIn, tokensOut, err := e.runTool(ctx, runner.ToolID, fullPrompt)
	if err != nil {
		e.pool.RecordResult(ctx, runner.ID, taskType, false, 0)
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	e.pool.RecordResult(ctx, runner.ID, taskType, true, tokensIn+tokensOut)

	return &Result{
		Success: true,
		Output:  output,
		Tokens: TokenUsage{
			Input:  tokensIn,
			Output: tokensOut,
			Total:  tokensIn + tokensOut,
		},
	}, nil
}

func (e *AgentExecutor) buildPrompt(role *Role, basePrompt string, context map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString("# ROLE\n\n")
	sb.WriteString(fmt.Sprintf("You are the %s. %s\n\n", role.Name, role.Description))

	if len(role.Skills) > 0 {
		sb.WriteString("## SKILLS\n\n")
		for _, skill := range role.Skills {
			sb.WriteString(fmt.Sprintf("- %s\n", skill))
		}
		sb.WriteString("\n")
	}

	if len(role.Tools) > 0 {
		sb.WriteString("## TOOLS AVAILABLE\n\n")
		for _, tool := range role.Tools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString(basePrompt)

	if len(context) > 0 {
		sb.WriteString("\n\n## CONTEXT\n\n")
		for k, v := range context {
			sb.WriteString(fmt.Sprintf("**%s:** %v\n", k, v))
		}
	}

	return sb.String()
}

func (e *AgentExecutor) runTool(ctx context.Context, toolID string, prompt string) (output string, tokensIn, tokensOut int, err error) {
	timeout := e.timeoutSec
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmdName := e.resolveToolCommand(toolID)
	cmd := exec.CommandContext(ctx, cmdName, "run", "--format", "json", prompt)
	raw, execErr := cmd.CombinedOutput()

	if execErr != nil {
		return "", 0, 0, fmt.Errorf("%s: %w\noutput: %s", cmdName, execErr, string(raw))
	}

	clean, warnings := e.leakDetector.Scan(string(raw))
	if len(warnings) > 0 {
		log.Printf("AgentExecutor: leak warnings: %+v", warnings)
	}

	var result struct {
		Content      string `json:"content"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}

	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		output = clean
		tokensIn = len(prompt) / 4
		tokensOut = len(output) / 4
	} else {
		output = result.Content
		tokensIn = result.InputTokens
		tokensOut = result.OutputTokens
	}

	return output, tokensIn, tokensOut, nil
}

func (e *AgentExecutor) resolveToolCommand(toolID string) string {
	switch toolID {
	case "opencode":
		return "opencode"
	case "kimi-cli":
		return "kimi"
	default:
		return "opencode"
	}
}

func (e *AgentExecutor) ExecuteWithJSON(ctx context.Context, role *Role, prompt string, context map[string]interface{}, target interface{}) (*Result, error) {
	result, err := e.Execute(ctx, role, prompt, context)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return result, nil
	}

	outputStr, ok := result.Output.(string)
	if !ok {
		return result, nil
	}

	jsonOutput := e.extractJSON(outputStr)
	if jsonOutput == "" {
		return result, nil
	}

	if err := json.Unmarshal([]byte(jsonOutput), target); err != nil {
		log.Printf("AgentExecutor: JSON parse warning: %v", err)
		return result, nil
	}

	result.Output = target
	return result, nil
}

func (e *AgentExecutor) extractJSON(output string) string {
	output = strings.TrimSpace(output)

	if strings.HasPrefix(output, "{") || strings.HasPrefix(output, "[") {
		return output
	}

	codeBlockStart := strings.Index(output, "```")
	if codeBlockStart == -1 {
		return ""
	}

	afterBlock := output[codeBlockStart+3:]

	langEnd := strings.Index(afterBlock, "\n")
	if langEnd != -1 {
		afterBlock = afterBlock[langEnd+1:]
	}

	blockEnd := strings.Index(afterBlock, "```")
	if blockEnd == -1 {
		return ""
	}

	return strings.TrimSpace(afterBlock[:blockEnd])
}
