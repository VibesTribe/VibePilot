package researcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/pkg/types"
)

type Researcher struct {
	db       *db.DB
	analyzer Analyzer
}

type Analyzer interface {
	AnalyzeFailure(ctx context.Context, input *AnalysisInput) (*AnalysisResult, error)
}

type AnalysisInput struct {
	TaskID       string
	TaskTitle    string
	TaskType     string
	FailureNotes string
	Runs         []types.TaskRun
	Packet       *types.PromptPacket
}

type AnalysisResult struct {
	Category    string   `json:"category"`
	RootCause   string   `json:"root_cause"`
	Suggestions []string `json:"suggestions"`
	AutoRetry   bool     `json:"auto_retry"`
	NewModel    string   `json:"new_model,omitempty"`
}

const (
	CategoryModelIssue     = "model_issue"
	CategoryTaskDefinition = "task_definition"
	CategoryDependency     = "dependency_issue"
	CategoryInfrastructure = "infrastructure"
	CategoryUnknown        = "unknown"
)

type DefaultAnalyzer struct{}

func New(database *db.DB) *Researcher {
	return &Researcher{
		db:       database,
		analyzer: &DefaultAnalyzer{},
	}
}

func NewWithAnalyzer(database *db.DB, analyzer Analyzer) *Researcher {
	return &Researcher{
		db:       database,
		analyzer: analyzer,
	}
}

func (r *Researcher) AnalyzeEscalation(ctx context.Context, taskID string, failureNotes string) (*AnalysisResult, error) {
	task, err := r.db.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	runs, err := r.db.GetTaskRuns(ctx, taskID)
	if err != nil {
		log.Printf("Researcher: warning - could not get task runs: %v", err)
		runs = []types.TaskRun{}
	}

	packet, err := r.db.GetTaskPacket(ctx, taskID)
	if err != nil {
		log.Printf("Researcher: warning - could not get task packet: %v", err)
		packet = &types.PromptPacket{}
	}

	input := &AnalysisInput{
		TaskID:       taskID,
		TaskTitle:    task.Title,
		TaskType:     task.Type,
		FailureNotes: failureNotes,
		Runs:         runs,
		Packet:       packet,
	}

	return r.analyzer.AnalyzeFailure(ctx, input)
}

func (r *Researcher) RecordAnalysis(ctx context.Context, taskID string, result *AnalysisResult) error {
	now := time.Now().UTC().Format(time.RFC3339)

	body := map[string]interface{}{
		"task_id":     taskID,
		"category":    result.Category,
		"root_cause":  result.RootCause,
		"suggestions": result.Suggestions,
		"auto_retry":  result.AutoRetry,
		"new_model":   result.NewModel,
		"analyzed_at": now,
	}

	_, err := r.db.CreateResearchSuggestion(ctx, body)
	if err != nil {
		return fmt.Errorf("record analysis: %w", err)
	}

	return nil
}

func (a *DefaultAnalyzer) AnalyzeFailure(ctx context.Context, input *AnalysisInput) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Category:    CategoryUnknown,
		RootCause:   input.FailureNotes,
		Suggestions: []string{},
		AutoRetry:   false,
	}

	a.classifyByFailureNotes(input, result)
	a.classifyByRuns(input, result)
	a.classifyByPacket(input, result)

	if len(result.Suggestions) == 0 {
		result.Suggestions = []string{"Manual review required"}
	}

	return result, nil
}

func (a *DefaultAnalyzer) classifyByFailureNotes(input *AnalysisInput, result *AnalysisResult) {
	notes := strings.ToLower(input.FailureNotes)

	if strings.Contains(notes, "timeout") || strings.Contains(notes, "timed out") {
		result.Category = CategoryModelIssue
		result.RootCause = "Task execution timeout - model may be struggling with complexity"
		result.Suggestions = append(result.Suggestions, "Consider splitting task into smaller chunks")
		result.Suggestions = append(result.Suggestions, "Try a model with larger context window")
		return
	}

	if strings.Contains(notes, "rate limit") || strings.Contains(notes, "429") {
		result.Category = CategoryInfrastructure
		result.RootCause = "Rate limit hit on platform"
		result.Suggestions = append(result.Suggestions, "Wait for rate limit window to reset")
		result.AutoRetry = true
		return
	}

	if strings.Contains(notes, "commit failed") || strings.Contains(notes, "git") {
		result.Category = CategoryInfrastructure
		result.RootCause = "Git operation failed"
		result.Suggestions = append(result.Suggestions, "Check repository state and permissions")
		return
	}

	if strings.Contains(notes, "missing deliverable") || strings.Contains(notes, "expected file") {
		result.Category = CategoryTaskDefinition
		result.RootCause = "Task output does not match expected deliverables"
		result.Suggestions = append(result.Suggestions, "Review task prompt for clarity")
		result.Suggestions = append(result.Suggestions, "Verify deliverables list is achievable")
		return
	}

	if strings.Contains(notes, "secret") || strings.Contains(notes, "leak") {
		result.Category = CategoryTaskDefinition
		result.RootCause = "Security issue detected in output"
		result.Suggestions = append(result.Suggestions, "Review output for sensitive data")
		result.Suggestions = append(result.Suggestions, "Sanitize task output before retry")
		return
	}

	if strings.Contains(notes, "test") && strings.Contains(notes, "fail") {
		result.Category = CategoryModelIssue
		result.RootCause = "Generated code failed tests"
		result.Suggestions = append(result.Suggestions, "Review test failures for patterns")
		result.Suggestions = append(result.Suggestions, "Try a model better suited for this task type")
	}
}

