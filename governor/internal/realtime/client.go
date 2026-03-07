// Package realtime provides a Supabase Realtime client for listening to database changes.
//
// This replaces the broken pg_net webhooks with WebSocket-based change notifications.
// Benefits:
//   - No egress charges (inbound WebSocket connection)
//   - Real-time (instant notifications)
//   - No pg_net dependency (works even when pg_net is broken)
//   - Included in Supabase free tier
//
// Architecture:
//   - Connects to Supabase Realtime via WebSocket
//   - Subscribes to Postgres Changes on specified tables
//   - Routes events to the existing EventRouter
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/vibepilot/governor/internal/runtime"
)

// Client connects to Supabase Realtime and listens for database changes.
type Client struct {
	url             string
	apiKey          string
	conn            *websocket.Conn
	router          *runtime.EventRouter
	subscriptions   map[string]*Subscription
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	connected       bool
	refCounter      int64
	heartbeatTicker *time.Ticker
	isReconnect     bool // Track if this is a reconnect
}

// Subscription represents a subscription to a table's changes.
type Subscription struct {
	Channel string
	Table   string
	Event   string // "*", "INSERT", "UPDATE", "DELETE"
	Schema  string // "public"
	Ref     string
}

// Config for the Realtime client.
type Config struct {
	URL    string // e.g., "wss://xyz.supabase.co/realtime/v1/websocket"
	APIKey string // anon key or service key
}

