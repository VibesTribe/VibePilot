package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type SystemState struct {
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`

	Metrics  Metrics        `json:"metrics"`
	Agents   []AgentState   `json:"agents"`
	Plans    []PlanState    `json:"plans"`
	Tasks    []TaskState    `json:"tasks"`
	Slices   []SliceState   `json:"slices"`
	Failures []FailureState `json:"failures"`
	Learning LearningState  `json:"learning"`
}

type Metrics struct {
	TotalTokensUsed     int     `json:"total_tokens_used"`
	TotalTasksCompleted int     `json:"total_tasks_completed"`
	TotalTasksActive    int     `json:"total_tasks_active"`
	TotalCostSaved      float64 `json:"total_cost_saved"`
	TotalCostActual     float64 `json:"total_cost_actual"`
	AverageTaskDuration float64 `json:"average_task_duration"`
	ErrorRate           float64 `json:"error_rate"`
}

type AgentState struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Status            string     `json:"status"`
	CurrentTask       *string    `json:"current_task,omitempty"`
	CooldownReason    string     `json:"cooldown_reason,omitempty"`
	CooldownExpiresAt *time.Time `json:"cooldown_expires_at,omitempty"`
	CreditStatus      string     `json:"credit_status"`
	SuccessRate       float64    `json:"success_rate"`
	TasksCompleted    int        `json:"tasks_completed"`
	TasksFailed       int        `json:"tasks_failed"`
	LastHeartbeat     time.Time  `json:"last_heartbeat"`
	ModelID           string     `json:"model_id"`
	Vendor            string     `json:"vendor"`
	Tier              string     `json:"tier"`
}

type PlanState struct {
	ID               string      `json:"id"`
	ProjectID        *string     `json:"project_id,omitempty"`
	PRDPath          string      `json:"prd_path"`
	PlanPath         string      `json:"plan_path,omitempty"`
	Status           string      `json:"status"`
	RevisionRound    int         `json:"revision_round"`
	RevisionFeedback []string    `json:"revision_feedback,omitempty"`
	Tasks            []TaskRef   `json:"tasks,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Checkpoint       *Checkpoint `json:"checkpoint,omitempty"`
}

type TaskState struct {
	ID             string      `json:"id"`
	PlanID         string      `json:"plan_id"`
	TaskNumber     string      `json:"task_number"`
	Title          string      `json:"title"`
	Status         string      `json:"status"`
	Confidence     float64     `json:"confidence"`
	Dependencies   []string    `json:"dependencies"`
	Category       string      `json:"category"`
	AssignedTo     string      `json:"assigned_to,omitempty"`
	PromptPacket   string      `json:"prompt_packet"`
	ExpectedOutput string      `json:"expected_output"`
	Progress       int         `json:"progress"`
	Checkpoint     *Checkpoint `json:"checkpoint,omitempty"`
	Attempts       int         `json:"attempts"`
	MaxAttempts    int         `json:"max_attempts"`
	LastError      string      `json:"last_error,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
}

type SliceState struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TasksTotal     int    `json:"tasks_total"`
	TasksCompleted int    `json:"tasks_completed"`
	TokensUsed     int    `json:"tokens_used"`
	Accent         string `json:"accent"`
}

type FailureState struct {
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	Type           string    `json:"type"`
	Message        string    `json:"message"`
	Timestamp      time.Time `json:"timestamp"`
	Recovered      bool      `json:"recovered"`
	RecoveryAction string    `json:"recovery_action,omitempty"`
}

type LearningState struct {
	ModelScores      map[string]ModelScore   `json:"model_scores"`
	PatternDetection []Pattern               `json:"pattern_detection"`
	Improvements     []ImprovementSuggestion `json:"improvements"`
	LastAnalysis     time.Time               `json:"last_analysis"`
}

type ModelScore struct {
	ModelID         string    `json:"model_id"`
	TaskType        string    `json:"task_type"`
	SuccessCount    int       `json:"success_count"`
	FailureCount    int       `json:"failure_count"`
	AverageDuration float64   `json:"average_duration"`
	LastUsed        time.Time `json:"last_used"`
	Score           float64   `json:"score"`
}

type Pattern struct {
	Type       string    `json:"type"`
	Count      int       `json:"count"`
	LastSeen   time.Time `json:"last_seen"`
	Suggestion string    `json:"suggestion,omitempty"`
}

type ImprovementSuggestion struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
}

type Checkpoint struct {
	Step      string    `json:"step"`
	Progress  int       `json:"progress"`
	Output    string    `json:"output,omitempty"`
	Files     []string  `json:"files,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type TaskRef struct {
	TaskNumber string `json:"task_number"`
	Title      string `json:"title"`
}

