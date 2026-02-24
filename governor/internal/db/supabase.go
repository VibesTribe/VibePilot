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
		return nil, fmt.Errorf("supabase %d: %s", resp.StatusCode, sanitizeErrorBody(data))
	}

	return data, nil
}

func sanitizeErrorBody(data []byte) string {
	s := string(data)
	if len(s) > 200 {
		s = s[:200] + "...(truncated)"
	}
	return s
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
		return nil, fmt.Errorf("task %s not found", taskID)
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

func (d *DB) UpdateTaskBranch(ctx context.Context, taskID string, branchName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"branch_name": branchName,
		"updated_at":  now,
	}

	_, err := d.rest(ctx, "PATCH", "tasks?id=eq."+taskID, body)
	return err
}

func (d *DB) CreateMergeTask(ctx context.Context, taskID, parentTaskID, sliceID, branchName, title string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	body := map[string]interface{}{
		"id":             taskID,
		"title":          "Merge: " + title,
		"type":           "merge",
		"status":         "available",
		"priority":       1,
		"routing_flag":   "internal",
		"slice_id":       sliceID,
		"parent_task_id": parentTaskID,
		"branch_name":    branchName,
		"max_attempts":   3,
		"created_at":     now,
		"updated_at":     now,
	}

	_, err := d.rest(ctx, "POST", "tasks", body)
	return err
}

func (d *DB) GetMergePendingTasks(ctx context.Context, sliceID string) ([]types.Task, error) {
	path := "tasks?type=eq.merge&status=neq.merged"
	if sliceID != "" {
		path += "&slice_id=eq." + sliceID
	}

	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
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

type Runner struct {
	ID              string     `json:"id"`
	ModelID         string     `json:"model_id"`
	ToolID          string     `json:"tool_id"`
	CostPriority    int        `json:"cost_priority"`
	DailyUsed       int        `json:"daily_used"`
	DailyLimit      int        `json:"daily_limit"`
	CooldownExpires *time.Time `json:"cooldown_expires_at"`
	RateLimitReset  *time.Time `json:"rate_limit_reset_at"`
}

type Destination struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Status          string                 `json:"status"`
	Command         string                 `json:"command"`
	Endpoint        string                 `json:"endpoint"`
	APIKeyRef       string                 `json:"api_key_ref"`
	URL             string                 `json:"url"`
	NewChatURL      string                 `json:"new_chat_url"`
	CostCategory    string                 `json:"cost_category"`
	RateLimits      map[string]interface{} `json:"rate_limits"`
	Throttle        map[string]interface{} `json:"throttle"`
	ModelsAvailable []string               `json:"models_available"`
	Config          map[string]interface{} `json:"config"`
}

func (d *DB) GetDestination(ctx context.Context, destID string) (*Destination, error) {
	data, err := d.rpc(ctx, "get_destination", map[string]interface{}{
		"p_id": destID,
	})
	if err != nil {
		return nil, err
	}

	var dest Destination
	if err := json.Unmarshal(data, &dest); err != nil {
		return nil, fmt.Errorf("unmarshal destination: %w", err)
	}
	if dest.ID == "" {
		return nil, fmt.Errorf("destination %s not found or inactive", destID)
	}
	return &dest, nil
}

func (d *DB) GetBestRunner(ctx context.Context, routing string, taskType string) (*Runner, error) {
	data, err := d.rpc(ctx, "get_best_runner", map[string]interface{}{
		"p_routing":   routing,
		"p_task_type": taskType,
	})
	if err != nil {
		return nil, err
	}

	var runners []Runner
	if err := json.Unmarshal(data, &runners); err != nil {
		return nil, fmt.Errorf("unmarshal runner: %w", err)
	}
	if len(runners) == 0 {
		return nil, nil
	}
	return &runners[0], nil
}

func (d *DB) RecordRunnerResult(ctx context.Context, runnerID string, taskType string, success bool, tokens int) error {
	_, err := d.rpc(ctx, "record_runner_result", map[string]interface{}{
		"p_runner_id":   runnerID,
		"p_task_type":   taskType,
		"p_success":     success,
		"p_tokens_used": tokens,
	})
	return err
}

