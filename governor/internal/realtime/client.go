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
	url           string
	apiKey        string
	conn          *websocket.Conn
	router        *runtime.EventRouter
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	connected     bool
	refCounter    int64
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
	Columns []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"columns"`
	CommitTimestamp string                 `json:"commit_timestamp"`
	EventType       string                 `json:"event_type"`
	Schema          string                 `json:"schema"`
	Table           string                 `json:"table"`
	New             map[string]interface{} `json:"new"`
	Old             map[string]interface{} `json:"old"`
	Errors          interface{}            `json:"errors"`
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

	// Start message handler
	go c.readMessages()

	log.Printf("[Realtime] Connected successfully")
	return nil
}

// SubscribeToTable subscribes to all changes on a specific table.
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

	// Skip if no new record data
	if change.New == nil {
		change.New = make(map[string]interface{})
	}

	// Route through the existing event router
	if c.router != nil {
		event := runtime.Event{
			Type:      runtime.EventType(c.mapToEventType(&change)),
			ID:        extractID(change.New),
			Table:     change.Table,
			Record:    mustMarshal(change.New),
			Timestamp: time.Now(),
		}
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
		status, _ := change.New["status"].(string)
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
		switch status {
		case "draft":
			return string(runtime.EventPlanCreated)
		case "review":
			return string(runtime.EventPlanReview)
		case "council_review":
			return string(runtime.EventCouncilReview)
		case "council_done":
			return string(runtime.EventCouncilDone)
		case "approved":
			return string(runtime.EventPlanApproved)
		case "blocked":
			return string(runtime.EventPlanBlocked)
		case "revision_needed":
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
	c.mu.Unlock()

	// Attempt reconnection after delay
	go func() {
		time.Sleep(5 * time.Second)
		log.Printf("[Realtime] Attempting reconnect...")
		if err := c.Connect(); err != nil {
			log.Printf("[Realtime] Reconnect failed: %v", err)
		} else {
			// Re-subscribe to all tables
			c.SubscribeToAllTables()
		}
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
