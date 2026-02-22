package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vibepilot/governor/pkg/types"
)

type DB struct {
	url    string
	key    string
	client *http.Client
}

func New(url, key string) *DB {
	return &DB{
		url: url,
		key: key,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *DB) Close() error {
	d.client.CloseIdleConnections()
	return nil
}

func (d *DB) rest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := d.url + "/rest/v1/" + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apikey", d.key)
	req.Header.Set("Authorization", "Bearer "+d.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (d *DB) rpc(ctx context.Context, name string, params interface{}) ([]byte, error) {
	return d.rest(ctx, "POST", "rpc/"+name, params)
}

func (d *DB) GetAvailableTasks(ctx context.Context, limit int) ([]types.Task, error) {
	path := fmt.Sprintf("tasks?status=eq.available&order=priority.asc,created_at.asc&limit=%d", limit)
	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}
	return tasks, nil
}

func (d *DB) GetTaskByID(ctx context.Context, taskID string) (*types.Task, error) {
	data, err := d.rest(ctx, "GET", "tasks?id=eq."+taskID, nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], nil
}

type taskPacketRow struct {
	TaskID  string `json:"task_id"`
	Prompt  string `json:"prompt"`
	Version int    `json:"version"`
}

func (d *DB) GetTaskPacket(ctx context.Context, taskID string) (*types.PromptPacket, error) {
	path := fmt.Sprintf("task_packets?task_id=eq.%s&order=version.desc&limit=1", taskID)
	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var rows []taskPacketRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal packet rows: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var packet types.PromptPacket
	if err := json.Unmarshal([]byte(rows[0].Prompt), &packet); err != nil {
		return nil, fmt.Errorf("unmarshal prompt JSON: %w", err)
	}
	return &packet, nil
}

func (d *DB) ClaimTask(ctx context.Context, taskID, modelID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"status":      "in_progress",
		"assigned_to": modelID,
		"started_at":  now,
		"updated_at":  now,
	}

	data, err := d.rest(ctx, "PATCH", "tasks?id=eq."+taskID+"&status=eq.available", body)
	if err != nil {
		return err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("unmarshal claim result: %w", err)
	}
	if len(result) == 0 {
		return fmt.Errorf("task %s not available", taskID)
	}
	return nil
}

type TaskRunInput struct {
	TaskID    string
	ModelID   string
	Courier   string
	Platform  string
	Status    string
	Result    interface{}
	TokensIn  int
	TokensOut int
}

func (d *DB) RecordTaskRun(ctx context.Context, input *TaskRunInput) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"task_id":     input.TaskID,
		"model_id":    input.ModelID,
		"courier":     input.Courier,
		"platform":    input.Platform,
		"status":      input.Status,
		"result":      input.Result,
		"tokens_in":   input.TokensIn,
		"tokens_out":  input.TokensOut,
		"tokens_used": input.TokensIn + input.TokensOut,
		"started_at":  now,
	}

	data, err := d.rest(ctx, "POST", "task_runs", body)
	if err != nil {
		return "", err
	}

	var result []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal run result: %w", err)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("no run ID returned")
	}
	return result[0].ID, nil
}

func (d *DB) CallROIRPC(ctx context.Context, runID string) error {
	_, err := d.rpc(ctx, "calculate_enhanced_task_roi", map[string]interface{}{
		"p_run_id": runID,
	})
	return err
}

func (d *DB) UpdateTaskStatus(ctx context.Context, taskID string, status types.TaskStatus, result interface{}) error {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"status":     string(status),
		"updated_at": now,
	}
	if result != nil {
		body["result"] = result
	}

	_, err := d.rest(ctx, "PATCH", "tasks?id=eq."+taskID, body)
	return err
}

func (d *DB) UnlockDependents(ctx context.Context, taskID string) error {
	_, err := d.rpc(ctx, "unlock_dependent_tasks", map[string]interface{}{
		"p_completed_task_id": taskID,
	})
	return err
}

func (d *DB) ResetTask(ctx context.Context, taskID string, escalate bool) error {
	task, err := d.GetTaskByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	status := "available"
	if escalate {
		status = "escalated"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"status":      status,
		"attempts":    task.Attempts + 1,
		"assigned_to": nil,
		"started_at":  nil,
		"updated_at":  now,
	}

	_, err = d.rest(ctx, "PATCH", "tasks?id=eq."+taskID, body)
	return err
}

func (d *DB) GetStuckTasks(ctx context.Context, timeout time.Duration) ([]types.Task, error) {
	cutoff := time.Now().Add(-timeout).UTC().Format(time.RFC3339)
	path := fmt.Sprintf("tasks?status=eq.in_progress&updated_at=lt.%s", cutoff)

	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal stuck tasks: %w", err)
	}
	return tasks, nil
}
