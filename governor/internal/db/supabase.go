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

func (d *DB) request(ctx context.Context, method, table string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := fmt.Sprintf("%s/rest/v1/%s", d.url, table)
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
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase error %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (d *DB) GetAvailableTasks(ctx context.Context) ([]types.Task, error) {
	data, err := d.request(ctx, "GET", "tasks?status=eq.available&order=priority.asc,created_at.asc&limit=10", nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}

	return tasks, nil
}

func (d *DB) GetTaskPacket(ctx context.Context, taskID string) (*types.PromptPacket, error) {
	url := fmt.Sprintf("task_packets?task_id=eq.%s&order=version.desc&limit=1", taskID)
	data, err := d.request(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var packets []types.PromptPacket
	if err := json.Unmarshal(data, &packets); err != nil {
		return nil, fmt.Errorf("unmarshal packet: %w", err)
	}

	if len(packets) == 0 {
		return nil, nil
	}

	return &packets[0], nil
}

func (d *DB) ClaimTask(ctx context.Context, taskID, modelID string) error {
	body := map[string]interface{}{
		"status":      "in_progress",
		"assigned_to": modelID,
		"started_at":  "now()",
		"updated_at":  "now()",
	}

	url := fmt.Sprintf("tasks?id=eq.%s&status=eq.available", taskID)
	data, err := d.request(ctx, "PATCH", url, body)
	if err != nil {
		return err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("unmarshal claim result: %w", err)
	}

	if len(result) == 0 {
		return fmt.Errorf("task %s not available for claiming", taskID)
	}

	return nil
}

func (d *DB) UpdateTaskStatus(ctx context.Context, taskID string, status types.TaskStatus) error {
	body := map[string]interface{}{
		"status":     string(status),
		"updated_at": "now()",
	}

	url := fmt.Sprintf("tasks?id=eq.%s", taskID)
	_, err := d.request(ctx, "PATCH", url, body)
	return err
}

func (d *DB) GetStuckTasks(ctx context.Context, timeout time.Duration) ([]types.Task, error) {
	cutoff := time.Now().Add(-timeout).Format(time.RFC3339)
	url := fmt.Sprintf("tasks?status=eq.in_progress&updated_at=lt.%s", cutoff)
	
	data, err := d.request(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal stuck tasks: %w", err)
	}

	return tasks, nil
}

func (d *DB) ResetTask(ctx context.Context, taskID string, escalate bool) error {
	status := "available"
	if escalate {
		status = "escalated"
	}

	body := map[string]interface{}{
		"status":     status,
		"attempts":   "attempts + 1",
		"started_at": nil,
		"updated_at": "now()",
	}

	url := fmt.Sprintf("tasks?id=eq.%s", taskID)
	_, err := d.request(ctx, "PATCH", url, body)
	return err
}

func (d *DB) IncrementModelUsage(ctx context.Context, modelID string, tokensIn, tokensOut int, success bool) error {
	inc := ""
	if success {
		inc = "tasks_completed = tasks_completed + 1"
	} else {
		inc = "tasks_failed = tasks_failed + 1"
	}

	body := map[string]interface{}{
		"tokens_used":   fmt.Sprintf("tokens_used + %d", tokensIn+tokensOut),
		"updated_at":    "now()",
	}

	url := fmt.Sprintf("models?id=eq.%s", modelID)
	_, err := d.request(ctx, "PATCH", url, body)
	_ = inc
	return err
}
