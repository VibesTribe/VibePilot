package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Analyst struct {
	sm           *StateMachine
	db           DBInterface
	checkpointer *CheckpointManager
}

type DBInterface interface {
	Query(ctx context.Context, table string, filters map[string]any) ([]map[string]any, error)
	RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error)
}

type AnalysisResult struct {
	Timestamp      time.Time               `json:"timestamp"`
	ModelScores    map[string]ModelScore   `json:"model_scores"`
	Patterns       []Pattern               `json:"patterns"`
	Suggestions    []ImprovementSuggestion `json:"suggestions"`
	ErrorsDetected int                     `json:"errors_detected"`
}

func NewAnalyst(sm *StateMachine, db DBInterface, checkpointMgr *CheckpointManager) *Analyst {
	return &Analyst{
		sm:           sm,
		db:           db,
		checkpointer: checkpointMgr,
	}
}

func (a *Analyst) RunDailyAnalysis(ctx context.Context) (*AnalysisResult, error) {
	start := time.Now()

	result := &AnalysisResult{
		Timestamp:   start,
		ModelScores: make(map[string]ModelScore),
		Patterns:    []Pattern{},
		Suggestions: []ImprovementSuggestion{},
	}

	scores, err := a.analyzeModelPerformance(ctx)
	if err != nil {
		return nil, fmt.Errorf("analyze model performance: %w", err)
	}
	result.ModelScores = scores

	patterns, err := a.analyzeFailurePatterns(ctx)
	if err != nil {
		return nil, fmt.Errorf("analyze failure patterns: %w", err)
	}
	result.Patterns = patterns

	suggestions := a.generateImprovementSuggestions(patterns)
	result.Suggestions = suggestions

	a.sm.Emit(ctx, Event{
		Type:      EventPatternDetected,
		Timestamp: time.Now(),
		Payload:   result,
	})

	return result, nil
}

func (a *Analyst) analyzeModelPerformance(ctx context.Context) (map[string]ModelScore, error) {
	result, err := a.db.RPC(ctx, "get_model_performance", map[string]any{})
	if err != nil {
		return nil, err
	}

	var scores map[string]ModelScore
	if err := json.Unmarshal(result, &scores); err != nil {
		return nil, fmt.Errorf("parse model scores: %w", err)
	}

	return scores, nil
}

func (a *Analyst) analyzeFailurePatterns(ctx context.Context) ([]Pattern, error) {
	result, err := a.db.RPC(ctx, "get_failure_patterns", map[string]any{
		"days": 7,
	})
	if err != nil {
		return nil, err
	}

	var patterns []Pattern
	if err := json.Unmarshal(result, &patterns); err != nil {
		return nil, fmt.Errorf("parse patterns: %w", err)
	}

	return patterns, nil
}

func (a *Analyst) generateImprovementSuggestions(patterns []Pattern) []ImprovementSuggestion {
	suggestions := []ImprovementSuggestion{}

	for _, pattern := range patterns {
		if pattern.Count >= 3 {
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        "config_change",
				Description: fmt.Sprintf("High failure rate for %s (%d occurrences). Review config and routing", pattern.Type, pattern.Count),
				Priority:    1,
				Status:      "pending",
			})
		}
	}

	return suggestions
}