func (d *DB) RefreshLimits(ctx context.Context) error {
	_, err := d.rpc(ctx, "refresh_limits", nil)
	return err
}

func (d *DB) SetRunnerCooldown(ctx context.Context, runnerID string, expiresAt time.Time) error {
	_, err := d.rpc(ctx, "set_runner_cooldown", map[string]interface{}{
		"p_runner_id":  runnerID,
		"p_expires_at": expiresAt.Format(time.RFC3339),
	})
	return err
}

func (d *DB) SetRunnerRateLimited(ctx context.Context, runnerID string, resetAt time.Time) error {
	_, err := d.rpc(ctx, "set_runner_rate_limited", map[string]interface{}{
		"p_runner_id": runnerID,
		"p_reset_at":  resetAt.Format(time.RFC3339),
	})
	return err
}

func (d *DB) GetRunners(ctx context.Context) ([]Runner, error) {
	data, err := d.rest(ctx, "GET", "runners?select=id,model_id,tool_id,cost_priority,status,daily_used,daily_limit&status=eq.active", nil)
	if err != nil {
		return nil, err
	}

	var runners []Runner
	if err := json.Unmarshal(data, &runners); err != nil {
		return nil, fmt.Errorf("unmarshal runners: %w", err)
	}
	return runners, nil
}

func (d *DB) GetTasksByStatus(ctx context.Context, status string, limit int) ([]types.Task, error) {
	path := fmt.Sprintf("tasks?status=eq.%s&order=priority.asc,created_at.asc&limit=%d", status, limit)
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

type ROISummary struct {
	TotalRuns       int     `json:"total_runs"`
	TotalTokensIn   int     `json:"total_tokens_in"`
	TotalTokensOut  int     `json:"total_tokens_out"`
	TheoreticalCost float64 `json:"theoretical_cost"`
	ActualCost      float64 `json:"actual_cost"`
	Savings         float64 `json:"savings"`
}

func (d *DB) GetROISummary(ctx context.Context) (*ROISummary, error) {
	data, err := d.rest(ctx, "GET", "task_runs?select=tokens_in,tokens_out,platform_theoretical_cost_usd,total_actual_cost_usd,total_savings_usd", nil)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		TokensIn                   int     `json:"tokens_in"`
		TokensOut                  int     `json:"tokens_out"`
		PlatformTheoreticalCostUsd float64 `json:"platform_theoretical_cost_usd"`
		TotalActualCostUsd         float64 `json:"total_actual_cost_usd"`
		TotalSavingsUsd            float64 `json:"total_savings_usd"`
	}

	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal roi rows: %w", err)
	}

	summary := &ROISummary{}
	for _, row := range rows {
		summary.TotalRuns++
		summary.TotalTokensIn += row.TokensIn
		summary.TotalTokensOut += row.TokensOut
		summary.TheoreticalCost += row.PlatformTheoreticalCostUsd
		summary.ActualCost += row.TotalActualCostUsd
		summary.Savings += row.TotalSavingsUsd
	}

	return summary, nil
}

type CouncilReviewInput struct {
	PlanID             string
	Round              int
	ModelID            string
	Lens               string
	Vote               string
	Confidence         float64
	Approach           string
	UserIntentCheck    string
	TechDriftCheck     string
	DependenciesCheck  string
	PreventativeIssues []string
	Concerns           []string
	Suggestions        []string
}

type CouncilReview struct {
	ID                 string   `json:"id"`
	PlanID             string   `json:"plan_id"`
	Round              int      `json:"round"`
	ModelID            string   `json:"model_id"`
	Lens               string   `json:"lens"`
	Vote               string   `json:"vote"`
	Confidence         float64  `json:"confidence"`
	Approach           string   `json:"approach"`
	UserIntentCheck    string   `json:"user_intent_check"`
	TechDriftCheck     string   `json:"tech_drift_check"`
	DependenciesCheck  string   `json:"dependencies_check"`
	PreventativeIssues []string `json:"preventative_issues"`
	Concerns           []string `json:"concerns"`
	Suggestions        []string `json:"suggestions"`
}

