package courier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/vibepilot/governor/pkg/types"
)

type Dispatcher struct {
	client      *github.Client
	owner       string
	repo        string
	workflow    string
	callbackURL string

	queue       chan types.Task
	maxInFlight int
	inFlight    int
	mu          sync.Mutex
	stagger     time.Duration
}

func NewDispatcher(token, owner, repo, workflow, callbackURL string, maxInFlight int) *Dispatcher {
	return &Dispatcher{
		client:      github.NewTokenClient(nil, token),
		owner:       owner,
		repo:        repo,
		workflow:    workflow,
		callbackURL: callbackURL,
		queue:       make(chan types.Task, 100),
		maxInFlight: maxInFlight,
		stagger:     30 * time.Second,
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	log.Println("Courier dispatcher started: max 3 concurrent, 30s stagger")

	for {
		select {
		case <-ctx.Done():
			log.Println("Courier dispatcher shutting down")
			return
		case task := <-d.queue:
			d.waitForSlot(ctx)
			go d.dispatch(ctx, task)
		}
	}
}

func (d *Dispatcher) waitForSlot(ctx context.Context) {
	for {
		d.mu.Lock()
		if d.inFlight < d.maxInFlight {
			d.inFlight++
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, task types.Task) {
	defer func() {
		d.mu.Lock()
		d.inFlight--
		d.mu.Unlock()
	}()

	time.Sleep(d.stagger)

	prompt := ""
	if task.PromptPacket != nil {
		prompt = task.PromptPacket.Prompt
	}

	payload := map[string]interface{}{
		"task_id":      task.ID,
		"prompt":       prompt,
		"slice_id":     task.SliceID,
		"callback_url": d.callbackURL,
		"branch_name":  fmt.Sprintf("task/%s", task.ID[:8]),
		"title":        task.Title,
		"type":         task.Type,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Courier: failed to marshal payload for %s: %v", task.ID[:8], err)
		return
	}
	rawPayload := json.RawMessage(payloadBytes)

	dispatch := github.DispatchRequestOptions{
		EventType:     "courier_task",
		ClientPayload: &rawPayload,
	}

	_, _, err = d.client.Repositories.Dispatch(ctx, d.owner, d.repo, dispatch)
	if err != nil {
		log.Printf("Courier: dispatch failed for %s: %v", task.ID[:8], err)
	} else {
		log.Printf("Courier: dispatched %s to GitHub Actions (slice=%s)", task.ID[:8], task.SliceID)
	}
}

func (d *Dispatcher) Enqueue(task types.Task) {
	select {
	case d.queue <- task:
		log.Printf("Courier: queued task %s", task.ID[:8])
	default:
		log.Printf("Courier: queue full, cannot enqueue %s", task.ID[:8])
	}
}

func (d *Dispatcher) QueueSize() int {
	return len(d.queue)
}

func (d *Dispatcher) InFlight() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.inFlight
}
