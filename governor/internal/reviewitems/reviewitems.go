package reviewitems

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

// ReviewItem represents a single item in the unified review queue.
type ReviewItem struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	SourceID    string          `json:"source_id"`
	Title       string          `json:"title"`
	Summary     string          `json:"summary,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Priority    string          `json:"priority"`
	HumanNotes  string          `json:"human_notes,omitempty"`
	ReviewedAt  *time.Time      `json:"reviewed_at,omitempty"`
	ReviewedBy  string          `json:"reviewed_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Valid types, statuses, and priorities (must match DB CHECK constraints).
var ValidTypes = []string{
	"visual_qa", "design_preview", "research", "contradiction",
	"council", "credit_alert", "task_review",
}

var ValidStatuses = []string{
	"pending", "approved", "rejected", "deferred", "flagged", "resolved",
}

var ValidPriorities = []string{
	"critical", "high", "medium", "low",
}

// Create inserts a new review item into the database.
func Create(ctx context.Context, database db.Database, item ReviewItem) (*ReviewItem, error) {
	if item.Type == "" || item.SourceID == "" || item.Title == "" {
		return nil, fmt.Errorf("review item requires type, source_id, and title")
	}
	if item.Status == "" {
		item.Status = "pending"
	}
	if item.Priority == "" {
		item.Priority = "medium"
	}
	if item.Payload == nil {
		item.Payload = json.RawMessage(`{}`)
	}

	data := map[string]any{
		"type":       item.Type,
		"source_id":  item.SourceID,
		"title":      item.Title,
		"summary":    item.Summary,
		"payload":    string(item.Payload),
		"status":     item.Status,
		"priority":   item.Priority,
		"updated_at": time.Now(),
	}

	result, err := database.Insert(ctx, "review_items", data)
	if err != nil {
		return nil, fmt.Errorf("insert review_item: %w", err)
	}

	var created ReviewItem
	if err := json.Unmarshal(result, &created); err != nil {
		return nil, fmt.Errorf("parse inserted review_item: %w", err)
	}
	return &created, nil
}

// List returns review items filtered by status and optionally by type.
// If status is empty, returns all pending items.
func List(ctx context.Context, database db.Database, status string, itemType string) ([]ReviewItem, error) {
	filters := map[string]any{
		"order": "priority.desc,created_at.desc",
	}
	if status != "" {
		filters["status"] = status
	} else {
		filters["status"] = "pending"
	}
	if itemType != "" {
		filters["type"] = itemType
	}

	result, err := database.Query(ctx, "review_items", filters)
	if err != nil {
		return nil, fmt.Errorf("query review_items: %w", err)
	}

	var items []ReviewItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("parse review_items: %w", err)
	}
	return items, nil
}

// UpdateStatus changes the status of a review item and records who reviewed it.
func UpdateStatus(ctx context.Context, database db.Database, id string, status string, notes string) (*ReviewItem, error) {
	now := time.Now()
	data := map[string]any{
		"status":      status,
		"human_notes": notes,
		"reviewed_at": now,
		"reviewed_by": "human",
		"updated_at":  now,
	}

	result, err := database.Update(ctx, "review_items", id, data)
	if err != nil {
		return nil, fmt.Errorf("update review_item %s: %w", id, err)
	}

	var updated ReviewItem
	if err := json.Unmarshal(result, &updated); err != nil {
		return nil, fmt.Errorf("parse updated review_item: %w", err)
	}
	return &updated, nil
}

// Exists checks if a review item with the given type and source_id already exists
// with the given status. Prevents duplicate review items.
func Exists(ctx context.Context, database db.Database, itemType string, sourceID string, status string) (bool, error) {
	filters := map[string]any{
		"type":      itemType,
		"source_id": sourceID,
	}
	if status != "" {
		filters["status"] = status
	}

	result, err := database.Query(ctx, "review_items", filters)
	if err != nil {
		return false, err
	}
	var items []ReviewItem
	if err := json.Unmarshal(result, &items); err != nil {
		return false, err
	}
	return len(items) > 0, nil
}
