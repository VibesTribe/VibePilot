package main

import (
	"context"
	"encoding/json"
	"testing"
)

type mockDB struct {
	checkpoints []map[string]any
	calls       []string
}

func (m *mockDB) RPC(ctx context.Context, name string, params map[string]any) ([]byte, error) {
	m.calls = append(m.calls, name)
	switch name {
	case "find_tasks_with_checkpoints":
		data, _ := json.Marshal(m.checkpoints)
		return data, nil
	case "update_task_status", "delete_checkpoint":
		return json.RawMessage("true"), nil
	default:
		return json.RawMessage("{}"), nil
	}
}

func TestCheckpointRecovery_NoCheckpoints(t *testing.T) {
	mockDB := &mockDB{checkpoints: []map[string]any{}}

	ctx := context.Background()
	database := mockDB

	result, err := database.RPC(ctx, "find_tasks_with_checkpoints", map[string]any{
		"p_statuses": []string{"in_progress", "review", "testing"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var tasks []map[string]any
	if err := json.Unmarshal(result, &tasks); err != nil {
		t.Errorf("unexpected unmarshal error: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestCheckpointRecovery_ExecutionStep(t *testing.T) {
	mockDB := &mockDB{
		checkpoints: []map[string]any{
			{
				"task_id":     "task-123",
				"task_number": "T001",
				"status":      "in_progress",
				"step":        "execution",
				"progress":    50,
			},
		},
	}

	ctx := context.Background()
	database := mockDB

	result, err := database.RPC(ctx, "find_tasks_with_checkpoints", map[string]any{
		"p_statuses": []string{"in_progress", "review", "testing"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var tasks []map[string]any
	if err := json.Unmarshal(result, &tasks); err != nil {
		t.Errorf("unexpected unmarshal error: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0]["task_id"] != "task-123" {
		t.Errorf("expected task_id task-123, got %s", tasks[0]["task_id"])
	}
	if tasks[0]["step"] != "execution" {
		t.Errorf("expected step execution, got %s", tasks[0]["step"])
	}
}