// Phoenix channel message types
type phoenixMessage struct {
	Topic   string          `json:"topic"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
	Ref     string          `json:"ref,omitempty"`
}

type postgresChangesPayload struct {
	Config postgresChangesConfig `json:"config"`
}

type postgresChangesConfig struct {
	Event  string `json:"event"`  // "*", "INSERT", "UPDATE", "DELETE"
	Schema string `json:"schema"` // "public"
	Table  string `json:"table"`  // table name
}

type channelResponse struct {
	Status   string `json:"status"`
	Response struct {
		PostgresChanges []struct {
			ID int `json:"id"`
		} `json:"postgres_changes"`
	} `json:"response"`
}

// ChangeEvent represents a database change event from Realtime.
type ChangeEvent struct {
	// Supabase wraps the actual change in a "data" field
	Data struct {
		Table     string                 `json:"table"`
		Type      string                 `json:"type"` // INSERT, UPDATE, DELETE
		Schema    string                 `json:"schema"`
		Record    map[string]interface{} `json:"record"`
		OldRecord map[string]interface{} `json:"old_record"`
		Columns   []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		CommitTimestamp string      `json:"commit_timestamp"`
		Errors          interface{} `json:"errors"`
	} `json:"data"`
	// For backwards compatibility, also support direct fields
	EventType string                 `json:"event_type"`
	Table     string                 `json:"table"`
	Schema    string                 `json:"schema"`
	New       map[string]interface{} `json:"new"`
	Old       map[string]interface{} `json:"old"`
}

// normalize extracts data from the nested structure into flat fields.
func (ce *ChangeEvent) normalize() {
	// If data is present, use it
	if ce.Data.Table != "" {
		ce.Table = ce.Data.Table
		ce.EventType = ce.Data.Type
		ce.Schema = ce.Data.Schema
		ce.New = ce.Data.Record
		ce.Old = ce.Data.OldRecord
	}
}

// NewClient creates a new Realtime client.
func NewClient(cfg *Config, router *runtime.EventRouter) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		url:           cfg.URL,
		apiKey:        cfg.APIKey,
		router:        router,
		subscriptions: make(map[string]*Subscription),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Connect establishes a WebSocket connection to Supabase Realtime.
func (c *Client) Connect() error {
	// Build URL with parameters
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("parse realtime URL: %w", err)
	}

	// Add required parameters
	q := u.Query()
	q.Set("apikey", c.apiKey)
	q.Set("vsn", "1.0.0")
	q.Set("log_level", "info")
	u.RawQuery = q.Encode()

	log.Printf("[Realtime] Connecting to %s", u.Host)

	// Connect with headers
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.apiKey)

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Start heartbeat to keep connection alive
	c.startHeartbeat()

	// Start message handler
	go c.readMessages()

	// Only re-subscribe on reconnect (not first connect)
	if c.isReconnect {
		go c.resubscribeAll()
	}
	c.isReconnect = true // Mark that future connects are reconnects

	log.Printf("[Realtime] Connected successfully")
	return nil
}

// SubscribeToTable subscribes to all events (INSERT, UPDATE, DELETE) on a specific table.
// We need UPDATE events to detect status changes (draft → review, available → in_progress, etc.)
func (c *Client) SubscribeToTable(table string) error {
	return c.SubscribeToTableWithFilter(table, "*", "")
}

// SubscribeToTableWithFilter subscribes to changes with specific event type.
func (c *Client) SubscribeToTableWithFilter(table, event, filter string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	channel := fmt.Sprintf("realtime:public:%s", table)
	if _, exists := c.subscriptions[channel]; exists {
		return nil // Already subscribed
	}

	sub := &Subscription{
		Channel: channel,
		Table:   table,
		Event:   event,
		Schema:  "public",
		Ref:     c.nextRef(),
	}
	c.subscriptions[channel] = sub

	// Send join message
	joinMsg := phoenixMessage{
		Topic: channel,
		Event: "phx_join",
		Ref:   sub.Ref,
		Payload: mustMarshal(map[string]interface{}{
			"config": map[string]interface{}{
				"broadcast": map[string]interface{}{"ack": false, "self": false},
				"presence":  map[string]interface{}{"key": ""},
				"postgres_changes": []map[string]interface{}{
					{
						"event":  event,
						"schema": "public",
						"table":  table,
					},
				},
			},
		}),
	}

	if err := c.sendMessage(joinMsg); err != nil {
		delete(c.subscriptions, channel)
		return fmt.Errorf("send join: %w", err)
	}

	log.Printf("[Realtime] Subscribed to table %s (event: %s)", table, event)
	return nil
}

// SubscribeToAllTables subscribes to changes on all monitored tables.
func (c *Client) SubscribeToAllTables() error {
	tables := []string{
		"plans",
		"tasks",
		"maintenance_commands",
		"research_suggestions",
		"test_results",
	}

	for _, table := range tables {
		if err := c.SubscribeToTable(table); err != nil {
			return fmt.Errorf("subscribe to %s: %w", table, err)
		}
	}

	return nil
}

// startHeartbeat sends periodic heartbeat messages to keep the connection alive.
func (c *Client) startHeartbeat() {
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}
	c.heartbeatTicker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-c.heartbeatTicker.C:
				if !c.IsConnected() {
					continue
				}
				heartbeat := phoenixMessage{
					Topic:   "phoenix",
					Event:   "heartbeat",
					Payload: json.RawMessage("{}"),
					Ref:     c.nextRef(),
				}
				if err := c.sendMessage(heartbeat); err != nil {
					log.Printf("[Realtime] Heartbeat failed: %v", err)
				}
			}
		}
	}()
}

// resubscribeAll re-subscribes to all channels after reconnect.
func (c *Client) resubscribeAll() {
	c.mu.RLock()
	subs := make([]*Subscription, 0, len(c.subscriptions))
	for _, sub := range c.subscriptions {
		subs = append(subs, sub)
	}
	c.mu.RUnlock()

	// Small delay to let connection stabilize
	time.Sleep(500 * time.Millisecond)

	for _, sub := range subs {
		// Re-send join message
		joinMsg := phoenixMessage{
			Topic: sub.Channel,
			Event: "phx_join",
			Ref:   c.nextRef(),
			Payload: mustMarshal(map[string]interface{}{
				"config": map[string]interface{}{
					"broadcast": map[string]interface{}{"ack": false, "self": false},
					"presence":  map[string]interface{}{"key": ""},
					"postgres_changes": []map[string]interface{}{
						{
							"event":  sub.Event,
							"schema": sub.Schema,
							"table":  sub.Table,
						},
					},
				},
			}),
		}

		if err := c.sendMessage(joinMsg); err != nil {
			log.Printf("[Realtime] Re-subscribe failed for %s: %v", sub.Table, err)
		} else {
			log.Printf("[Realtime] Re-subscribed to table %s", sub.Table)
		}
	}
}

// readMessages continuously reads messages from the WebSocket.
func (c *Client) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if c.ctx.Err() != nil {
				return // Context cancelled, normal shutdown
			}
			log.Printf("[Realtime] Read error: %v", err)
			c.handleDisconnect()
			return
		}

		var msg phoenixMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[Realtime] Parse error: %v", err)
			continue
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming WebSocket messages.
func (c *Client) handleMessage(msg phoenixMessage) {
	switch msg.Event {
	case "phx_reply":
		// Response to join - can check status
		var reply channelResponse
		if err := json.Unmarshal(msg.Payload, &reply); err == nil {
			if reply.Status != "ok" {
				log.Printf("[Realtime] Channel join failed: %s", string(msg.Payload))
			}
		}

	case "phx_error":
		log.Printf("[Realtime] Channel error: %s", string(msg.Payload))
		c.handleDisconnect()

	case "phx_close":
		log.Printf("[Realtime] Channel closed: %s", msg.Topic)

	case "postgres_changes":
		c.handlePostgresChange(msg)

	default:
		// Ignore other events (presence, broadcast, etc.)
	}
}

// handlePostgresChange processes a database change event.
func (c *Client) handlePostgresChange(msg phoenixMessage) {
	var change ChangeEvent
	if err := json.Unmarshal(msg.Payload, &change); err != nil {
		log.Printf("[Realtime] Failed to parse change event: %v", err)
		return
	}

	// Normalize nested data structure
	change.normalize()

	// Skip if no new record data
	if change.New == nil {
		change.New = make(map[string]interface{})
	}

	// Route through the existing event router
	if c.router != nil {
		eventType := c.mapToEventType(&change)
		log.Printf("[Realtime] Mapped %s on %s to event type: %s", change.EventType, change.Table, eventType)

		if eventType == "" {
			log.Printf("[Realtime] No event type mapped, skipping")
			return
		}

		event := runtime.Event{
			Type:      runtime.EventType(eventType),
			ID:        extractID(change.New),
			Table:     change.Table,
			Record:    mustMarshal(change.New),
			Timestamp: time.Now(),
		}
		log.Printf("[Realtime] Routing event: Type=%s, ID=%s, Table=%s", event.Type, event.ID, event.Table)
		c.router.Route(event)
	}

	log.Printf("[Realtime] %s on %s (id: %s)", change.EventType, change.Table, extractID(change.New))
}

// mapToEventType converts a change event to an internal event type.
func (c *Client) mapToEventType(change *ChangeEvent) string {
	table := change.Table
	action := change.EventType

	switch {
	case table == "tasks":
		statusRaw := change.New["status"]
		status, ok := statusRaw.(string)
		log.Printf("[Realtime] Task change: statusRaw=%v (%T), status=%q, ok=%v, action=%s", statusRaw, statusRaw, status, ok, action)
		if !ok && statusRaw != nil {
			status = fmt.Sprintf("%v", statusRaw)
			log.Printf("[Realtime] Converted status to string: %q", status)
		}
		switch {
		case status == "available" && action == "INSERT":
			return string(runtime.EventTaskAvailable)
		case status == "available" && action == "UPDATE":
			if oldStatus, _ := change.Old["status"].(string); oldStatus != "available" {
				return string(runtime.EventTaskAvailable)
			}
		case status == "review":
			return string(runtime.EventTaskReview)
		case status == "testing" || status == "approval":
			return string(runtime.EventTaskCompleted)
		}

	case table == "plans":
		status, _ := change.New["status"].(string)
		oldStatus, _ := change.Old["status"].(string)
		switch {
		case status == "draft" && action == "INSERT":
			return string(runtime.EventPlanCreated)
		case status == "review" && (action == "UPDATE" || oldStatus != "review"):
			return string(runtime.EventPlanReview)
		case status == "council_review":
			return string(runtime.EventCouncilReview)
		case status == "council_done":
			return string(runtime.EventCouncilDone)
		case status == "approved":
			return string(runtime.EventPlanApproved)
		case status == "blocked":
			return string(runtime.EventPlanBlocked)
		case status == "revision_needed":
			return string(runtime.EventRevisionNeeded)
		}

	case table == "research_suggestions":
		status, _ := change.New["status"].(string)
		switch status {
		case "ready":
			return string(runtime.EventResearchReady)
		case "council_review":
			return string(runtime.EventResearchCouncil)
		}

	case table == "maintenance_commands":
		return string(runtime.EventMaintenanceCmd)

	case table == "test_results":
		return string(runtime.EventTestResults)
	}

	return ""
}

// sendMessage sends a message over the WebSocket.
func (c *Client) sendMessage(msg phoenixMessage) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	return c.conn.Write(ctx, websocket.MessageText, data)
}

// handleDisconnect handles WebSocket disconnection.
func (c *Client) handleDisconnect() {
	c.mu.Lock()
	c.connected = false
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}
	c.mu.Unlock()

	// Attempt reconnection after delay
	go func() {
		time.Sleep(5 * time.Second)
		log.Printf("[Realtime] Attempting reconnect...")
		if err := c.Connect(); err != nil {
			log.Printf("[Realtime] Reconnect failed: %v", err)
		}
		// resubscribeAll() is called by Connect() when isReconnect is true
	}()
}

// nextRef generates the next message reference number.
func (c *Client) nextRef() string {
	c.refCounter++
	return fmt.Sprintf("%d", c.refCounter)
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.cancel()
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "shutting down")
	}
	return nil
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Helper functions

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func extractID(record map[string]interface{}) string {
	if id, ok := record["id"].(string); ok {
		return id
	}
	return ""
}
