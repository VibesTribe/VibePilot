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

func TestCheckpointRecovery_ReviewStep(t *testing.T) {
	mockDB := &mockDB{
		checkpoints: []map[string]any{
			{
				"task_id":     "task-456",
				"task_number": "T002",
				"status":      "in_progress",
				"step":        "review",
				"progress":    100,
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

	if tasks[0]["step"] != "review" {
		t.Errorf("expected step review, got %s", tasks[0]["step"])
	}
}

func TestCheckpointRecovery_TestingStep(t *testing.T) {
	mockDB := &mockDB{
		checkpoints: []map[string]any{
			{
				"task_id":     "task-789",
				"task_number": "T003",
				"status":      "testing",
				"step":        "testing",
				"progress":    95,
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

	if tasks[0]["step"] != "testing" {
		t.Errorf("expected step testing, got %s", tasks[0]["step"])
	}
}

func TestCheckpointRecovery_MultipleTasks(t *testing.T) {
	mockDB := &mockDB{
		checkpoints: []map[string]any{
			{
				"task_id":     "task-1",
				"task_number": "T001",
				"status":      "in_progress",
				"step":        "execution",
				"progress":    25,
			},
			{
				"task_id":     "task-2",
				"task_number": "T002",
				"status":      "in_progress",
				"step":        "review",
				"progress":    100,
			},
			{
				"task_id":     "task-3",
				"task_number": "T003",
				"status":      "testing",
				"step":        "testing",
				"progress":    100,
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

	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	steps := make(map[string]int)
	for _, task := range tasks {
		step := task["step"].(string)
		steps[step]++
	}

	if steps["execution"] != 1 {
		t.Errorf("expected 1 execution task, got %d", steps["execution"])
	}
	if steps["review"] != 1 {
		t.Errorf("expected 1 review task, got %d", steps["review"])
	}
	if steps["testing"] != 1 {
		t.Errorf("expected 1 testing task, got %d", steps["testing"])
	}
}

func TestCheckpointRecovery_JSONBParameter(t *testing.T) {
	mockDB := &mockDB{checkpoints: []map[string]any{}}

	ctx := context.Background()
	database := mockDB

	params := map[string]any{
		"p_statuses": []string{"in_progress", "review", "testing"},
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Errorf("failed to marshal params: %v", err)
	}

	var unmarshaled map[string]any
	if err := json.Unmarshal(paramsJSON, &unmarshaled); err != nil {
		t.Errorf("failed to unmarshal params: %v", err)
	}

	statuses, ok := unmarshaled["p_statuses"].([]interface{})
	if !ok {
		t.Errorf("p_statuses should be array")
	}

	if len(statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(statuses))
	}

	_, err = database.RPC(ctx, "find_tasks_with_checkpoints", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckpointRecovery_ProgressValues(t *testing.T) {
	testCases := []struct {
		name     string
		progress int
	}{
		{"zero progress", 0},
		{"quarter progress", 25},
		{"half progress", 50},
		{"three quarter progress", 75},
		{"full progress", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &mockDB{
				checkpoints: []map[string]any{
					{
						"task_id":     "task-progress",
						"task_number": "T-PROG",
						"status":      "in_progress",
						"step":        "execution",
						"progress":    tc.progress,
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

			progress := int(tasks[0]["progress"].(float64))
			if progress != tc.progress {
				t.Errorf("expected progress %d, got %d", tc.progress, progress)
			}
		})
	}
}

func TestCheckpointRecovery_TaskNumberParsing(t *testing.T) {
	taskNumbers := []string{"T001", "T999", "TASK-ABC-123", "feature-xyz-001"}

	for _, taskNum := range taskNumbers {
		t.Run(taskNum, func(t *testing.T) {
			mockDB := &mockDB{
				checkpoints: []map[string]any{
					{
						"task_id":     "task-" + taskNum,
						"task_number": taskNum,
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

			if tasks[0]["task_number"] != taskNum {
				t.Errorf("expected task_number %s, got %s", taskNum, tasks[0]["task_number"])
			}
		})
	}
}

func TestCheckpointRecovery_StatusFilter(t *testing.T) {
	testCases := []struct {
		name       string
		statuses   []string
		taskStatus string
	}{
		{"in_progress included", []string{"in_progress"}, "in_progress"},
		{"review included", []string{"review"}, "review"},
		{"testing included", []string{"testing"}, "testing"},
		{"completed excluded", []string{"in_progress", "review", "testing"}, "completed"},
		{"merged excluded", []string{"in_progress", "review", "testing"}, "merged"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &mockDB{
				checkpoints: []map[string]any{
					{
						"task_id":     "task-status",
						"task_number": "T-STATUS",
						"status":      tc.taskStatus,
						"step":        "execution",
						"progress":    50,
					},
				},
			}

			ctx := context.Background()
			database := mockDB

			result, err := database.RPC(ctx, "find_tasks_with_checkpoints", map[string]any{
				"p_statuses": tc.statuses,
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

			if tasks[0]["status"] != tc.taskStatus {
				t.Errorf("expected status %s, got %s", tc.taskStatus, tasks[0]["status"])
			}
		})
	}
}