func (a *DefaultAnalyzer) classifyByRuns(input *AnalysisInput, result *AnalysisResult) {
	if len(input.Runs) == 0 {
		return
	}

	modelFailures := make(map[string]int)
	modelSuccesses := make(map[string]int)

	for _, run := range input.Runs {
		if run.Status == "success" {
			modelSuccesses[run.ModelID]++
		} else {
			modelFailures[run.ModelID]++
		}
	}

	if len(modelFailures) >= 3 {
		if result.Category == CategoryUnknown {
			result.Category = CategoryModelIssue
		}
		result.RootCause = "Multiple models failed on this task"
		result.Suggestions = append(result.Suggestions, "Task may be fundamentally difficult or poorly defined")
	}

	var failedModels []string
	for modelID := range modelFailures {
		failedModels = append(failedModels, modelID)
	}

	if len(failedModels) > 0 && result.Category == CategoryModelIssue {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("Models that failed: %s", strings.Join(failedModels, ", ")))
	}
}

func (a *DefaultAnalyzer) classifyByPacket(input *AnalysisInput, result *AnalysisResult) {
	if input.Packet == nil {
		return
	}

	if input.Packet.Prompt == "" {
		if result.Category == CategoryUnknown {
			result.Category = CategoryTaskDefinition
		}
		result.RootCause = "Task has no prompt"
		result.Suggestions = append(result.Suggestions, "Add clear prompt to task packet")
	}

	if len(input.Packet.Deliverables) == 0 {
		if result.Category == CategoryUnknown {
			result.Category = CategoryTaskDefinition
		}
		result.Suggestions = append(result.Suggestions, "Add expected deliverables to task packet")
	}

	if input.Packet.Context != "" && len(input.Packet.Context) > 50000 {
		if result.Category == CategoryUnknown {
			result.Category = CategoryModelIssue
		}
		result.Suggestions = append(result.Suggestions, "Large context may be overwhelming model - consider summarization")
	}
}

func (r *Researcher) SuggestAlternativeModel(ctx context.Context, taskType string, failedModelID string) (string, error) {
	runners, err := r.db.GetAvailableModels(ctx, 5)
	if err != nil {
		return "", fmt.Errorf("get available models: %w", err)
	}

	for _, runner := range runners {
		if runner.ID != failedModelID {
			return runner.ID, nil
		}
	}

	return "", fmt.Errorf("no alternative model available")
}

func (r *Researcher) GetEscalationAnalysis(ctx context.Context, taskID string) (map[string]interface{}, error) {
	data, err := r.db.GetResearchSuggestion(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get research suggestion: %w", err)
	}
	return data, nil
}

func (r *Researcher) FormatAnalysisForHuman(result *AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Category: %s\n", result.Category))
	sb.WriteString(fmt.Sprintf("Root Cause: %s\n", result.RootCause))
	sb.WriteString("Suggestions:\n")
	for i, s := range result.Suggestions {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
	}
	if result.AutoRetry {
		sb.WriteString("Auto-retry: Yes\n")
	}
	if result.NewModel != "" {
		sb.WriteString(fmt.Sprintf("Suggested Model: %s\n", result.NewModel))
	}

	return sb.String()
}

func (r *Researcher) MarshalAnalysis(result *AnalysisResult) ([]byte, error) {
	return json.Marshal(result)
}

func UnmarshalAnalysis(data []byte) (*AnalysisResult, error) {
	var result AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal analysis: %w", err)
	}
	return &result, nil
}
