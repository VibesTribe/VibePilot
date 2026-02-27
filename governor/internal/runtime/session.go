package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultSessionTimeoutSecs = 300
	DefaultSessionMaxTurns    = 10
)

type LLMClient interface {
	Call(ctx context.Context, prompt string, tools []string) (string, error)
}

type DestinationRunner interface {
	Run(ctx context.Context, prompt string, timeout int) (output string, tokensIn, tokensOut int, err error)
}

type Session struct {
	ID           string
	AgentID      string
	destination  DestinationRunner
	prompt       string
	tools        []string
	toolRegistry *ToolRegistry
	timeout      time.Duration
	maxTurns     int
}

type SessionOption func(*Session)

func WithTimeout(d time.Duration) SessionOption {
	return func(s *Session) { s.timeout = d }
}

func WithMaxTurns(n int) SessionOption {
	return func(s *Session) { s.maxTurns = n }
}

func NewSession(id, agentID string, dest DestinationRunner, prompt string, tools []string, registry *ToolRegistry, opts ...SessionOption) *Session {
	s := &Session{
		ID:           id,
		AgentID:      agentID,
		destination:  dest,
		prompt:       prompt,
		tools:        tools,
		toolRegistry: registry,
		timeout:      time.Duration(DefaultSessionTimeoutSecs) * time.Second,
		maxTurns:     DefaultSessionMaxTurns,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type SessionResult struct {
	Output     string         `json:"output"`
	ToolCalls  int            `json:"tool_calls"`
	TokensIn   int            `json:"tokens_in"`
	TokensOut  int            `json:"tokens_out"`
	Duration   time.Duration  `json:"duration"`
	Turns      int            `json:"turns"`
	ToolResult map[string]any `json:"tool_results,omitempty"`
}

func (s *Session) Run(ctx context.Context, input map[string]any) (*SessionResult, error) {
	start := time.Now()
	result := &SessionResult{}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var prompt strings.Builder
	prompt.WriteString(s.prompt)
	prompt.WriteString("\n\n---\n\n")

	if input != nil {
		inputJSON, _ := json.MarshalIndent(input, "", "  ")
		prompt.WriteString("INPUT:\n")
		prompt.WriteString(string(inputJSON))
		prompt.WriteString("\n\n")
	}

	if len(s.tools) > 0 {
		prompt.WriteString("AVAILABLE TOOLS:\n")
		for _, t := range s.tools {
			prompt.WriteString(fmt.Sprintf("- %s\n", t))
		}
		prompt.WriteString("\nTo use a tool, output: TOOL: tool_name {\"arg\": \"value\"}\n\n")
	}

	prompt.WriteString("Respond with your output. If you need to use a tool, use the TOOL: format.\n")

	currentPrompt := prompt.String()
	turn := 0

	for turn < s.maxTurns {
		turn++
		result.Turns = turn

		output, tokensIn, tokensOut, err := s.destination.Run(ctx, currentPrompt, int(s.timeout.Seconds()))
		if err != nil {
			return nil, fmt.Errorf("llm call failed: %w", err)
		}

		result.TokensIn += tokensIn
		result.TokensOut += tokensOut

		toolCalls := ParseToolCalls(output)
		if len(toolCalls) == 0 {
			result.Output = output
			result.Duration = time.Since(start)
			return result, nil
		}

		result.ToolCalls += len(toolCalls)

		toolResults := make(map[string]ToolResult)
		for _, call := range toolCalls {
			toolResults[call.Name] = s.toolRegistry.Execute(ctx, s.AgentID, call.Name, call.Args)
		}

		var nextPrompt strings.Builder
		nextPrompt.WriteString(currentPrompt)
		nextPrompt.WriteString("\n\n---\n\nAGENT OUTPUT:\n")
		nextPrompt.WriteString(output)
		nextPrompt.WriteString("\n\n")
		nextPrompt.WriteString(FormatToolResults(toolResults))
		nextPrompt.WriteString("\n\nContinue with your next action or provide your final output.\n")

		currentPrompt = nextPrompt.String()

		allDone := true
		for _, tr := range toolResults {
			if !tr.Success {
				allDone = false
				break
			}
		}

		if allDone && len(toolCalls) == 1 {
			var parsedResult map[string]any
			if len(toolResults) > 0 {
				for _, tr := range toolResults {
					if tr.Success && len(tr.Result) > 0 {
						json.Unmarshal(tr.Result, &parsedResult)
						break
					}
				}
			}
			result.Output = output
			result.ToolResult = parsedResult
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	return nil, fmt.Errorf("max turns (%d) exceeded", s.maxTurns)
}

type SessionFactory struct {
	config       *Config
	registry     *ToolRegistry
	destinations map[string]DestinationRunner
}

func NewSessionFactory(cfg *Config, registry *ToolRegistry) *SessionFactory {
	return &SessionFactory{
		config:       cfg,
		registry:     registry,
		destinations: make(map[string]DestinationRunner),
	}
}

func (f *SessionFactory) RegisterDestination(id string, runner DestinationRunner) {
	f.destinations[id] = runner
}

func (f *SessionFactory) Create(agentID string, opts ...SessionOption) (*Session, error) {
	agent := f.config.GetAgent(agentID)
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	prompt, err := f.config.LoadPrompt(agent.Prompt)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	dest, ok := f.destinations[agent.DefaultDestination]
	if !ok {
		return nil, fmt.Errorf("destination %s not registered", agent.DefaultDestination)
	}

	sessionID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	cfgOpts := []SessionOption{
		WithTimeout(time.Duration(f.config.GetRuntimeConfig().AgentTimeoutSeconds) * time.Second),
		WithMaxTurns(f.config.GetRuntimeConfig().MaxToolTurns),
	}
	cfgOpts = append(cfgOpts, opts...)

	return NewSession(sessionID, agentID, dest, prompt, agent.Tools, f.registry, cfgOpts...), nil
}
