package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
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

// CourierRunner dispatches tasks to either local browser-harness (primary) or
// GitHub Actions (fallback) for browser-based execution on web AI platforms.
//
// Local path: connects to Chrome on CDP port 9222, opens a new tab per task,
// types the prompt, waits for the response, extracts it, closes the tab.
// Each task gets its own CDP target — no cursor conflicts between agents.
//
// Remote path: dispatches to GitHub Actions workflow which runs browser-use
// in the cloud and POSTs results back via webhook.
type CourierRunner struct {
	githubToken string
	githubRepo  string
	governorURL string // external URL for GitHub Actions to callback
	db          CourierDB
	httpClient  *http.Client
	timeout     time.Duration

	// waiters maps taskID -> channel for result delivery (GitHub Actions path only)
	waiters map[string]*courierWaiter
	mu      sync.RWMutex
}

// platformConfig defines how to interact with a web AI platform.
type platformConfig struct {
	InputX           int
	InputY           int
	ResponseSelector string
	SubmitKey        string
	WaitInitial      int
	WaitStable       int
}

// platformConfigs maps platform domain substrings to their interaction configs.
// These are verified working values, not guesses.
var platformConfigs = map[string]platformConfig{
	"gemini.google": {
		InputX:           482,
		InputY:           283,
		ResponseSelector: "message-content",
		SubmitKey:        "Enter",
		WaitInitial:      8,
		WaitStable:       3,
	},
	"chatgpt.com": {
		InputX:           640,
		InputY:           700,
		ResponseSelector: "[data-message-author-role='assistant']",
		SubmitKey:        "Enter",
		WaitInitial:      8,
		WaitStable:       3,
	},
	"chat.openai.com": {
		InputX:           640,
		InputY:           700,
		ResponseSelector: "[data-message-author-role='assistant']",
		SubmitKey:        "Enter",
		WaitInitial:      8,
		WaitStable:       3,
	},
	"claude.ai": {
		InputX:           640,
		InputY:           700,
		ResponseSelector: "[data-testid='assistant-message']",
		SubmitKey:        "Enter",
		WaitInitial:      10,
		WaitStable:       3,
	},
	"chat.deepseek.com": {
		InputX:           640,
		InputY:           700,
		ResponseSelector: ".ds-markdown",
		SubmitKey:        "Enter",
		WaitInitial:      8,
		WaitStable:       3,
	},
}

// DefaultTimeoutSecs is defined in runners.go in this package.

func NewCourierRunner(githubToken, githubRepo string, db CourierDB, timeoutSecs int) *CourierRunner {
	timeout := DefaultTimeoutSecs
	if timeoutSecs > 0 {
		timeout = timeoutSecs
	}
	return &CourierRunner{
		githubToken: githubToken,
		githubRepo:  githubRepo,
		governorURL: "http://localhost:8080",
		db:          db,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		timeout:     time.Duration(timeout) * time.Second,
		waiters:     make(map[string]*courierWaiter),
	}
}

// SetGovernorURL sets the external callback URL for courier result delivery.
// Must be called before Run() if using GitHub Actions dispatch (needs public URL).
func (r *CourierRunner) SetGovernorURL(url string) {
	if url != "" {
		r.governorURL = url
	}
}