type CouncilSummary struct {
	PlanID         string          `json:"plan_id"`
	Round          int             `json:"round"`
	Reviews        []CouncilReview `json:"reviews"`
	Consensus      string          `json:"consensus"`
	AllApproved    bool            `json:"all_approved"`
	AnyBlocked     bool            `json:"any_blocked"`
	CommonConcerns []string        `json:"common_concerns"`
	AllSuggestions []string        `json:"all_suggestions"`
}

func (d *DB) SubmitCouncilReview(ctx context.Context, input *CouncilReviewInput) (string, error) {
	data, err := d.rpc(ctx, "submit_council_review", map[string]interface{}{
		"p_plan_id":             input.PlanID,
		"p_round":               input.Round,
		"p_model_id":            input.ModelID,
		"p_lens":                input.Lens,
		"p_vote":                input.Vote,
		"p_confidence":          input.Confidence,
		"p_approach":            input.Approach,
		"p_user_intent_check":   input.UserIntentCheck,
		"p_tech_drift_check":    input.TechDriftCheck,
		"p_dependencies_check":  input.DependenciesCheck,
		"p_preventative_issues": input.PreventativeIssues,
		"p_concerns":            input.Concerns,
		"p_suggestions":         input.Suggestions,
	})
	if err != nil {
		return "", err
	}

	var result string
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal council review id: %w", err)
	}
	return result, nil
}

func (d *DB) GetCouncilSummary(ctx context.Context, planID string, round int) (*CouncilSummary, error) {
	data, err := d.rpc(ctx, "get_council_summary", map[string]interface{}{
		"p_plan_id": planID,
		"p_round":   round,
	})
	if err != nil {
		return nil, err
	}

	var summary CouncilSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("unmarshal council summary: %w", err)
	}
	return &summary, nil
}

func (d *DB) NeedsNextRound(ctx context.Context, planID string, currentRound int) (bool, error) {
	data, err := d.rpc(ctx, "needs_next_round", map[string]interface{}{
		"p_plan_id":       planID,
		"p_current_round": currentRound,
	})
	if err != nil {
		return false, err
	}

	var result bool
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("unmarshal needs_next_round: %w", err)
	}
	return result, nil
}

func (d *DB) GetRoundFeedback(ctx context.Context, planID string, round int) (map[string]interface{}, error) {
	data, err := d.rpc(ctx, "get_round_feedback", map[string]interface{}{
		"p_plan_id": planID,
		"p_round":   round,
	})
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal round feedback: %w", err)
	}
	return result, nil
}

type MaintenanceCommand struct {
	ID             string                 `json:"command_id"`
	CommandType    string                 `json:"command_type"`
	Payload        map[string]interface{} `json:"payload"`
	Status         string                 `json:"status"`
	IdempotencyKey string                 `json:"idempotency_key"`
	ApprovedBy     string                 `json:"approved_by"`
	ExecutedBy     string                 `json:"executed_by"`
	Result         map[string]interface{} `json:"result"`
	ErrorMessage   string                 `json:"error_message"`
	RetryCount     int                    `json:"retry_count"`
}

func (d *DB) CreateMaintenanceCommand(ctx context.Context, commandType, idempotencyKey, approvedBy string, payload map[string]interface{}) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"command_type":    commandType,
		"payload":         payload,
		"idempotency_key": idempotencyKey,
		"approved_by":     approvedBy,
		"created_at":      now,
		"updated_at":      now,
	}

	data, err := d.rest(ctx, "POST", "maintenance_commands", body)
	if err != nil {
		return "", err
	}

	var result []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal command result: %w", err)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("no command ID returned")
	}
	return result[0].ID, nil
}

func (d *DB) ClaimNextCommand(ctx context.Context, agentID string) (*MaintenanceCommand, error) {
	data, err := d.rpc(ctx, "claim_next_command", map[string]interface{}{
		"p_agent_id": agentID,
	})
	if err != nil {
		return nil, err
	}

	var commands []MaintenanceCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil, fmt.Errorf("unmarshal command: %w", err)
	}
	if len(commands) == 0 {
		return nil, nil
	}
	return &commands[0], nil
}

