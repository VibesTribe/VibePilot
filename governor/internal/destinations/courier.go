package destinations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	CourierPollIntervalSecs = 5
)

type CourierDB interface {
	Insert(ctx context.Context, table string, data map[string]any) (json.RawMessage, error)
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	Update(ctx context.Context, table, id string, data map[string]any) (json.RawMessage, error)
}

type CourierRunner struct {
	githubToken string
	githubRepo  string
	db          CourierDB
	httpClient  *http.Client
	timeout     time.Duration
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
	}
}

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

	if llmProvider == "" || llmModel == "" || llmAPIKey == "" {
		return "", 0, 0, fmt.Errorf("orchestrator must provide browser_llm_provider, browser_llm_model, browser_llm_api_key")
	}
	if webPlatformURL == "" {
		return "", 0, 0, fmt.Errorf("orchestrator must provide web_platform_url")
	}

	effectiveTimeout := r.timeout
	if timeout > 0 {
		effectiveTimeout = time.Duration(timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	_, err := r.db.Insert(ctx, "task_runs", map[string]any{
		"id":         taskID,
		"status":     "running",
		"started_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("create task_run: %w", err)
	}

	if err := r.dispatch(ctx, taskID, taskPrompt, branchName, llmProvider, llmModel, llmAPIKey, webPlatformURL, supabaseURL, supabaseKey); err != nil {
		r.failTaskRun(ctx, taskID, err.Error())
		return "", 0, 0, err
	}

	result, err := r.pollCompletion(ctx, taskID)
	if err != nil {
		return "", 0, 0, err
	}

	return result.Output, result.TokensIn, result.TokensOut, nil
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

type taskRunResult struct {
	Status    string `json:"status"`
	Output    string `json:"output"`
	Error     string `json:"error"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

func (r *CourierRunner) pollCompletion(ctx context.Context, taskID string) (*taskRunResult, error) {
	ticker := time.NewTicker(time.Duration(CourierPollIntervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for courier completion")
		case <-ticker.C:
			data, err := r.db.Query(ctx, "task_runs", map[string]any{
				"id":     taskID,
				"select": "status,output,error,tokens_in,tokens_out",
			})
			if err != nil {
				continue
			}

			var results []taskRunResult
			if err := json.Unmarshal(data, &results); err != nil {
				continue
			}

			if len(results) == 0 {
				continue
			}

			result := &results[0]
			if result.Status == "running" {
				continue
			}

			if result.Status == "failed" {
				return nil, fmt.Errorf("courier failed: %s", result.Error)
			}

			return result, nil
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
