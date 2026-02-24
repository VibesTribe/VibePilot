package janitor

import (
	"context"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/db"
)

type Janitor struct {
	db           *db.DB
	stuckTimeout time.Duration
	depreciation config.DeprecationConfig
}

func New(database *db.DB, stuckTimeout time.Duration, depreciation config.DeprecationConfig) *Janitor {
	return &Janitor{
		db:           database,
		stuckTimeout: stuckTimeout,
		depreciation: depreciation,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("Janitor started: stuck timeout %v, depreciation enabled=%v", j.stuckTimeout, j.depreciation.Enabled)

	for {
		select {
		case <-ctx.Done():
			log.Println("Janitor shutting down")
			return
		case <-ticker.C:
			j.resetStuckTasks(ctx)
			j.refreshLimits(ctx)
			if j.depreciation.Enabled {
				j.checkDepreciation(ctx)
			}
		}
	}
}

func (j *Janitor) resetStuckTasks(ctx context.Context) {
	tasks, err := j.db.GetStuckTasks(ctx, j.stuckTimeout)
	if err != nil {
		log.Printf("Janitor: failed to get stuck tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("Janitor: found %d stuck tasks", len(tasks))

	for _, task := range tasks {
		escalate := task.Attempts >= task.MaxAttempts-1

		if escalate {
			log.Printf("Janitor: escalating stuck task %s (attempts: %d/%d)",
				task.ID, task.Attempts, task.MaxAttempts)
		} else {
			log.Printf("Janitor: resetting stuck task %s (attempts: %d/%d)",
				task.ID, task.Attempts, task.MaxAttempts)
		}

		if err := j.db.ResetTask(ctx, task.ID, escalate); err != nil {
			log.Printf("Janitor: failed to reset task %s: %v", task.ID, err)
		}
	}
}

func (j *Janitor) refreshLimits(ctx context.Context) {
	if err := j.db.RefreshLimits(ctx); err != nil {
		log.Printf("Janitor: failed to refresh limits: %v", err)
	}
}

func (j *Janitor) checkDepreciation(ctx context.Context) {
	runners, err := j.db.GetRunnersToArchive(
		ctx,
		j.depreciation.ArchiveThreshold,
		j.depreciation.MinAttempts,
		j.depreciation.CooldownHours,
	)
	if err != nil {
		log.Printf("Janitor: failed to get runners to archive: %v", err)
		return
	}

	if len(runners) == 0 {
		return
	}

	for _, r := range runners {
		log.Printf("Janitor: archiving runner %s (model: %s) - depreciation_score=%.2f, attempts=%d",
			r.ID[:8], r.ModelID, r.DepreciationScore, r.TotalAttempts)
		if err := j.db.ArchiveRunner(ctx, r.ID, "depreciation_threshold"); err != nil {
			log.Printf("Janitor: failed to archive runner %s: %v", r.ID[:8], err)
		}
	}
}