// cdpAvailable checks if Chrome is running with CDP on localhost:9222.
// This determines whether we can use the local browser-harness path.
func (r *CourierRunner) cdpAvailable(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, "GET", "http://localhost:9222/json/version", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// detectPlatform finds the platform config matching the given URL.
func detectPlatform(url string) (string, platformConfig, bool) {
	for domain, cfg := range platformConfigs {
		if strings.Contains(strings.ToLower(url), domain) {
			return domain, cfg, true
		}
	}
	return "", platformConfig{}, false
}

// dispatchLocal runs a courier task locally via browser-harness connected to Chrome CDP.
// Opens a new tab, types the prompt, waits for the response, extracts it, closes the tab.
// Returns the result synchronously — no webhook callback needed.
func (r *CourierRunner) dispatchLocal(ctx context.Context, taskID, taskPrompt, platformURL string) (*TaskRunResult, error) {
	// Detect platform and get interaction config
	platformName, pcfg, found := detectPlatform(platformURL)
	if !found {
		return nil, fmt.Errorf("no platform config for URL: %s", platformURL)
	}

	// Write task packet to temp file for the browser-harness script to read
	taskData := map[string]any{
		"task_id":          taskID,
		"prompt":           taskPrompt,
		"web_platform_url": platformURL,
	}
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("marshal task data: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "courier_task_*.json")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(taskJSON); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Generate the browser-harness script
	// Note: double braces {{ }} for literal braces in fmt.Sprintf with maps
	bhScript := fmt.Sprintf(`import json, time
with open("%s") as f:
    task = json.load(f)
prompt = task.get("prompt", "")
platform_url = task.get("web_platform_url", "")
tid = new_tab(platform_url)
try:
    wait_for_load()
    time.sleep(3)
    click_at_xy(%d, %d)
    time.sleep(0.5)
    type_text(prompt)
    time.sleep(0.5)
    press_key("%s")
    time.sleep(%d)
    last_text = ""
    stable_count = 0
    for i in range(36):
        time.sleep(5)
        current = js("document.body.innerText")
        if current == last_text:
            stable_count += 1
            if stable_count >= %d:
                break
        else:
            stable_count = 0
            last_text = current
    response = js("(function() { var els = document.querySelectorAll('%s'); var t = ''; for (var i = 0; i < els.length; i++) { t += els[i].textContent; } return t; })()")
    result = {"status": "success", "output": response, "tokens_in": len(prompt) // 4, "tokens_out": len(response) // 4}
    print(json.dumps(result))
except Exception as e:
    print(json.dumps({"status": "error", "error": str(e)}))
finally:
    try:
        cdp("Target.closeTarget", targetId=tid)
    except:
        pass
`, tmpPath, pcfg.InputX, pcfg.InputY, pcfg.SubmitKey, pcfg.WaitInitial, pcfg.WaitStable, pcfg.ResponseSelector)

	// Execute browser-harness
	cmd := exec.CommandContext(ctx, "browser-harness", "-c", bhScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logPrefix := fmt.Sprintf("[CourierLocal:%s:%s]", truncateCourierID(taskID), platformName)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s browser-harness failed: %w\nstderr: %s", logPrefix, err, stderr.String())
	}

	// Parse result from stdout
	var result TaskRunResult
	output := strings.TrimSpace(stdout.String())
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("%s parse result: %w\nstdout: %s", logPrefix, err, output)
	}

	if result.Status == "error" {
		return nil, fmt.Errorf("%s platform error: %s", logPrefix, result.Error)
	}

	return &result, nil
}

// Run dispatches a courier task and waits for the result.
// Primary path: local browser-harness via CDP if Chrome is running on port 9222.
// Fallback path: GitHub Actions with webhook callback if no local Chrome.
func (r *CourierRunner) Run(ctx context.Context, prompt string, timeout int) (string, int, int, error) {
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
		branchName = "task/" + taskID[:minInt(8, len(taskID))]
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

	if webPlatformURL == "" {
		return "", 0, 0, fmt.Errorf("orchestrator must provide web_platform_url")
	}

	effectiveTimeout := r.timeout
	if timeout > 0 {
		effectiveTimeout = time.Duration(timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, effectiveTimeout)
	defer cancel()

	// Decide execution path
	useLocal := r.cdpAvailable(ctx)
	courierType := "github-actions"
	if useLocal {
		courierType = "local-browser-harness"
	}

	// Create task_runs row
	_, err := r.db.Insert(ctx, "task_runs", map[string]any{
		"task_id":    taskID,
		"status":     "running",
		"courier":    courierType,
		"started_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("create task_run: %w", err)
	}

	if useLocal {
		// LOCAL PATH: browser-harness via CDP
		logPrefix := fmt.Sprintf("[CourierLocal:%s]", truncateCourierID(taskID))
		result, err := r.dispatchLocal(ctx, taskID, taskPrompt, webPlatformURL)
		if err != nil {
			r.failTaskRun(ctx, taskID, err.Error())
			return "", 0, 0, err
		}

		// Update task_runs with success
		r.db.Update(ctx, "task_runs", taskID, map[string]any{
			"status":       "success",
			"completed_at": time.Now().UTC().Format(time.RFC3339),
			"tokens_used":  result.TokensIn + result.TokensOut,
		})

		fmt.Printf("%s completed in local mode (tokens: %d/%d)\n", logPrefix, result.TokensIn, result.TokensOut)
		return result.Output, result.TokensIn, result.TokensOut, nil
	}

	// REMOTE PATH: GitHub Actions with webhook callback (existing logic)
	if r.githubToken == "" {
		return "", 0, 0, fmt.Errorf("neither local CDP nor GITHUB_TOKEN available")
	}
	if r.githubRepo == "" {
		return "", 0, 0, fmt.Errorf("neither local CDP nor GITHUB_REPO configured")
	}

	w := r.registerWaiter(taskID)
	defer r.unregisterWaiter(taskID)

	if err := r.dispatch(ctx, taskID, taskPrompt, branchName, llmProvider, llmModel, llmAPIKey, webPlatformURL); err != nil {
		r.failTaskRun(ctx, taskID, err.Error())
		return "", 0, 0, err
	}

	// Wait for result via channel (fed by /api/courier/result POST callback)
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

// dispatch sends the task to GitHub Actions for remote browser-based execution.
func (r *CourierRunner) dispatch(ctx context.Context, taskID, taskPrompt, branchName, llmProvider, llmModel, llmAPIKey, webPlatformURL string) error {
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
			"governor_api_url": r.governorURL,
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

func truncateCourierID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
