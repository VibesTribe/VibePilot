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

func (p *Pool) SelectBest(ctx context.Context, routing string, taskType string) (*db.Runner, error) {
	runner, err := p.db.GetBestRunner(ctx, routing, taskType)
	if err != nil {
		return nil, err
	}
	if runner == nil {
		log.Printf("Pool: no runner for routing=%s type=%s", routing, taskType)
	}
	return runner, nil
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
