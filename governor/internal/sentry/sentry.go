package sentry

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/pkg/types"
)

type Sentry struct {
	db           *db.DB
	pollInterval time.Duration
	maxInFlight  int
	dispatchCh   chan types.Task
	inFlight     map[string]struct{}
	mu           sync.Mutex
}

func New(database *db.DB, pollInterval time.Duration, maxInFlight int, dispatchCh chan types.Task) *Sentry {
	return &Sentry{
		db:           database,
		pollInterval: pollInterval,
		maxInFlight:  maxInFlight,
		dispatchCh:   dispatchCh,
		inFlight:     make(map[string]struct{}),
	}
}

func (s *Sentry) Run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	log.Printf("Sentry started: polling every %v, max %d concurrent", s.pollInterval, s.maxInFlight)

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

	tasks, err := s.db.GetAvailableTasks(ctx)
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

		s.inFlight[task.ID] = struct{}{}
		s.mu.Unlock()

		log.Printf("Sentry: dispatching task %s (%s)", task.ID, task.Title)

		select {
		case s.dispatchCh <- task:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sentry) Complete(taskID string) {
	s.mu.Lock()
	delete(s.inFlight, taskID)
	s.mu.Unlock()
}

func (s *Sentry) InFlightCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inFlight)
}
