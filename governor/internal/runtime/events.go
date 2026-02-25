package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type EventType string

const (
	EventTaskAvailable  EventType = "task_available"
	EventTaskCompleted  EventType = "task_completed"
	EventTaskReview     EventType = "task_review"
	EventPlanCreated    EventType = "plan_created"
	EventCouncilDone    EventType = "council_done"
	EventResearchReady  EventType = "research_ready"
	EventMaintenanceCmd EventType = "maintenance_command"
)

type Event struct {
	Type      EventType       `json:"type"`
	ID        string          `json:"id"`
	Table     string          `json:"table"`
	Record    json.RawMessage `json:"record"`
	OldRecord json.RawMessage `json:"old_record,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type EventHandler func(Event)

type EventWatcher interface {
	Subscribe(ctx context.Context, handler EventHandler) error
	Close() error
}

type PollingWatcher struct {
	db       Querier
	interval time.Duration
	handlers []EventHandler
	mu       sync.RWMutex
	stop     chan struct{}
	stopped  bool
}

type Querier interface {
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
}

func NewPollingWatcher(db Querier, interval time.Duration) *PollingWatcher {
	if interval == 0 {
		interval = time.Second
	}
	return &PollingWatcher{
		db:       db,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (w *PollingWatcher) Subscribe(ctx context.Context, handler EventHandler) error {
	w.mu.Lock()
	w.handlers = append(w.handlers, handler)
	w.mu.Unlock()

	if !w.stopped {
		go w.poll(ctx)
	}

	return nil
}

func (w *PollingWatcher) poll(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	lastSeen := make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.checkForEvents(ctx, lastSeen)
		}
	}
}

func (w *PollingWatcher) checkForEvents(ctx context.Context, lastSeen map[string]time.Time) {
	w.detectTaskEvents(ctx, lastSeen)
	w.detectPlanEvents(ctx, lastSeen)
	w.detectMaintenanceEvents(ctx, lastSeen)
}

func (w *PollingWatcher) detectTaskEvents(ctx context.Context, lastSeen map[string]time.Time) {
	tasks, err := w.db.Query(ctx, "tasks", map[string]any{
		"or": "(status.eq.available,status.eq.review,status.eq.testing)",
	})
	if err != nil {
		return
	}

	var taskList []map[string]any
	if err := json.Unmarshal(tasks, &taskList); err != nil {
		return
	}

	for _, task := range taskList {
		id, _ := task["id"].(string)
		status, _ := task["status"].(string)
		updatedAt, _ := task["updated_at"].(string)

		key := fmt.Sprintf("task:%s", id)
		ts, _ := time.Parse(time.RFC3339, updatedAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			var eventType EventType
			switch status {
			case "available":
				eventType = EventTaskAvailable
			case "review":
				eventType = EventTaskReview
			case "testing", "approval":
				eventType = EventTaskCompleted
			}

			if eventType != "" {
				record, _ := json.Marshal(task)
				w.emit(Event{
					Type:      eventType,
					ID:        id,
					Table:     "tasks",
					Record:    record,
					Timestamp: time.Now(),
				})
			}
		}
	}
}

func (w *PollingWatcher) detectPlanEvents(ctx context.Context, lastSeen map[string]time.Time) {
	plans, err := w.db.Query(ctx, "plans", map[string]any{
		"or": "(status.eq.approved,status.eq.council_review)",
	})
	if err != nil {
		return
	}

	var planList []map[string]any
	if err := json.Unmarshal(plans, &planList); err != nil {
		return
	}

	for _, plan := range planList {
		id, _ := plan["id"].(string)
		status, _ := plan["status"].(string)
		updatedAt, _ := plan["updated_at"].(string)

		key := fmt.Sprintf("plan:%s", id)
		ts, _ := time.Parse(time.RFC3339, updatedAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			var eventType EventType
			switch status {
			case "council_review":
				eventType = EventPlanCreated
			case "approved":
				eventType = EventCouncilDone
			}

			if eventType != "" {
				record, _ := json.Marshal(plan)
				w.emit(Event{
					Type:      eventType,
					ID:        id,
					Table:     "plans",
					Record:    record,
					Timestamp: time.Now(),
				})
			}
		}
	}
}

func (w *PollingWatcher) detectMaintenanceEvents(ctx context.Context, lastSeen map[string]time.Time) {
	cmds, err := w.db.Query(ctx, "maintenance_commands", map[string]any{
		"status": "pending",
	})
	if err != nil {
		return
	}

	var cmdList []map[string]any
	if err := json.Unmarshal(cmds, &cmdList); err != nil {
		return
	}

	for _, cmd := range cmdList {
		id, _ := cmd["id"].(string)
		createdAt, _ := cmd["created_at"].(string)

		key := fmt.Sprintf("maintenance:%s", id)
		ts, _ := time.Parse(time.RFC3339, createdAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			record, _ := json.Marshal(cmd)
			w.emit(Event{
				Type:      EventMaintenanceCmd,
				ID:        id,
				Table:     "maintenance_commands",
				Record:    record,
				Timestamp: time.Now(),
			})
		}
	}
}

func (w *PollingWatcher) emit(event Event) {
	w.mu.RLock()
	handlers := make([]EventHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

func (w *PollingWatcher) Close() error {
	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()
	close(w.stop)
	return nil
}

type EventRouter struct {
	watcher   EventWatcher
	routes    map[EventType][]EventHandler
	mu        sync.RWMutex
	agentPool *AgentPool
}

func NewEventRouter(watcher EventWatcher) *EventRouter {
	return &EventRouter{
		watcher: watcher,
		routes:  make(map[EventType][]EventHandler),
	}
}

func (r *EventRouter) On(eventType EventType, handler EventHandler) {
	r.mu.Lock()
	r.routes[eventType] = append(r.routes[eventType], handler)
	r.mu.Unlock()
}

func (r *EventRouter) Start(ctx context.Context) error {
	return r.watcher.Subscribe(ctx, func(event Event) {
		r.mu.RLock()
		handlers := r.routes[event.Type]
		r.mu.RUnlock()

		for _, h := range handlers {
			go h(event)
		}
	})
}

func (r *EventRouter) SetAgentPool(pool *AgentPool) {
	r.mu.Lock()
	r.agentPool = pool
	r.mu.Unlock()
}