func (d *DB) CompleteCommand(ctx context.Context, commandID string, success bool, result map[string]interface{}, errorMessage string) error {
	_, err := d.rpc(ctx, "complete_command", map[string]interface{}{
		"p_command_id":    commandID,
		"p_success":       success,
		"p_result":        result,
		"p_error_message": errorMessage,
	})
	return err
}

func (d *DB) GetAvailableModels(ctx context.Context, limit int) ([]Runner, error) {
	path := fmt.Sprintf("runners?status=eq.active&order=cost_priority.asc&limit=%d", limit)
	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var runners []Runner
	if err := json.Unmarshal(data, &runners); err != nil {
		return nil, fmt.Errorf("unmarshal runners: %w", err)
	}
	return runners, nil
}

func (d *DB) GetTaskRuns(ctx context.Context, taskID string) ([]types.TaskRun, error) {
	path := fmt.Sprintf("task_runs?task_id=eq.%s&order=created_at.desc", taskID)
	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var runs []types.TaskRun
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, fmt.Errorf("unmarshal task runs: %w", err)
	}
	return runs, nil
}

func (d *DB) CreateResearchSuggestion(ctx context.Context, payload map[string]interface{}) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	body := map[string]interface{}{
		"task_id":     payload["task_id"],
		"category":    payload["category"],
		"root_cause":  payload["root_cause"],
		"suggestions": payload["suggestions"],
		"auto_retry":  payload["auto_retry"],
		"new_model":   payload["new_model"],
		"analyzed_at": payload["analyzed_at"],
		"created_at":  now,
	}

	data, err := d.rest(ctx, "POST", "research_suggestions?select=id", body)
	if err != nil {
		return "", err
	}

	var result []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal research suggestion: %w", err)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("no ID returned from research suggestion creation")
	}
	return result[0].ID, nil
}

func (d *DB) GetResearchSuggestion(ctx context.Context, taskID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("research_suggestions?task_id=eq.%s&order=created_at.desc&limit=1", taskID)
	data, err := d.rest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("unmarshal research suggestion: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no research suggestion found for task %s", taskID)
	}
	return results[0], nil
}

func (d *DB) REST(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	return d.rest(ctx, method, path, body)
}

func (d *DB) RPC(ctx context.Context, name string, params interface{}) ([]byte, error) {
	return d.rpc(ctx, name, params)
}

func (d *DB) LogOrchestratorEvent(ctx context.Context, eventType, taskID, runnerID, fromRunnerID, toRunnerID, modelID, reason string, details map[string]interface{}) error {
	params := map[string]interface{}{
		"p_event_type":     eventType,
		"p_task_id":        nil,
		"p_runner_id":      nil,
		"p_from_runner_id": nil,
		"p_to_runner_id":   nil,
		"p_model_id":       nil,
		"p_reason":         nil,
		"p_details":        nil,
	}
	if taskID != "" {
		params["p_task_id"] = taskID
	}
	if runnerID != "" {
		params["p_runner_id"] = runnerID
	}
	if fromRunnerID != "" {
		params["p_from_runner_id"] = fromRunnerID
	}
	if toRunnerID != "" {
		params["p_to_runner_id"] = toRunnerID
	}
	if modelID != "" {
		params["p_model_id"] = modelID
	}
	if reason != "" {
		params["p_reason"] = reason
	}
	if details != nil {
		params["p_details"] = details
	}
	_, err := d.rpc(ctx, "log_orchestrator_event", params)
	return err
}

func (d *DB) AppendRoutingHistory(ctx context.Context, taskID, fromModel, toModel, reason string) error {
	_, err := d.rpc(ctx, "append_routing_history", map[string]interface{}{
		"p_task_id":    taskID,
		"p_from_model": fromModel,
		"p_to_model":   toModel,
		"p_reason":     reason,
	})
	return err
}

