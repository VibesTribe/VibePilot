package pgnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/vibepilot/governor/internal/runtime"
)

// SSEBroadcaster is a minimal interface for pushing notifications to dashboard clients.
// Implemented by webhooks.SSEBroker — keeps pgnotify decoupled from HTTP concerns.
type SSEBroadcaster interface {
	Broadcast(table, action, id string)
}

// Listener listens for PostgreSQL NOTIFY events on the 'vp_changes' channel.
// It does two things:
//  1. Maps generic pg_notify payloads to domain-specific runtime events
//     (task_available, plan_created, etc.) and routes them through EventRouter.
//  2. Broadcasts the generic notification to SSE clients for dashboard live updates.
//
// Status-aware mapping: the vp_notify_change() trigger includes `status` and
// `processing_by` fields, so we can determine the right event type without
// querying the row back.
type Listener struct {
	conn       *pgx.Conn
	router     *runtime.EventRouter
	broadcaster SSEBroadcaster
	doneChan   chan struct{}
}

// NewListener creates a new PostgreSQL NOTIFY listener.
func NewListener(ctx context.Context, connString string, router *runtime.EventRouter, broadcaster SSEBroadcaster) (*Listener, error) {
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection string: %w", err)
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	listener := &Listener{
		conn:        conn,
		router:      router,
		broadcaster: broadcaster,
		doneChan:    make(chan struct{}),
	}

	go listener.listenLoop(ctx)

	return listener, nil
}

// Close closes the database connection and stops the listener.
func (l *Listener) Close() error {
	close(l.doneChan)
	if l.conn != nil && !l.conn.IsClosed() {
		if err := l.conn.Close(context.Background()); err != nil {
			log.Printf("[PGNotify] Error closing connection: %v", err)
			return err
		}
	}
	return nil
}

type notifyPayload struct {
	Table         string `json:"table"`
	Action        string `json:"action"`
	ID            string `json:"id"`
	Status        string `json:"status"`
	ProcessingBy  string `json:"processing_by"`
}

func (l *Listener) listenLoop(ctx context.Context) {
	listenCfg, err := pgx.ParseConfig(l.conn.Config().ConnString())
	if err != nil {
		log.Printf("[PGNotify] Failed to parse config for listen connection: %v", err)
		return
	}
	listenConn, err := pgx.ConnectConfig(ctx, listenCfg)
	if err != nil {
		log.Printf("[PGNotify] Failed to connect for listening: %v", err)
		return
	}
	defer listenConn.Close(ctx)

	if _, err := listenConn.Exec(ctx, "LISTEN vp_changes"); err != nil {
		log.Printf("[PGNotify] Failed to LISTEN vp_changes: %v", err)
		return
	}
	log.Printf("[PGNotify] Listening for NOTIFY events on vp_changes")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[PGNotify] Context closed, stopping listener")
			return
		case <-l.doneChan:
			log.Printf("[PGNotify] Done channel closed, stopping listener")
			return
		default:
		}

		// Wait for notification (blocks until one arrives or context cancels)
		n, err := listenConn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[PGNotify] Wait error: %v, reconnecting...", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var payload notifyPayload
		if err := json.Unmarshal([]byte(n.Payload), &payload); err != nil {
			log.Printf("[PGNotify] Failed to parse notification payload: %v", err)
			continue
		}

		// Broadcast generic notification to SSE clients (dashboard)
		if l.broadcaster != nil {
			l.broadcaster.Broadcast(payload.Table, payload.Action, payload.ID)
		}

		// Map to domain-specific event and route internally
		if l.router != nil {
			event := l.mapEvent(payload)
			if event != nil {
				log.Printf("[PGNotify] Routed %s on %s (status: %s)", event.Type, payload.Table, payload.Status)
				l.router.Route(*event)
			}
		}
	}
}

// mapEvent converts a notification payload to a domain-specific runtime.Event.
// Uses status + action + processing_by to determine the right event type,
// matching the logic from the old Supabase realtime client.
func (l *Listener) mapEvent(p notifyPayload) *runtime.Event {
	if p.ID == "" {
		return nil
	}

	var eventType runtime.EventType

	switch {
	case p.Table == "tasks":
		// Skip processing-locked rows — they're mid-operation
		if p.ProcessingBy != "" {
			return nil
		}
		switch {
		case p.Status == "available" && p.Action == "INSERT":
			eventType = runtime.EventTaskAvailable
		case p.Status == "available" && p.Action == "UPDATE":
			eventType = runtime.EventTaskAvailable
		case p.Status == "review":
			eventType = runtime.EventTaskReview
		case p.Status == "testing":
			eventType = runtime.EventTaskTesting
		case p.Status == "approval":
			eventType = runtime.EventTaskApproval
		case p.Status == "merge_pending":
			eventType = runtime.EventTaskMergePending
		}

	case p.Table == "plans":
		if p.ProcessingBy != "" {
			return nil
		}
		switch {
		case p.Status == "draft" && p.Action == "INSERT":
			eventType = runtime.EventPlanCreated
		case p.Status == "draft" && p.Action == "UPDATE":
			eventType = runtime.EventPlanCreated
		case p.Status == "review":
			eventType = runtime.EventPlanReview
		case p.Status == "council_review":
			eventType = runtime.EventCouncilReview
		case p.Status == "council_done":
			eventType = runtime.EventCouncilDone
		case p.Status == "approved":
			eventType = runtime.EventPlanApproved
		case p.Status == "blocked":
			eventType = runtime.EventPlanBlocked
		case p.Status == "revision_needed":
			eventType = runtime.EventRevisionNeeded
		}

	case p.Table == "task_runs":
		if p.Status == "completed" || p.Status == "failed" {
			eventType = runtime.EventCourierResult
		}

	case p.Table == "research_suggestions":
		switch p.Status {
		case "ready":
			eventType = runtime.EventResearchReady
		case "council_review":
			eventType = runtime.EventResearchCouncil
		}

	case p.Table == "maintenance_commands":
		eventType = runtime.EventMaintenanceCmd

	case p.Table == "test_results":
		eventType = runtime.EventTestResults

	// Tables that matter for dashboard but don't trigger internal handlers.
	// SSE already broadcast them above — no internal event needed.
	case p.Table == "orchestrator_events",
		p.Table == "models",
		p.Table == "platforms",
		p.Table == "exchange_rates":
		return nil
	}

	if eventType == "" {
		return nil
	}

	return &runtime.Event{
		Type:      eventType,
		ID:        p.ID,
		Table:     p.Table,
		Record:    nil, // We don't have full record — handlers query if needed
		Timestamp: time.Now(),
	}
}
