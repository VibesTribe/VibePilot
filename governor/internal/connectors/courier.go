package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type CourierDB interface {
	Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error)
}

// courierWaiter holds a channel that gets signaled when a courier task completes.
type courierWaiter struct {
	result chan *TaskRunResult
}

// CourierRunner dispatches tasks to GitHub Actions for browser-based execution
// and waits for results via Supabase realtime (not polling).
type CourierRunner struct {
	githubToken string
	githubRepo  string
	db          CourierDB
	httpClient  *http.Client
	timeout     time.Duration

	// waiters maps taskID -> channel for realtime result delivery
	waiters map[string]*courierWaiter
	mu      sync.RWMutex
}

func NewCourierRunner(githubToken, githubRepo string, db CourierDB, timeoutSecs int) *CourierRunner {
	timeout := DefaultTimeoutSecs
	if timeoutSecs > 0 {
		timeout = timeoutSecs
	}
	return &CourierRunner{
		githubToken: githubToken,
		githubRepo:  githubRepo,
		db:          db,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		timeout:     time.Duration(timeout) * time.Second,
		waiters:     make(map[string]*courierWaiter),
	}
}

// Run dispatches a courier task to GitHub Actions and waits for the result.
// Results are delivered via Supabase realtime (zero polling).
func (r *CourierRunner) Run(ctx context.Context, prompt string, timeout int) (string, int, int, error) {
	if r.githubToken == "" {
		return "", 0, 0, fmt.Errorf("GITHUB_TOKEN not configured")
	}
	if r.githubRepo == "" {
		return "", 0, 0, fmt.Errorf("GITHUB_REPO not configured")
	}

	var task map[string]any
	if err := json.Unmarshal([]byte(prompt), &task); err != nil {
		return "", 0, 0, fmt.Errorf("parse task packet: %w", err)
	}

	taskID, _ := task["task_id"].(string)
	if taskID == "" {
		return "", 0, 0, fmt.Errorf("task_packet missing task_id")
	}

	taskPrompt, _ := task["task_prompt"].(string)
	if taskPrompt == "" {
		taskPrompt, _ = task["prompt"].(string)
	}

	branchName, _ := task["branch_name"].(string)
	if branchName == "" {
		branchName = "task/" + taskID[:min(8, len(taskID))]
	}

	llmProvider, _ := task["browser_llm_provider"].(string)
	if llmProvider == "" {
		llmProvider, _ = task["llm_provider"].(string)
	}

	llmModel, _ := task["browser_llm_model"].(string)
	if llmModel == "" {
		llmModel, _ = task["llm_model"].(string)
	}

	llmAPIKey, _ := task["browser_llm_api_key"].(string)
	if llmAPIKey == "" {
		llmAPIKey, _ = task["llm_api_key"].(string)
	}

	webPlatformURL, _ := task["web_platform_url"].(string)
	if webPlatformURL == "" {
		webPlatformURL, _ = task["web_platform"].(string)
	}

	supabaseURL, _ := task["supabase_url"].(string)
	supabaseKey, _ := task["supabase_key"].(string)

	if webPlatformURL == "" {
		return "", 0, 0, fmt.Errorf("orchestrator must provide web_platform_url")
	}

	effectiveTimeout := r.timeout
	if timeout > 0 {
		effectiveTimeout = time.Duration(timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	// Register as a waiter BEFORE dispatching, so we don't miss the realtime event
	w := r.registerWaiter(taskID)
	defer r.unregisterWaiter(taskID)

	// Create task_runs row so the GitHub Action can update it
	_, err := r.db.Insert(ctx, "task_runs", map[string]any{
		"task_id":     taskID,
		"status":      "running",
		"courier":     "github-actions",
		"started_at":  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("create task_run: %w", err)
	}

	// Dispatch to GitHub Actions
	if err := r.dispatch(ctx, taskID, taskPrompt, branchName, llmProvider, llmModel, llmAPIKey, webPlatformURL, supabaseURL, supabaseKey); err != nil {
		r.failTaskRun(ctx, taskID, err.Error())
		return "", 0, 0, err
	}

	// Wait for result via channel (fed by Supabase realtime, zero polling)
	result, err := r.waitForCompletion(ctx, w)
	if err != nil {
		return "", 0, 0, err
	}

	return result.Output, result.TokensIn, result.TokensOut, nil
}

// NotifyResult is called by the event handler when a realtime EventCourierResult arrives.
// It finds the waiting goroutine and delivers the result.
func (r *CourierRunner) NotifyResult(taskID string, result *TaskRunResult) {
	r.mu.RLock()
	w, ok := r.waiters[taskID]
	r.mu.RUnlock()

	if ok {
		select {
		case w.result <- result:
		default:
			// Channel full or already delivered, skip
		}
	}
}

func (r *CourierRunner) registerWaiter(taskID string) *courierWaiter {
	w := &courierWaiter{result: make(chan *TaskRunResult, 1)}
	r.mu.Lock()
	r.waiters[taskID] = w
	r.mu.Unlock()
	return w
}

func (r *CourierRunner) unregisterWaiter(taskID string) {
	r.mu.Lock()
	delete(r.waiters, taskID)
	r.mu.Unlock()
}

func (r *CourierRunner) waitForCompletion(ctx context.Context, w *courierWaiter) (*TaskRunResult, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for courier completion")
	case result := <-w.result:
		if result.Status == "failed" {
			return nil, fmt.Errorf("courier failed: %s", result.Error)
		}
		return result, nil
	}
}

func (r *CourierRunner) failTaskRun(ctx context.Context, id, errMsg string) {
	r.db.Update(ctx, "task_runs", id, map[string]any{
		"status":       "failed",
		"error":        errMsg,
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *CourierRunner) dispatch(ctx context.Context, taskID, taskPrompt, branchName, llmProvider, llmModel, llmAPIKey, webPlatformURL, supabaseURL, supabaseKey string) error {
	payload := map[string]interface{}{
		"event_type": "courier_task",
		"client_payload": map[string]interface{}{
			"task_id":          taskID,
			"prompt":           taskPrompt,
			"branch_name":      branchName,
			"llm_provider":     llmProvider,
			"llm_model":        llmModel,
			"llm_api_key":      llmAPIKey,
			"web_platform_url": webPlatformURL,
			"supabase_url":     supabaseURL,
			"supabase_key":     supabaseKey,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/dispatches", r.githubRepo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+r.githubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TaskRunResult holds the result of a courier task execution.
type TaskRunResult struct {
	Status    string `json:"status"`
	Output    string `json:"output"`
	Error     string `json:"error"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