func (d *DB) IncrementInFlight(ctx context.Context, runnerID string) (bool, error) {
	data, err := d.rpc(ctx, "increment_in_flight", map[string]interface{}{
		"p_runner_id": runnerID,
	})
	if err != nil {
		return false, err
	}

	var result bool
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("unmarshal increment_in_flight: %w", err)
	}
	return result, nil
}

func (d *DB) DecrementInFlight(ctx context.Context, runnerID string) error {
	_, err := d.rpc(ctx, "decrement_in_flight", map[string]interface{}{
		"p_runner_id": runnerID,
	})
	return err
}

func (d *DB) GetSystemState(ctx context.Context) (map[string]interface{}, error) {
	data, err := d.rpc(ctx, "get_system_state", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal system state: %w", err)
	}
	return result, nil
}

type FailureRecord struct {
	TaskID          string                 `json:"task_id,omitempty"`
	TaskRunID       string                 `json:"task_run_id,omitempty"`
	FailureType     string                 `json:"failure_type"`
	FailureCategory string                 `json:"failure_category"`
	Details         map[string]interface{} `json:"failure_details,omitempty"`
	ModelID         string                 `json:"model_id,omitempty"`
	Platform        string                 `json:"platform,omitempty"`
	RunnerID        string                 `json:"runner_id,omitempty"`
	TaskType        string                 `json:"task_type,omitempty"`
	TokensUsed      int                    `json:"tokens_used,omitempty"`
	DurationSec     int                    `json:"duration_sec,omitempty"`
}

func (d *DB) RecordFailure(ctx context.Context, record *FailureRecord) (string, error) {
	params := map[string]interface{}{
		"p_failure_type":     record.FailureType,
		"p_failure_category": record.FailureCategory,
	}
	if record.TaskID != "" {
		params["p_task_id"] = record.TaskID
	}
	if record.TaskRunID != "" {
		params["p_task_run_id"] = record.TaskRunID
	}
	if record.Details != nil {
		params["p_failure_details"] = record.Details
	}
	if record.ModelID != "" {
		params["p_model_id"] = record.ModelID
	}
	if record.Platform != "" {
		params["p_platform"] = record.Platform
	}
	if record.RunnerID != "" {
		params["p_runner_id"] = record.RunnerID
	}
	if record.TaskType != "" {
		params["p_task_type"] = record.TaskType
	}
	if record.TokensUsed > 0 {
		params["p_tokens_used"] = record.TokensUsed
	}
	if record.DurationSec > 0 {
		params["p_duration_sec"] = record.DurationSec
	}

	data, err := d.rpc(ctx, "record_failure", params)
	if err != nil {
		return "", err
	}

	var id string
	if err := json.Unmarshal(data, &id); err != nil {
		return "", fmt.Errorf("unmarshal failure id: %w", err)
	}
	return id, nil
}

type Heuristic struct {
	ID             string                 `json:"id"`
	TaskType       string                 `json:"task_type,omitempty"`
	Condition      map[string]interface{} `json:"condition,omitempty"`
	PreferredModel string                 `json:"preferred_model"`
	Action         map[string]interface{} `json:"action,omitempty"`
	Confidence     float64                `json:"confidence"`
}

func (d *DB) GetHeuristic(ctx context.Context, taskType string, condition map[string]interface{}) (*Heuristic, error) {
	params := map[string]interface{}{}
	if taskType != "" {
		params["p_task_type"] = taskType
	}
	if condition != nil {
		params["p_condition"] = condition
	}

	data, err := d.rpc(ctx, "get_heuristic", params)
	if err != nil {
		return nil, err
	}

	var heuristics []Heuristic
	if err := json.Unmarshal(data, &heuristics); err != nil {
		return nil, fmt.Errorf("unmarshal heuristic: %w", err)
	}
	if len(heuristics) == 0 {
		return nil, nil
	}
	return &heuristics[0], nil
}

func (d *DB) RecordHeuristicResult(ctx context.Context, heuristicID string, success bool) error {
	_, err := d.rpc(ctx, "record_heuristic_result", map[string]interface{}{
		"p_heuristic_id": heuristicID,
		"p_success":      success,
	})
	return err
}

