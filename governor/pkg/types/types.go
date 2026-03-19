package types

import (
	"time"
)

type TaskStatus string

const (
	StatusPending       TaskStatus = "pending"
	StatusLocked        TaskStatus = "locked"
	StatusAvailable     TaskStatus = "available"
	StatusInProgress    TaskStatus = "in_progress"
	StatusReview        TaskStatus = "review"
	StatusTesting       TaskStatus = "testing"
	StatusComplete      TaskStatus = "complete"
	StatusApproval      TaskStatus = "approval"
	StatusMergePending  TaskStatus = "merge_pending"
	StatusMerged        TaskStatus = "merged"
	StatusEscalated     TaskStatus = "escalated"
	StatusAwaitingHuman TaskStatus = "awaiting_human"
)

type RoutingFlag string

const (
	RoutingInternal RoutingFlag = "internal"
	RoutingWeb      RoutingFlag = "web"
	RoutingMCP      RoutingFlag = "mcp"
)

type Task struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Type         string        `json:"type"`
	Priority     int           `json:"priority"`
	Status       TaskStatus    `json:"status"`
	RoutingFlag  RoutingFlag   `json:"routing_flag"`
	AssignedTo   string        `json:"assigned_to"`
	Dependencies []string      `json:"dependencies"`
	SliceID      string        `json:"slice_id"`
	Phase        string        `json:"phase"`
	TaskNumber   string        `json:"task_number"`
	BranchName   string        `json:"branch_name"`
	ParentTaskID string        `json:"parent_task_id"`
	Attempts     int           `json:"attempts"`
	MaxAttempts  int           `json:"max_attempts"`
	PromptPacket *PromptPacket `json:"prompt_packet"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	StartedAt    *time.Time    `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at"`
}

type PromptPacket struct {
	TaskID       string                 `json:"task_id"`
	Prompt       string                 `json:"prompt"`
	Title        string                 `json:"title"`
	Objectives   []string               `json:"objectives"`
	Deliverables []string               `json:"deliverables"`
	Context      string                 `json:"context"`
	OutputFormat map[string]interface{} `json:"output_format"`
	Constraints  *Constraints           `json:"constraints"`
}

type Constraints struct {
	MaxTokens      int `json:"max_tokens"`
	TimeoutSeconds int `json:"timeout_seconds"`
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
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Platform        string     `json:"platform"`
	Vendor          string     `json:"vendor"`
	ContextLimit    int        `json:"context_limit"`
	Status          string     `json:"status"`
	StatusReason    string     `json:"status_reason"`
	AccessType      string     `json:"access_type"`
	TokensUsed      int        `json:"tokens_used"`
	TasksCompleted  int        `json:"tasks_completed"`
	TasksFailed     int        `json:"tasks_failed"`
	SuccessRate     float64    `json:"success_rate"`
	CooldownExpires *time.Time `json:"cooldown_expires_at"`
}

type Platform struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	URL                 string  `json:"url"`
	Type                string  `json:"type"`
	DailyLimit          int     `json:"daily_limit"`
	DailyUsed           int     `json:"daily_used"`
	Status              string  `json:"status"`
	SuccessRate         float64 `json:"success_rate"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
}

type DispatchResult struct {
	TaskID     string `json:"task_id"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	ChatURL    string `json:"chat_url,omitempty"`
	BranchName string `json:"branch_name,omitempty"`
}
