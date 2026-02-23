package sentry

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/throttle"
	"github.com/vibepilot/governor/pkg/types"
)

type Sentry struct {
	db            *db.DB
	pollInterval  time.Duration
	maxInFlight   int
	dispatchCh    chan types.Task
	inFlight      map[string]struct{}
	taskSlices    map[string]string
	mu            sync.Mutex
	moduleLimiter *throttle.ModuleLimiter
}

func New(database *db.DB, pollInterval time.Duration, maxInFlight int, dispatchCh chan types.Task, moduleLimiter *throttle.ModuleLimiter) *Sentry {
	return &Sentry{
		db:            database,
		pollInterval:  pollInterval,
		maxInFlight:   maxInFlight,
		dispatchCh:    dispatchCh,
		inFlight:      make(map[string]struct{}),
		taskSlices:    make(map[string]string),
		moduleLimiter: moduleLimiter,
	}
}

func (s *Sentry) Run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	maxStr := ""
	if s.moduleLimiter != nil {
		maxStr = ", 8 per module"
	}
	log.Printf("Sentry started: polling every %v, max %d concurrent%s", s.pollInterval, s.maxInFlight, maxStr)

	for {
		select {
		case <-ctx.Done():
			log.Println("Sentry shutting down")
			return
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

func (s *Sentry) poll(ctx context.Context) {
	s.mu.Lock()
	inFlightCount := len(s.inFlight)
	s.mu.Unlock()

	if inFlightCount >= s.maxInFlight {
		log.Printf("Sentry: max in-flight reached (%d), skipping poll", inFlightCount)
		return
	}

	tasks, err := s.db.GetAvailableTasks(ctx, s.maxInFlight-inFlightCount)
	if err != nil {
		log.Printf("Sentry: failed to poll tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("Sentry: found %d available tasks", len(tasks))

	for _, task := range tasks {
		s.mu.Lock()
		if _, exists := s.inFlight[task.ID]; exists {
			s.mu.Unlock()
			continue
		}

		if len(s.inFlight) >= s.maxInFlight {
			s.mu.Unlock()
			break
		}

		if s.moduleLimiter != nil && task.SliceID != "" {
			if !s.moduleLimiter.Acquire(task.SliceID) {
				log.Printf("Sentry: slice %s at max concurrent, skipping task %s", task.SliceID, task.ID[:8])
				s.mu.Unlock()
				continue
			}
		}

		s.inFlight[task.ID] = struct{}{}
		s.taskSlices[task.ID] = task.SliceID
		s.mu.Unlock()

		log.Printf("Sentry: dispatching task %s (%s, slice=%s)", task.ID[:8], task.Title, task.SliceID)

		select {
		case s.dispatchCh <- task:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sentry) Complete(taskID string, sliceID string) {
	s.mu.Lock()
	delete(s.inFlight, taskID)
	delete(s.taskSlices, taskID)
	s.mu.Unlock()

	if s.moduleLimiter != nil && sliceID != "" {
		s.moduleLimiter.Release(sliceID)
	}
}

func (s *Sentry) InFlightCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inFlight)
}

func (s *Sentry) ModuleCounts() map[string]int {
	if s.moduleLimiter == nil {
		return nil
	}
	return s.moduleLimiter.Counts()
}
