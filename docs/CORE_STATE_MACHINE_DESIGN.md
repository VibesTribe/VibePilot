# VibePilot Core State Machine Design

**Version:** 1.0
**Date:** 2026-03-03
**Purpose:** Define the proper state machine for VibeFlow 2.0

---

## Design Principles

1. **Supabase is source of truth** - all state lives here
2. **Event sourcing** - Every change is an event and persisted, replayable
3. **Checkpointing** - Work is saved incrementally, not lost on crash
4. **Recovery** - Crash → read state → continue
5. **Simplicity** - Minimal code, easy to understand, 6. **No infinite loops** - Every path has an exit condition

7. **No hardcoding** - Everything configurable

---

## Core State Document
type SystemState struct {
    Version   string    `json:"1.0"`
    UpdatedAt time.Time `json:"updated_at"`
    
    // Metrics (derived from queries)
    Metrics   Metrics   `json:"metrics"`
    
    // Agents (who can work)
    Agents   []AgentState `json:"agents"`
    
    // Plans (PRD → Tasks)
    Plans   []PlanState   `json:"plans"`
    
    // Tasks (work items)
    Tasks   []TaskState `json:"tasks"`
    
    // Slices (grouped progress)
    Slices  []SliceState `json:"slices"`
    
    // Failures (what went wrong)
    Failures []FailureState `json:"failures"`
    
    // Learning (model scores, patterns)
    Learning LearningState `json:"learning"`
}

type Metrics struct {
    TotalTokensUsed    int     `json:"total_tokens_used"`
    TotalTasksCompleted int     `json:"total_tasks_completed"`
    TotalTasksActive   int     `json:"total_tasks_active"`
    TotalCostSaved    float64 `json:"total_cost_saved"`
    TotalCostActual   float64 `json:"total_cost_actual"`
    AverageTaskDuration float64 `json:"average_task_duration"`
    ErrorRate          float64 `json:"error_rate"`
}

type AgentState struct {
    ID                  string    `json:"id"`
    Name               string    `json:"name"`
    Status             string    `json:"status"` // idle, working, cooldown, error, credit_needed
    CurrentTask        *string  `json:"current_task,omitempty"`
    CooldownReason     string  `json:"cooldown_reason,omitempty"`
    CooldownExpiresAt  *time.Time `json:"cooldown_expires_at,omitempty"`
    CreditStatus       string  `json:"credit_status"` // available, low, depleted
    SuccessRate        float64 `json:"success_rate"`
    TasksCompleted    int     `json:"tasks_completed"`
    TasksFailed        int     `json:"tasks_failed"`
    LastHeartbeat      time.Time `json:"last_heartbeat"`
    ModelID            string  `json:"model_id"`
    Vendor             string  `json:"vendor"`
    Tier               string  `json:"tier"` // Q (fast/cheap), M (balanced), W (premium)
}

