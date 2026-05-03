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
)

type ConnectorRunner interface {
	Run(ctx context.Context, prompt string, timeout int) (output string, tokensIn, tokensOut int, err error)
}

// HealthChecker is an optional interface that connectors can implement
// to verify their endpoint and credentials are valid at startup or periodically.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

type Session struct {
	ID          string
	AgentID     string
	connector   ConnectorRunner
	connectorID string
	prompt      string
	timeout     time.Duration
}

type SessionOption func(*Session)

func WithTimeout(d time.Duration) SessionOption {
	return func(s *Session) { s.timeout = d }
}

func NewSession(id, agentID string, conn ConnectorRunner, connID, prompt string, opts ...SessionOption) *Session {
	s := &Session{
		ID:          id,
		AgentID:     agentID,
		connector:   conn,
		connectorID: connID,
		prompt:      prompt,
		timeout:     time.Duration(DefaultSessionTimeoutSecs) * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type SessionResult struct {
	Output      string        `json:"output"`
	TokensIn    int           `json:"tokens_in"`
	TokensOut   int           `json:"tokens_out"`
	Duration    time.Duration `json:"duration"`
	ConnectorID string        `json:"connector_id"`
	AgentID     string        `json:"agent_id"`
}

func (s *Session) Run(ctx context.Context, input map[string]any) (*SessionResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var prompt strings.Builder
	prompt.WriteString(s.prompt)
	prompt.WriteString("\n\n---\n\n")

	if input != nil {
		inputJSON, _ := json.MarshalIndent(input, "", "  ")
		prompt.WriteString("INPUT:\n")
		prompt.WriteString(string(inputJSON))
		prompt.WriteString("\n")
	}

	output, tokensIn, tokensOut, err := s.connector.Run(ctx, prompt.String(), int(s.timeout.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("session run failed: %w", err)
	}

	return &SessionResult{
		Output:      output,
		TokensIn:    tokensIn,
		TokensOut:   tokensOut,
		Duration:    time.Since(start),
		ConnectorID: s.connectorID,
		AgentID:     s.AgentID,
	}, nil
}

// Compact stores a session summary via the factory's compactor (if set).
// Call this after parsing the session result. Non-blocking, never errors.
func (f *SessionFactory) Compact(ctx context.Context, result *SessionResult, taskID string) {
	if f.compactor != nil {
		f.compactor.CompactSession(ctx, result, taskID)
	}
}

type SessionFactory struct {
	config         *Config
	connectors     map[string]ConnectorRunner
	contextBuilder *ContextBuilder
	compactor      SessionCompactor
}

// SessionCompactor compresses session results into summaries.
// Implemented by memory.Compactor. Nil = no compaction.
type SessionCompactor interface {
	CompactSession(ctx context.Context, result *SessionResult, taskID string)
}

func NewSessionFactory(cfg *Config) *SessionFactory {
	return &SessionFactory{
		config:     cfg,
		connectors: make(map[string]ConnectorRunner),
	}
}

func (f *SessionFactory) SetContextBuilder(cb *ContextBuilder) {
	f.contextBuilder = cb
}

func (f *SessionFactory) SetCompactor(c SessionCompactor) {
	f.compactor = c
}

func (f *SessionFactory) RegisterConnector(id string, runner ConnectorRunner) {
	f.connectors[id] = runner
}

func (f *SessionFactory) GetConnector(id string) (ConnectorRunner, bool) {
	runner, ok := f.connectors[id]
	return runner, ok
}

func (f *SessionFactory) GetConnectorConfig(id string) *ConnectorConfig {
	return f.config.GetConnector(id)
}

// buildAgentContext resolves context for an agent based on its context_policy from config.
// Single source of truth -- no hardcoded agent type switches.
func (f *SessionFactory) buildAgentContext(ctx context.Context, agentID string, taskType string) string {
	if f.contextBuilder == nil || taskType == "" {
		return ""
	}

	agent := f.config.GetAgent(agentID)
	if agent == nil {
		return ""
	}

	policy := agent.ContextPolicy
	if policy == "" {
		policy = "file_tree" // sensible default
	}

	switch policy {
	case "full_map":
		extra, err := f.contextBuilder.BuildPlannerContext(ctx, taskType)
		if err != nil {
			return ""
		}
		return extra

	case "kb_pack":
		// KB context pack: topic-oriented symbols, data flow, docs, decisions, rules
		// Topic is derived from task type (e.g. "dashboard feature" -> "dashboard")
		return f.contextBuilder.BuildKBContextPack(ctx, taskType)

	case "council":
		extra, err := f.contextBuilder.BuildCouncilContext(ctx, taskType)
		if err != nil {
			// Fallback to base file tree
			return f.contextBuilder.BuildBaseContext()
		}
		return extra

	case "file_tree":
		return f.contextBuilder.BuildBaseContext()

	case "targeted":
		// Targeted context (specific task files) is injected separately at execution time
		// via BuildTargetedContext in handlers_task.go. This returns nothing here.
		return ""

	case "none":
		return ""

	default:
		// Unknown policy -- default to file_tree
		return f.contextBuilder.BuildBaseContext()
	}
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

	connID := agent.DefaultConnector
	conn, ok := f.connectors[connID]
	if !ok {
		return nil, fmt.Errorf("connector %s not registered", connID)
	}

	sessionID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	cfgOpts := []SessionOption{
		WithTimeout(time.Duration(f.config.GetRuntimeConfig().AgentTimeoutSeconds) * time.Second),
	}
	cfgOpts = append(cfgOpts, opts...)

	return NewSession(sessionID, agentID, conn, connID, prompt, cfgOpts...), nil
}

func (f *SessionFactory) CreateWithContext(ctx context.Context, agentID string, taskType string, opts ...SessionOption) (*Session, error) {
	agent := f.config.GetAgent(agentID)
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	prompt, err := f.config.LoadPrompt(agent.Prompt)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	if extra := f.buildAgentContext(ctx, agentID, taskType); extra != "" {
		prompt += extra
	}

	connID := agent.DefaultConnector
	conn, ok := f.connectors[connID]
	if !ok {
		return nil, fmt.Errorf("connector %s not registered", connID)
	}

	sessionID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	cfgOpts := []SessionOption{
		WithTimeout(time.Duration(f.config.GetRuntimeConfig().AgentTimeoutSeconds) * time.Second),
	}
	cfgOpts = append(cfgOpts, opts...)

	return NewSession(sessionID, agentID, conn, connID, prompt, cfgOpts...), nil
}

func (f *SessionFactory) CreateWithConnector(ctx context.Context, agentID string, taskType string, connectorID string, opts ...SessionOption) (*Session, error) {
	agent := f.config.GetAgent(agentID)
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	prompt, err := f.config.LoadPrompt(agent.Prompt)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	if extra := f.buildAgentContext(ctx, agentID, taskType); extra != "" {
		prompt += extra
	}

	conn, ok := f.connectors[connectorID]
	if !ok {
		return nil, fmt.Errorf("connector %s not registered", connectorID)
	}

	sessionID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	cfgOpts := []SessionOption{
		WithTimeout(time.Duration(f.config.GetRuntimeConfig().AgentTimeoutSeconds) * time.Second),
	}
	cfgOpts = append(cfgOpts, opts...)

	return NewSession(sessionID, agentID, conn, connectorID, prompt, cfgOpts...), nil
}
