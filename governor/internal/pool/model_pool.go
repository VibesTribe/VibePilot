package pool

import (
	"context"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

type Pool struct {
	db *db.DB
}

func New(database *db.DB) *Pool {
	return &Pool{db: database}
}

type SelectionResult struct {
	Runner       *db.Runner
	HeuristicID  string
	SolutionID   string
	UsedFallback bool
}

func (p *Pool) SelectBest(ctx context.Context, routing string, taskType string) (*db.Runner, error) {
	result, err := p.SelectBestWithTracking(ctx, routing, taskType, nil)
	if err != nil {
		return nil, err
	}
	return result.Runner, nil
}

func (p *Pool) SelectBestWithTracking(ctx context.Context, routing string, taskType string, condition map[string]interface{}) (*SelectionResult, error) {
	result := &SelectionResult{}

	heuristic, err := p.db.GetHeuristic(ctx, taskType, condition)
	if err != nil {
		log.Printf("Pool: error getting heuristic: %v", err)
	}
	if heuristic != nil && heuristic.PreferredModel != "" {
		runner, err := p.getRunnerByModel(ctx, heuristic.PreferredModel, routing)
		if err == nil && runner != nil && p.isRunnerAvailable(runner) {
			log.Printf("Pool: using heuristic preference %s for task_type=%s", heuristic.PreferredModel, taskType)
			result.Runner = runner
			result.HeuristicID = heuristic.ID
			return result, nil
		}
		log.Printf("Pool: heuristic model %s not available, falling back", heuristic.PreferredModel)
	}

	failures, err := p.db.GetRecentFailures(ctx, taskType, 60)
	if err != nil {
		log.Printf("Pool: error getting recent failures: %v", err)
	}
	excludedModels := make(map[string]bool)
	for _, f := range failures {
		if f.FailureCount >= 2 {
			excludedModels[f.ModelID] = true
			log.Printf("Pool: excluding model %s due to %d recent failures of type %s", f.ModelID, f.FailureCount, f.FailureType)
		}
	}

	runner, err := p.db.GetBestRunner(ctx, routing, taskType)
	if err != nil {
		return nil, err
	}

	if runner == nil {
		log.Printf("Pool: no runner for routing=%s type=%s", routing, taskType)
		return result, nil
	}

	if excludedModels[runner.ModelID] {
		log.Printf("Pool: best runner %s excluded, trying next best", runner.ModelID)
		result.UsedFallback = true
	}

	result.Runner = runner
	return result, nil
}

func (p *Pool) getRunnerByModel(ctx context.Context, modelID, routing string) (*db.Runner, error) {
	runners, err := p.db.GetAvailableModels(ctx, 10)
	if err != nil {
		return nil, err
	}
	for i := range runners {
		if runners[i].ModelID == modelID {
			return &runners[i], nil
		}
	}
	return nil, nil
}

func (p *Pool) isRunnerAvailable(runner *db.Runner) bool {
	if runner.CooldownExpires != nil && runner.CooldownExpires.After(time.Now()) {
		return false
	}
	if runner.RateLimitReset != nil && runner.RateLimitReset.After(time.Now()) {
		return false
	}
	if runner.DailyLimit > 0 && runner.DailyUsed >= runner.DailyLimit {
		return false
	}
	return true
}

func (p *Pool) RecordHeuristicSuccess(ctx context.Context, heuristicID string, success bool) error {
	if heuristicID == "" {
		return nil
	}
	return p.db.RecordHeuristicResult(ctx, heuristicID, success)
}

func (p *Pool) RecordSolutionSuccess(ctx context.Context, solutionID string, success bool) error {
	if solutionID == "" {
		return nil
	}
	return p.db.RecordSolutionResult(ctx, solutionID, success)
}

func (p *Pool) RecordResult(ctx context.Context, runnerID string, taskType string, success bool, tokens int) error {
	return p.db.RecordRunnerResult(ctx, runnerID, taskType, success, tokens)
}

func (p *Pool) SetCooldown(ctx context.Context, runnerID string, duration time.Duration) error {
	expiresAt := time.Now().Add(duration)
	return p.db.SetRunnerCooldown(ctx, runnerID, expiresAt)
}

func (p *Pool) SetRateLimited(ctx context.Context, runnerID string, resetAt time.Time) error {
	return p.db.SetRunnerRateLimited(ctx, runnerID, resetAt)
}

func (p *Pool) ShouldThrottle(runner *db.Runner) bool {
	if runner.DailyLimit <= 0 {
		return false
	}
	usage := float64(runner.DailyUsed) / float64(runner.DailyLimit)
	return usage >= 0.80
}

func (p *Pool) RefreshLimits(ctx context.Context) error {
	return p.db.RefreshLimits(ctx)
}
