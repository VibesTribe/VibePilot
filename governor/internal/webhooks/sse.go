package webhooks

import (
	"encoding/json"
	"log"
	"sync"
)

// SSENotification is the generic payload broadcast to all connected dashboards.
// It carries no domain knowledge — just table, action, row ID.
// Clients decide what to do with it.
type SSENotification struct {
	Table  string `json:"table"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

// SSEBroker manages connected SSE clients and broadcasts notifications.
// Zero dependencies on domain types — fully agnostic.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan SSENotification]struct{}
}

// NewSSEBroker creates a new broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan SSENotification]struct{}),
	}
}

// Broadcast pushes a notification to all connected SSE clients.
// Implements pgnotify.SSEBroadcaster interface.
func (b *SSEBroker) Broadcast(table, action, id string) {
	notif := SSENotification{Table: table, Action: action, ID: id}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.clients) == 0 {
		return
	}

	data, _ := json.Marshal(notif)
	log.Printf("[SSE] Broadcasting %s to %d client(s)", string(data), len(b.clients))

	for ch := range b.clients {
		select {
		case ch <- notif:
		default:
			log.Printf("[SSE] Client buffer full, dropping event for %s", table)
		}
	}
}

// Subscribe creates a new client channel and returns it.
func (b *SSEBroker) Subscribe() chan SSENotification {
	ch := make(chan SSENotification, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	log.Printf("[SSE] Client connected (total: %d)", b.count())
	return ch
}

// Unsubscribe removes a client and closes its channel.
func (b *SSEBroker) Unsubscribe(ch chan SSENotification) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
	log.Printf("[SSE] Client disconnected (total: %d)", b.count())
}

func (b *SSEBroker) count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
