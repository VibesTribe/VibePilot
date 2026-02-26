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
	EventPlanReview     EventType = "plan_review"
	EventPlanCreated    EventType = "plan_created"
	EventCouncilDone    EventType = "council_done"
	EventResearchReady  EventType = "research_ready"
	EventMaintenanceCmd EventType = "maintenance_command"
	EventPRDReady       EventType = "prd_ready"
	EventTestResults    EventType = "test_results"
	EventHumanQuery     EventType = "human_query"
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

type Querier interface {
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
}

type PollingWatcher struct {
	db       Querier
	cfg      *Config
	interval time.Duration
	handlers []EventHandler
	mu       sync.RWMutex
	stop     chan struct{}
	stopped  bool
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

func (w *PollingWatcher) SetConfig(cfg *Config) {
	w.mu.Lock()
	w.cfg = cfg
	w.mu.Unlock()
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
	w.detectPlanReview(ctx, lastSeen)
	w.detectPlanEvents(ctx, lastSeen)
	w.detectPRDReady(ctx, lastSeen)
	w.detectMaintenanceEvents(ctx, lastSeen)
	w.detectTestResults(ctx, lastSeen)
}

func (w *PollingWatcher) getEventsConfig() *EventsConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.cfg != nil {
		return w.cfg.GetEventsConfig()
	}
	return &EventsConfig{
		TaskStatusesAvailable:    []string{"available"},
		TaskStatusesReview:       []string{"review"},
		TaskStatusesCompleted:    []string{"testing", "approval"},
		PlanStatusesDraft:        []string{"draft"},
		PlanStatusesReview:       []string{"review"},
		PlanStatusesCouncil:      []string{"council_review", "revision_needed"},
		PlanStatusesPendingHuman: []string{"pending_human"},
		PlanStatusesApproved:     []string{"approved"},
		MaintenanceStatus:        "pending",
		TestResultsStatus:        "pending_review",
	}
}

func (w *PollingWatcher) detectTaskEvents(ctx context.Context, lastSeen map[string]time.Time) {
	eventsCfg := w.getEventsConfig()

	allStatuses := make([]string, 0)
	allStatuses = append(allStatuses, eventsCfg.TaskStatusesAvailable...)
	allStatuses = append(allStatuses, eventsCfg.TaskStatusesReview...)
	allStatuses = append(allStatuses, eventsCfg.TaskStatusesCompleted...)

	orFilter := buildOrFilter(allStatuses, "status")

	tasks, err := w.db.Query(ctx, "tasks", map[string]any{
		"or": orFilter,
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
			if contains(eventsCfg.TaskStatusesAvailable, status) {
				eventType = EventTaskAvailable
			} else if contains(eventsCfg.TaskStatusesReview, status) {
				eventType = EventTaskReview
			} else if contains(eventsCfg.TaskStatusesCompleted, status) {
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

func (w *PollingWatcher) detectPlanReview(ctx context.Context, lastSeen map[string]time.Time) {
	eventsCfg := w.getEventsConfig()

	if len(eventsCfg.PlanStatusesReview) == 0 {
		return
	}

	orFilter := buildOrFilter(eventsCfg.PlanStatusesReview, "status")

	plans, err := w.db.Query(ctx, "plans", map[string]any{
		"or":    orFilter,
		"limit": 10,
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
		updatedAt, _ := plan["updated_at"].(string)

		key := fmt.Sprintf("plan_review:%s", id)
		ts, _ := time.Parse(time.RFC3339, updatedAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			record, _ := json.Marshal(plan)
			w.emit(Event{
				Type:      EventPlanReview,
				ID:        id,
				Table:     "plans",
				Record:    record,
				Timestamp: time.Now(),
			})
		}
	}
}

func (w *PollingWatcher) detectPlanEvents(ctx context.Context, lastSeen map[string]time.Time) {
	eventsCfg := w.getEventsConfig()

	allStatuses := make([]string, 0)
	allStatuses = append(allStatuses, eventsCfg.PlanStatusesApproved...)
	allStatuses = append(allStatuses, eventsCfg.PlanStatusesCouncil...)

	orFilter := buildOrFilter(allStatuses, "status")

	plans, err := w.db.Query(ctx, "plans", map[string]any{
		"or": orFilter,
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
			if contains(eventsCfg.PlanStatusesCouncil, status) {
				eventType = EventPlanCreated
			} else if contains(eventsCfg.PlanStatusesApproved, status) {
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
	eventsCfg := w.getEventsConfig()

	cmds, err := w.db.Query(ctx, "maintenance_commands", map[string]any{
		"status": eventsCfg.MaintenanceStatus,
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

func (w *PollingWatcher) detectPRDReady(ctx context.Context, lastSeen map[string]time.Time) {
	eventsCfg := w.getEventsConfig()

	if len(eventsCfg.PlanStatusesDraft) == 0 {
		return
	}

	orFilter := buildOrFilter(eventsCfg.PlanStatusesDraft, "status")

	plans, err := w.db.Query(ctx, "plans", map[string]any{
		"or":    orFilter,
		"limit": 10,
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
		updatedAt, _ := plan["updated_at"].(string)

		key := fmt.Sprintf("prd_ready:%s", id)
		ts, _ := time.Parse(time.RFC3339, updatedAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			record, _ := json.Marshal(plan)
			w.emit(Event{
				Type:      EventPRDReady,
				ID:        id,
				Table:     "plans",
				Record:    record,
				Timestamp: time.Now(),
			})
		}
	}
}

func (w *PollingWatcher) detectTestResults(ctx context.Context, lastSeen map[string]time.Time) {
	eventsCfg := w.getEventsConfig()

	if eventsCfg.TestResultsStatus == "" {
		return
	}

	results, err := w.db.Query(ctx, "test_results", map[string]any{
		"status": eventsCfg.TestResultsStatus,
		"limit":  10,
	})
	if err != nil {
		return
	}

	var resultList []map[string]any
	if err := json.Unmarshal(results, &resultList); err != nil {
		return
	}

	for _, result := range resultList {
		id, _ := result["id"].(string)
		createdAt, _ := result["created_at"].(string)

		key := fmt.Sprintf("test_result:%s", id)
		ts, _ := time.Parse(time.RFC3339, createdAt)

		if lastSeen[key].Before(ts) {
			lastSeen[key] = ts

			record, _ := json.Marshal(result)
			w.emit(Event{
				Type:      EventTestResults,
				ID:        id,
				Table:     "test_results",
				Record:    record,
				Timestamp: time.Now(),
			})
		}
	}
}

func buildOrFilter(values []string, field string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return field + ".eq." + values[0]
	}
	result := field + ".eq." + values[0]
	for i := 1; i < len(values); i++ {
		result += "," + field + ".eq." + values[i]
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
	watcher EventWatcher
	routes  map[EventType][]EventHandler
	mu      sync.RWMutex
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
