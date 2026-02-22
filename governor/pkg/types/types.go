package types

import (
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusLocked     TaskStatus = "locked"
	StatusAvailable  TaskStatus = "available"
	StatusInProgress TaskStatus = "in_progress"
	StatusReview     TaskStatus = "review"
	StatusTesting    TaskStatus = "testing"
	StatusApproval   TaskStatus = "approval"
	StatusMerged     TaskStatus = "merged"
	StatusEscalated  TaskStatus = "escalated"
)

type RoutingFlag string

const (
	RoutingInternal RoutingFlag = "internal"
	RoutingWeb      RoutingFlag = "web"
	RoutingMCP      RoutingFlag = "mcp"
)

type Task struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Type           string      `json:"type"`
	Priority       int         `json:"priority"`
	Status         TaskStatus  `json:"status"`
	RoutingFlag    RoutingFlag `json:"routing_flag"`
	AssignedTo     string      `json:"assigned_to"`
	Dependencies   []string    `json:"dependencies"`
	SliceID        string      `json:"slice_id"`
	Phase          string      `json:"phase"`
	TaskNumber     string      `json:"task_number"`
	BranchName     string      `json:"branch_name"`
	Attempts       int         `json:"attempts"`
	MaxAttempts    int         `json:"max_attempts"`
	PromptPacket   *PromptPacket `json:"prompt_packet"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	StartedAt      *time.Time  `json:"started_at"`
	CompletedAt    *time.Time  `json:"completed_at"`
}

type PromptPacket struct {
	TaskID        string                 `json:"task_id"`
	Prompt        string                 `json:"prompt"`
	Title         string                 `json:"title"`
	Objectives    []string               `json:"objectives"`
	Deliverables  []string               `json:"deliverables"`
	Context       string                 `json:"context"`
	OutputFormat  map[string]interface{} `json:"output_format"`
	Constraints   *Constraints           `json:"constraints"`
}

type Constraints struct {
	MaxTokens       int `json:"max_tokens"`
	TimeoutSeconds  int `json:"timeout_seconds"`
}

type TaskRun struct {
	ID                      string     `json:"id"`
	TaskID                  string     `json:"task_id"`
	Courier                 string     `json:"courier"`
	Platform                string     `json:"platform"`
	ModelID                 string     `json:"model_id"`
	Status                  string     `json:"status"`
	Result                  []byte     `json:"result"`
	Error                   string     `json:"error"`
	TokensIn                int        `json:"tokens_in"`
	TokensOut               int        `json:"tokens_out"`
	ChatURL                 string     `json:"chat_url"`
	PlatformTheoreticalCost float64    `json:"platform_theoretical_cost_usd"`
	TotalActualCost         float64    `json:"total_actual_cost_usd"`
	TotalSavings            float64    `json:"total_savings_usd"`
	StartedAt               time.Time  `json:"started_at"`
	CompletedAt             *time.Time `json:"completed_at"`
}

type Model struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Platform        string    `json:"platform"`
	Vendor          string    `json:"vendor"`
	ContextLimit    int       `json:"context_limit"`
	Status          string    `json:"status"`
	StatusReason    string    `json:"status_reason"`
	AccessType      string    `json:"access_type"`
	TokensUsed      int       `json:"tokens_used"`
	TasksCompleted  int       `json:"tasks_completed"`
	TasksFailed     int       `json:"tasks_failed"`
	SuccessRate     float64   `json:"success_rate"`
	CooldownExpires *time.Time `json:"cooldown_expires_at"`
}

type Platform struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	URL                   string    `json:"url"`
	Type                  string    `json:"type"`
	DailyLimit            int       `json:"daily_limit"`
	DailyUsed             int       `json:"daily_used"`
	Status                string    `json:"status"`
	SuccessRate           float64   `json:"success_rate"`
	ConsecutiveFailures   int       `json:"consecutive_failures"`
}

type DispatchResult struct {
	TaskID    string `json:"task_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	ChatURL   string `json:"chat_url,omitempty"`
	BranchName string `json:"branch_name,omitempty"`
}

type Provider interface {
	Chat(req *ChatRequest) (*ChatResponse, error)
	SupportsNativeTools() bool
}

type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ChatMessage          `json:"messages"`
	Tools    []ToolDefinition       `json:"tools,omitempty"`
	MaxTokens int                   `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ChatResponse struct {
	ID      string        `json:"id"`
	Content string        `json:"content"`
	Usage   *TokenUsage   `json:"usage"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}