type ProblemSolution struct {
	ID              string                 `json:"id"`
	SolutionType    string                 `json:"solution_type"`
	SolutionModel   string                 `json:"solution_model,omitempty"`
	SolutionDetails map[string]interface{} `json:"solution_details,omitempty"`
	SuccessRate     float64                `json:"success_rate"`
}

func (d *DB) GetProblemSolution(ctx context.Context, failureType, taskType string, keywords []string) (*ProblemSolution, error) {
	params := map[string]interface{}{}
	if failureType != "" {
		params["p_failure_type"] = failureType
	}
	if taskType != "" {
		params["p_task_type"] = taskType
	}
	if keywords != nil {
		params["p_keywords"] = keywords
	}

	data, err := d.rpc(ctx, "get_problem_solution", params)
	if err != nil {
		return nil, err
	}

	var solutions []ProblemSolution
	if err := json.Unmarshal(data, &solutions); err != nil {
		return nil, fmt.Errorf("unmarshal problem solution: %w", err)
	}
	if len(solutions) == 0 {
		return nil, nil
	}
	return &solutions[0], nil
}

func (d *DB) RecordSolutionResult(ctx context.Context, solutionID string, success bool) error {
	_, err := d.rpc(ctx, "record_solution_result", map[string]interface{}{
		"p_solution_id": solutionID,
		"p_success":     success,
	})
	return err
}

type RecentFailure struct {
	ModelID      string `json:"model_id"`
	Platform     string `json:"platform,omitempty"`
	FailureType  string `json:"failure_type"`
	FailureCount int    `json:"failure_count"`
}

func (d *DB) GetRecentFailures(ctx context.Context, taskType string, sinceMinutes int) ([]RecentFailure, error) {
	params := map[string]interface{}{}
	if taskType != "" {
		params["p_task_type"] = taskType
	}
	if sinceMinutes > 0 {
		params["p_since"] = fmt.Sprintf("NOW() - INTERVAL '%d minutes'", sinceMinutes)
	}

	data, err := d.rpc(ctx, "get_recent_failures", params)
	if err != nil {
		return nil, err
	}

	var failures []RecentFailure
	if err := json.Unmarshal(data, &failures); err != nil {
		return nil, fmt.Errorf("unmarshal recent failures: %w", err)
	}
	return failures, nil
}

func (d *DB) UpsertHeuristic(ctx context.Context, taskType string, condition map[string]interface{}, preferredModel string, action map[string]interface{}, confidence float64, source string) (string, error) {
	params := map[string]interface{}{
		"p_preferred_model": preferredModel,
		"p_confidence":      confidence,
	}
	if taskType != "" {
		params["p_task_type"] = taskType
	}
	if condition != nil {
		params["p_condition"] = condition
	}
	if action != nil {
		params["p_action"] = action
	}
	if source != "" {
		params["p_source"] = source
	}

	data, err := d.rpc(ctx, "upsert_heuristic", params)
	if err != nil {
		return "", err
	}

	var id string
	if err := json.Unmarshal(data, &id); err != nil {
		return "", fmt.Errorf("unmarshal heuristic id: %w", err)
	}
	return id, nil
}