type PlanState struct {
    ID                 string    `json:"id"`
    ProjectID          *string  `json:"project_id,omitempty"`
    PRDPath            string  `json:"prd_path"`
    PlanPath           string  `json:"plan_path,omitempty"`
    Status             string  `json:"status"` // draft, review, approved, revision_needed, council_review, error, complete
    RevisionRound      int     `json:"revision_round"`
    RevisionFeedback  []string `json:"revision_feedback,omitempty"`
    Tasks             []TaskRef `json:"tasks,omitempty"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
    Checkpoint          *Checkpoint `json:"checkpoint,omitempty"`
}

type TaskState struct {
    ID                 string    `json:"id"`
    PlanID             string    `json:"plan_id"`
    TaskNumber         string    `json:"task_number"` // T001, T002, etc.
    Title              string    `json:"title"`
    Status             string  `json:"status"` // pending, available, in_progress, review, testing, approval, merged, error, escalated
    Confidence          float64 `json:"confidence"`
    Dependencies        []string `json:"dependencies"`
    Category            string    `json:"category"` // coding, research, testing, documentation
    AssignedTo         string  `json:"assigned_to,omitempty"`
    PromptPacket        string  `json:"prompt_packet"`
    ExpectedOutput      string  `json:"expected_output"`
    Progress            int       `json:"progress"` // 0-100
    Checkpoint          *Checkpoint `json:"checkpoint,omitempty"`
    Attempts           int     `json:"attempts"`
    MaxAttempts        int     `json:"max_attempts"`
    LastError          string  `json:"last_error,omitempty"`
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
    StartedAt           *time.Time `json:"started_at,omitempty"`
    CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

type SliceState struct {
    ID              string  `json:"id"`
    Name            string  `json:"name"`
    TasksTotal     int     `json:"tasks_total"`
    TasksCompleted int     `json:"tasks_completed"`
    TokensUsed     int     `json:"tokens_used"`
    Accent         string  `json:"accent"` // hex color for dashboard
}

type FailureState struct {
    ID              string    `json:"id"`
    TaskID          string  `json:"task_id"`
    Type            string    `json:"type"` // timeout, parse_error, validation_failed, test_failed
    Message         string  `json:"message"`
    Timestamp       time.Time `json:"timestamp"`
    Recovered        bool    `json:"recovered"`
    RecoveryAction string  `json:"recovery_action,omitempty"`
}

type LearningState struct {
    ModelScores       map[string]ModelScore    `json:"model_scores"`       // model_id -> score
    PatternDetection []Pattern             `json:"pattern_detection"` // failure patterns detected
    Improvements     []ImprovementSuggestion `json:"improvements"`     // suggested improvements
    LastAnalysis      time.Time             `json:"last_analysis"`
}

type ModelScore struct {
    ModelID        string  `json:"model_id"`
    TaskType        string  `json:"task_type"`
    SuccessCount   int     `json:"success_count"`
    FailureCount   int     `json:"failure_count"`
    AverageDuration float64 `json:"average_duration"`
    LastUsed        time.Time `json:"last_used"`
    Score           float64 `json:"score"` // 0.0-1.0, computed
}

type Pattern struct {
    Type        string    `json:"type"` // timeout, parse_error, etc.
    Count       int     `json:"count"`
    LastSeen   time.Time `json:"last_seen"`
    Suggestion string    `json:"suggestion,omitempty"`
}

type ImprovementSuggestion struct {
    Type        string    `json:"type"` // prompt_update, config_change, workflow_change
    Description string    `json:"description"`
    Priority   int     `json:"priority"`
    Status     string  `json:"status"` // pending, approved, applied
}

type Checkpoint struct {
    Step        string    `json:"step"` // "planning", "execution" "review" "testing"
    Progress    int     `json:"progress"` // percentage
    Output     string    `json:"output,omitempty"`
    Files      []string `json:"files,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
}

type TaskRef struct {
    TaskNumber string `json:"task_number"`
    Title      string `json:"title"`
}

---

## Event Types
const (
    EventType string = iota {
    // Lifecycle Events
    EventPlanCreated
    EventPlanReviewStarted
    EventPlanReviewCompleted
    EventPlanApproved
    EventPlanRevisionNeeded
    EventPlanCouncilReview
    EventPlanError
    
    EventPlanComplete
    
    // Task Events
    EventTaskAvailable
    EventTaskClaimed
    EventTaskStarted
    EventTaskCheckpoint
    EventTaskCompleted
    EventTaskReviewStarted
    EventTaskReviewCompleted
    EventTaskTestStarted
    EventTaskTestCompleted
    EventTaskMerged
    EventTaskError
    EventTaskEscalated
    
    // Council Events
    EventCouncilStarted
    EventCouncilVote
    EventCouncilComplete
    
    EventCouncilConsensus
    
    // Research Events
    EventResearchSuggested
    EventResearchReviewStarted
    EventResearchApproved
    EventResearchApplied
    
    // Recovery Events
    EventRecoveryStarted
    EventRecoveryCompleted
    EventCrashDetected
    
    
    // Learning Events
    EventModelSuccess
    EventModelFailure
    EventPatternDetected
    EventImprovementSuggested
)