type EventType string

const (
	EventPlanCreated         EventType = "plan_created"
	EventPlanReviewStarted   EventType = "plan_review_started"
	EventPlanReviewCompleted EventType = "plan_review_completed"
	EventPlanApproved        EventType = "plan_approved"
	EventPlanRevisionNeeded  EventType = "plan_revision_needed"
	EventPlanCouncilReview   EventType = "plan_council_review"
	EventPlanError           EventType = "plan_error"
	EventPlanComplete        EventType = "plan_complete"

	EventTaskAvailable       EventType = "task_available"
	EventTaskClaimed         EventType = "task_claimed"
	EventTaskStarted         EventType = "task_started"
	EventTaskCheckpoint      EventType = "task_checkpoint"
	EventTaskCompleted       EventType = "task_completed"
	EventTaskReviewStarted   EventType = "task_review_started"
	EventTaskReviewCompleted EventType = "task_review_completed"
	EventTaskTestStarted     EventType = "task_test_started"
	EventTaskTestCompleted   EventType = "task_test_completed"
	EventTaskMerged          EventType = "task_merged"
	EventTaskError           EventType = "task_error"
	EventTaskEscalated       EventType = "task_escalated"
	EventTaskHumanReview     EventType = "task_human_review"

	EventCouncilStarted   EventType = "council_started"
	EventCouncilVote      EventType = "council_vote"
	EventCouncilComplete  EventType = "council_complete"
	EventCouncilConsensus EventType = "council_consensus"

	EventRecoveryStarted   EventType = "recovery_started"
	EventRecoveryCompleted EventType = "recovery_completed"
	EventCrashDetected     EventType = "crash_detected"

	EventModelSuccess         EventType = "model_success"
	EventModelFailure         EventType = "model_failure"
	EventPatternDetected      EventType = "pattern_detected"
	EventImprovementSuggested EventType = "improvement_suggested"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	PlanID    string      `json:"plan_id,omitempty"`
	TaskID    string      `json:"task_id,omitempty"`
	AgentID   string      `json:"agent_id,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
}

type StateMachine struct {
	mu       sync.RWMutex
	state    *SystemState
	handlers map[EventType][]EventHandler
}

type EventHandler func(ctx context.Context, event Event) error

func NewStateMachine() *StateMachine {
	return &StateMachine{
		state: &SystemState{
			Version:   "1.0",
			UpdatedAt: time.Now(),
			Metrics: Metrics{
				ErrorRate: 0.0,
			},
			Agents:   []AgentState{},
			Plans:    []PlanState{},
			Tasks:    []TaskState{},
			Slices:   []SliceState{},
			Failures: []FailureState{},
			Learning: LearningState{
				ModelScores:      make(map[string]ModelScore),
				PatternDetection: []Pattern{},
				Improvements:     []ImprovementSuggestion{},
			},
		},
		handlers: make(map[EventType][]EventHandler),
	}
}

func (sm *StateMachine) RegisterHandler(eventType EventType, handler EventHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.handlers[eventType] = append(sm.handlers[eventType], handler)
}

func (sm *StateMachine) Emit(ctx context.Context, event Event) error {
	sm.mu.RLock()
	handlers := sm.handlers[event.Type]
	sm.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return fmt.Errorf("handler for %s: %w", event.Type, err)
		}
	}
	return nil
}

func (sm *StateMachine) GetState() *SystemState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *StateMachine) UpdatePlan(planID string, update func(*PlanState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i := range sm.state.Plans {
		if sm.state.Plans[i].ID == planID {
			update(&sm.state.Plans[i])
			sm.state.Plans[i].UpdatedAt = time.Now()
			sm.state.UpdatedAt = time.Now()
			return
		}
	}
}

func (sm *StateMachine) UpdateTask(taskID string, update func(*TaskState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i := range sm.state.Tasks {
		if sm.state.Tasks[i].ID == taskID {
			update(&sm.state.Tasks[i])
			sm.state.Tasks[i].UpdatedAt = time.Now()
			sm.state.UpdatedAt = time.Now()
			return
		}
	}
}

func (sm *StateMachine) AddPlan(plan PlanState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Plans = append(sm.state.Plans, plan)
	sm.state.UpdatedAt = time.Now()
}

func (sm *StateMachine) AddTask(task TaskState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Tasks = append(sm.state.Tasks, task)
	sm.state.UpdatedAt = time.Now()
}

func (sm *StateMachine) ToJSON() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return json.MarshalIndent(sm.state, "", "  ")
}