type RunnerToArchive struct {
	ID                string     `json:"id"`
	ModelID           string     `json:"model_id"`
	DepreciationScore float64    `json:"depreciation_score"`
	TotalAttempts     int        `json:"total_attempts"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
}

func (d *DB) GetRunnersToArchive(ctx context.Context, threshold float64, minAttempts, cooldownHours int) ([]RunnerToArchive, error) {
	data, err := d.rpc(ctx, "get_runners_to_archive", map[string]interface{}{
		"p_threshold":      threshold,
		"p_min_attempts":   minAttempts,
		"p_cooldown_hours": cooldownHours,
	})
	if err != nil {
		return nil, err
	}

	var runners []RunnerToArchive
	if err := json.Unmarshal(data, &runners); err != nil {
		return nil, fmt.Errorf("unmarshal runners to archive: %w", err)
	}
	return runners, nil
}

func (d *DB) ArchiveRunner(ctx context.Context, runnerID, reason string) error {
	_, err := d.rpc(ctx, "archive_runner", map[string]interface{}{
		"p_runner_id": runnerID,
		"p_reason":    reason,
	})
	return err
}

func (d *DB) BoostRunner(ctx context.Context, runnerID string) error {
	_, err := d.rpc(ctx, "boost_runner", map[string]interface{}{
		"p_runner_id": runnerID,
	})
	return err
}

func (d *DB) ReviveRunner(ctx context.Context, runnerID, reason string) error {
	_, err := d.rpc(ctx, "revive_runner", map[string]interface{}{
		"p_runner_id": runnerID,
		"p_reason":    reason,
	})
	return err
}

type PlannerRule struct {
	ID                  string                 `json:"id"`
	AppliesTo           string                 `json:"applies_to"`
	RuleType            string                 `json:"rule_type"`
	RuleText            string                 `json:"rule_text"`
	Details             map[string]interface{} `json:"details"`
	Source              string                 `json:"source"`
	Priority            int                    `json:"priority"`
	TimesApplied        int                    `json:"times_applied"`
	TimesPreventedIssue int                    `json:"times_prevented_issue"`
	EffectivenessScore  float64                `json:"effectiveness_score"`
}

func (d *DB) GetPlannerRules(ctx context.Context, appliesTo string, limit int) ([]PlannerRule, error) {
	params := map[string]interface{}{
		"p_limit": limit,
	}
	if appliesTo != "" {
		params["p_applies_to"] = appliesTo
	}

	data, err := d.rpc(ctx, "get_planner_rules", params)
	if err != nil {
		return nil, err
	}

	var rules []PlannerRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("unmarshal planner rules: %w", err)
	}
	return rules, nil
}

func (d *DB) CreatePlannerRule(ctx context.Context, appliesTo, ruleType, ruleText, source string, details map[string]interface{}, sourceTaskID, sourceReviewType *string, priority int) (string, error) {
	params := map[string]interface{}{
		"p_applies_to": appliesTo,
		"p_rule_type":  ruleType,
		"p_rule_text":  ruleText,
		"p_source":     source,
		"p_details":    details,
		"p_priority":   priority,
	}
	if sourceTaskID != nil {
		params["p_source_task_id"] = *sourceTaskID
	}
	if sourceReviewType != nil {
		params["p_source_review_type"] = *sourceReviewType
	}

	data, err := d.rpc(ctx, "create_planner_rule", params)
	if err != nil {
		return "", err
	}

	var ruleID string
	if err := json.Unmarshal(data, &ruleID); err != nil {
		return "", fmt.Errorf("unmarshal rule id: %w", err)
	}
	return ruleID, nil
}

func (d *DB) RecordPlannerRuleApplied(ctx context.Context, ruleID string) error {
	_, err := d.rpc(ctx, "record_planner_rule_applied", map[string]interface{}{
		"p_rule_id": ruleID,
	})
	return err
}

func (d *DB) RecordPlannerRulePreventedIssue(ctx context.Context, ruleID string) error {
	_, err := d.rpc(ctx, "record_planner_rule_prevented_issue", map[string]interface{}{
		"p_rule_id": ruleID,
	})
	return err
}

func (d *DB) DeactivatePlannerRule(ctx context.Context, ruleID, reason string) error {
	_, err := d.rpc(ctx, "deactivate_planner_rule", map[string]interface{}{
		"p_rule_id": ruleID,
		"p_reason":  reason,
	})
	return err
}

func (d *DB) CreateRuleFromRejection(ctx context.Context, taskID, rejectionType, rejectionReason, appliesTo, source string) (string, error) {
	data, err := d.rpc(ctx, "create_rule_from_rejection", map[string]interface{}{
		"p_task_id":          taskID,
		"p_rejection_type":   rejectionType,
		"p_rejection_reason": rejectionReason,
		"p_applies_to":       appliesTo,
		"p_source":           source,
	})
	if err != nil {
		return "", err
	}

	var ruleID string
	if err := json.Unmarshal(data, &ruleID); err != nil {
		return "", fmt.Errorf("unmarshal rule id: %w", err)
	}
	return ruleID, nil
}
