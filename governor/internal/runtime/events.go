package runtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

type EventType string

const (
	EventTaskAvailable    EventType = "task_available"
	EventTaskCompleted    EventType = "task_completed"
	EventTaskReview       EventType = "task_review"
	EventTaskTesting      EventType = "task_testing"
	EventPlanReview       EventType = "plan_review"
	EventPlanCreated      EventType = "plan_created"
	EventCouncilDone      EventType = "council_done"
	EventCouncilReview    EventType = "council_review"
	EventResearchReady    EventType = "research_ready"
	EventResearchCouncil  EventType = "research_council"
	EventMaintenanceCmd   EventType = "maintenance_command"
	EventPRDReady         EventType = "prd_ready"
	EventTestResults      EventType = "test_results"
	EventHumanQuery       EventType = "human_query"
	EventRevisionNeeded   EventType = "revision_needed"
	EventCouncilComplete  EventType = "council_complete"
	EventPlanApproved     EventType = "plan_approved"
	EventPlanBlocked      EventType = "plan_blocked"
	EventPRDIncomplete    EventType = "prd_incomplete"
	EventPlanError        EventType = "plan_error"
	EventTaskApproval     EventType = "task_approved"
	EventTaskMerged       EventType = "task_merged"
	EventTaskMergePending EventType = "task_merge_pending"
	EventCourierResult    EventType = "courier_result" // task_runs UPDATE with completed/failed status
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
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}

type NopWatcher struct{}

func (n *NopWatcher) Subscribe(ctx context.Context, handler EventHandler) error { return nil }
func (n *NopWatcher) Close() error                                              { return nil }

type EventRouter struct {
	watcher EventWatcher
	routes  map[EventType][]EventHandler
	mu      sync.RWMutex
}

func NewEventRouter(watcher EventWatcher) *EventRouter {
	if watcher == nil {
		watcher = &NopWatcher{}
	}
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

func (r *EventRouter) Route(event Event) {
	r.mu.RLock()
	handlers := r.routes[event.Type]
	r.mu.RUnlock()

	log.Printf("[EventRouter] Routing event type=%s, handlers=%d", event.Type, len(handlers))

	for _, h := range handlers {
		go func(handler EventHandler, e Event) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventRouter] Handler panic: %v", r)
				}
			}()
			handler(e)
		}(h, event)
	}
}

func hasCouncilReviews(plan map[string]any) bool {
	reviews := plan["council_reviews"]
	if reviews == nil {
		return false
	}

	switch v := reviews.(type) {
	case []interface{}:
		return len(v) > 0
	case []map[string]interface{}:
		return len(v) > 0
	case string:
		return v != "" && v != "[]" && v != "null"
	default:
		return false
	}
}
