package janitor

import (
	"context"
	"log"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

type Janitor struct {
	db           *db.DB
	stuckTimeout time.Duration
}

func New(database *db.DB, stuckTimeout time.Duration) *Janitor {
	return &Janitor{
		db:           database,
		stuckTimeout: stuckTimeout,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Printf("Janitor started: stuck timeout %v", j.stuckTimeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("Janitor shutting down")
			return
		case <-ticker.C:
			j.resetStuckTasks(ctx)
			j.refreshLimits(ctx)
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
