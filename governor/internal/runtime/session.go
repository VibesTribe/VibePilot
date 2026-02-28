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

type DestinationRunner interface {
	Run(ctx context.Context, prompt string, timeout int) (output string, tokensIn, tokensOut int, err error)
}

type Session struct {
	ID            string
	AgentID       string
	destination   DestinationRunner
	destinationID string
	prompt        string
	timeout       time.Duration
}

type SessionOption func(*Session)

func WithTimeout(d time.Duration) SessionOption {
	return func(s *Session) { s.timeout = d }
}

func NewSession(id, agentID string, dest DestinationRunner, destID, prompt string, opts ...SessionOption) *Session {
	s := &Session{
		ID:            id,
		AgentID:       agentID,
		destination:   dest,
		destinationID: destID,
		prompt:        prompt,
		timeout:       time.Duration(DefaultSessionTimeoutSecs) * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type SessionResult struct {
	Output        string        `json:"output"`
	TokensIn      int           `json:"tokens_in"`
	TokensOut     int           `json:"tokens_out"`
	Duration      time.Duration `json:"duration"`
	DestinationID string        `json:"destination_id"`
	AgentID       string        `json:"agent_id"`
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

	output, tokensIn, tokensOut, err := s.destination.Run(ctx, prompt.String(), int(s.timeout.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("session run failed: %w", err)
	}

	return &SessionResult{
		Output:        output,
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
		Duration:      time.Since(start),
		DestinationID: s.destinationID,
		AgentID:       s.AgentID,
	}, nil
}

type SessionFactory struct {
	config       *Config
	destinations map[string]DestinationRunner
}

func NewSessionFactory(cfg *Config) *SessionFactory {
	return &SessionFactory{
		config:       cfg,
		destinations: make(map[string]DestinationRunner),
	}
}

func (f *SessionFactory) RegisterDestination(id string, runner DestinationRunner) {
	f.destinations[id] = runner
}

func (f *SessionFactory) GetDestination(id string) (DestinationRunner, bool) {
	runner, ok := f.destinations[id]
	return runner, ok
}

func (f *SessionFactory) GetDestinationConfig(id string) *DestinationConfig {
	return f.config.GetDestination(id)
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

	destID := agent.DefaultDestination
	dest, ok := f.destinations[destID]
	if !ok {
		return nil, fmt.Errorf("destination %s not registered", destID)
	}

	sessionID := fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())

	cfgOpts := []SessionOption{
		WithTimeout(time.Duration(f.config.GetRuntimeConfig().AgentTimeoutSeconds) * time.Second),
	}
	cfgOpts = append(cfgOpts, opts...)

	return NewSession(sessionID, agentID, dest, destID, prompt, cfgOpts...), nil
}
