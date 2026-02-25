package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/courier"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/pool"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/internal/vault"
	"github.com/vibepilot/governor/pkg/types"
)

type TaskCompleter interface {
	OnTaskComplete(ctx context.Context, taskID string, result interface{})
}

type TaskFinalizer interface {
	Complete(taskID string, sliceID string)
}

type Dispatcher struct {
	db           *db.DB
	cfg          *config.Config
	pool         *pool.Pool
	leakDetector *security.LeakDetector
	courier      *courier.Dispatcher
	completer    TaskCompleter
	finalizer    TaskFinalizer
	gitree       Gitree
	vault        *vault.Vault
}

type Gitree interface {
	CreateBranch(ctx context.Context, branchName string) error
	ClearBranch(ctx context.Context, branchName string, baseBranch string) error
}

type GitreeExecutor interface {
	ExecuteMerge(ctx context.Context, taskID, branchName string) error
}

func New(database *db.DB, cfg *config.Config, leakDetector *security.LeakDetector) *Dispatcher {
	d := &Dispatcher{
		db:           database,
		cfg:          cfg,
		pool:         pool.New(database),
		leakDetector: leakDetector,
	}
	return d
}

func (d *Dispatcher) SetCourier(c *courier.Dispatcher) {
	d.courier = c
}

func (d *Dispatcher) SetOrchestrator(completer TaskCompleter) {
	d.completer = completer
}

func (d *Dispatcher) SetFinalizer(f TaskFinalizer) {
	d.finalizer = f
}

func (d *Dispatcher) SetGitree(g Gitree) {
	d.gitree = g
}

func (d *Dispatcher) SetVault(v *vault.Vault) {
	d.vault = v
}

func (d *Dispatcher) ensureTaskBranch(ctx context.Context, task *types.Task) string {
	if task.BranchName != "" {
		return task.BranchName
	}

	branchName := d.generateBranchName(task)

	if d.gitree != nil {
		if err := d.gitree.CreateBranch(ctx, branchName); err != nil {
			log.Printf("Dispatcher: failed to create branch for %s: %v", task.ID[:8], err)
			d.db.LogOrchestratorEvent(ctx, "branch_failed", task.ID, "", "", "", "", "Branch creation failed", map[string]interface{}{"error": err.Error()})
			return ""
		}
	}

	if err := d.db.UpdateTaskBranch(ctx, task.ID, branchName); err != nil {
		log.Printf("Dispatcher: failed to update task branch for %s: %v", task.ID[:8], err)
	}

	task.BranchName = branchName
	d.db.LogOrchestratorEvent(ctx, "branch_created", task.ID, "", "", "", "", "Branch created", map[string]interface{}{"branch": branchName})

	return branchName
}

func (d *Dispatcher) generateBranchName(task *types.Task) string {
	taskNum := task.TaskNumber
	if taskNum == "" {
		taskNum = task.ID[:8]
	}
	return fmt.Sprintf("task/%s", taskNum)
}

func (d *Dispatcher) Run(ctx context.Context, dispatchCh <-chan types.Task) {
	log.Println("Dispatcher started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Dispatcher shutting down")
			return
		case task := <-dispatchCh:
			go d.execute(ctx, task)
		}
	}
}

func (d *Dispatcher) execute(ctx context.Context, task types.Task) {
	log.Printf("Dispatcher: task %s (routing=%s, type=%s, slice=%s)", task.ID[:8], task.RoutingFlag, task.Type, task.SliceID)

	d.db.LogOrchestratorEvent(ctx, "task_dispatched", task.ID, "", "", "", string(task.RoutingFlag), "", map[string]interface{}{
		"type":     task.Type,
		"slice_id": task.SliceID,
		"routing":  string(task.RoutingFlag),
	})

	if task.RoutingFlag == types.RoutingWeb && d.courier != nil && d.cfg.Courier.Enabled {
		d.db.LogOrchestratorEvent(ctx, "routed_to_courier", task.ID, "", "", "", string(task.RoutingFlag), "Routing to web courier", nil)
		d.dispatchToCourier(ctx, task)
		return
	}

	runner, err := d.pool.SelectBest(ctx, string(task.RoutingFlag), task.Type)
	if err != nil {
		log.Printf("Dispatcher: pool error for %s: %v", task.ID[:8], err)
		d.db.LogOrchestratorEvent(ctx, "pool_error", task.ID, "", "", "", "", err.Error(), nil)
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}
	if runner == nil {
		log.Printf("Dispatcher: no runner available for %s (routing=%s)", task.ID[:8], task.RoutingFlag)
		d.db.LogOrchestratorEvent(ctx, "no_runner", task.ID, "", "", "", string(task.RoutingFlag), "No runner available", nil)
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}

	log.Printf("Dispatcher: selected runner %s (model=%s, priority=%d)", runner.ID[:8], runner.ModelID, runner.CostPriority)
	d.db.LogOrchestratorEvent(ctx, "runner_selected", task.ID, runner.ID, "", "", runner.ModelID, "", map[string]interface{}{
		"priority": runner.CostPriority,
	})

	canRun, err := d.db.IncrementInFlight(ctx, runner.ID)
	if err != nil {
		log.Printf("Dispatcher: in-flight check failed for %s: %v", task.ID[:8], err)
	}
	if !canRun {
		log.Printf("Dispatcher: runner %s at max concurrent capacity, selecting alternative", runner.ID[:8])
		altRunner, altErr := d.pool.SelectBest(ctx, string(task.RoutingFlag), task.Type)
		if altErr != nil || altRunner == nil || altRunner.ID == runner.ID {
			d.handleFailure(ctx, task)
			d.finalize(task.ID, task.SliceID)
			return
		}
		runner = altRunner
		canRun, _ = d.db.IncrementInFlight(ctx, runner.ID)
		if !canRun {
			d.handleFailure(ctx, task)
			d.finalize(task.ID, task.SliceID)
			return
		}
	}

	defer func() {
		if err := d.db.DecrementInFlight(ctx, runner.ID); err != nil {
			log.Printf("Dispatcher: failed to decrement in-flight for %s: %v", runner.ID[:8], err)
		}
	}()

	if err := d.db.ClaimTask(ctx, task.ID, runner.ModelID); err != nil {
		log.Printf("Dispatcher: claim failed for %s: %v", task.ID[:8], err)
		d.finalize(task.ID, task.SliceID)
		return
	}

	d.ensureTaskBranch(ctx, &task)

	packet, err := d.db.GetTaskPacket(ctx, task.ID)
	if err != nil || packet == nil {
		log.Printf("Dispatcher: no packet for %s: %v", task.ID[:8], err)
		d.handleFailure(ctx, task)
		d.recordResult(ctx, runner.ID, task.Type, false, 0)
		d.finalize(task.ID, task.SliceID)
		return
	}

	taskTimeout := d.cfg.Governor.TaskTimeoutSec
	if taskTimeout <= 0 {
		taskTimeout = 300
	}

	output, tokensIn, tokensOut, execErr := d.runTool(ctx, runner.ToolID, packet.Prompt, taskTimeout)

	success := execErr == nil
	status := "success"
	var result interface{}
	if !success {
		status = "failed"
		result = map[string]interface{}{"error": execErr.Error()}
	} else {
		result = map[string]interface{}{"output": output}
	}

	if success && strings.TrimSpace(output) == "" {
		success = false
		status = "failed"
		result = map[string]interface{}{"error": "Empty output - task produced no response"}
		log.Printf("Dispatcher: task %s produced empty output", task.ID[:8])
	}

	runID, err := d.db.RecordTaskRun(ctx, &db.TaskRunInput{
		TaskID:    task.ID,
		ModelID:   runner.ModelID,
		Courier:   "governor",
		Platform:  string(task.RoutingFlag),
		Status:    status,
		Result:    result,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
	})
	if err != nil {
		log.Printf("Dispatcher: failed to record run for %s: %v", task.ID[:8], err)
	} else {
		if err := d.db.CallROIRPC(ctx, runID); err != nil {
			log.Printf("Dispatcher: ROI RPC failed for run %s: %v", runID[:8], err)
		}
	}

	d.recordResult(ctx, runner.ID, task.Type, success, tokensIn+tokensOut)

	if !success {
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}

	if d.pool.ShouldThrottle(runner) {
		d.pool.SetCooldown(ctx, runner.ID, d.timeUntilMidnight())
		log.Printf("Dispatcher: runner %s at 80%% daily, cooling down", runner.ID[:8])
	}

	if d.completer != nil {
		d.completer.OnTaskComplete(ctx, task.ID, result)
	} else {
		if err := d.db.UpdateTaskStatus(ctx, task.ID, types.StatusReview, result); err != nil {
			log.Printf("Dispatcher: failed to update status for %s: %v", task.ID[:8], err)
		}
	}

	d.finalize(task.ID, task.SliceID)
	log.Printf("Dispatcher: task %s completed successfully", task.ID[:8])
}

func (d *Dispatcher) finalize(taskID string, sliceID string) {
	if d.finalizer != nil && sliceID != "" {
		d.finalizer.Complete(taskID, sliceID)
	}
}

func (d *Dispatcher) recordResult(ctx context.Context, runnerID string, taskType string, success bool, tokens int) {
	if err := d.pool.RecordResult(ctx, runnerID, taskType, success, tokens); err != nil {
		log.Printf("Dispatcher: failed to record runner result: %v", err)
	}
}

func (d *Dispatcher) runTool(ctx context.Context, destID string, prompt string, timeoutSec int) (output string, tokensIn, tokensOut int, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	dest, err := d.db.GetDestination(ctx, destID)
	if err != nil {
		return "", 0, 0, fmt.Errorf("get destination %s: %w", destID, err)
	}

	switch dest.Type {
	case "cli":
		return d.executeCLI(ctx, dest.Command, prompt)
	case "api", "api_free", "api_credit":
		return d.executeAPI(ctx, dest, prompt)
	case "web":
		return "", 0, 0, fmt.Errorf("web destinations use courier, not direct execution")
	default:
		return "", 0, 0, fmt.Errorf("unknown destination type: %s", dest.Type)
	}
}

func (d *Dispatcher) executeCLI(ctx context.Context, command string, prompt string) (output string, tokensIn, tokensOut int, err error) {
	cmd := exec.CommandContext(ctx, command, "run", "--format", "json", prompt)
	raw, execErr := cmd.CombinedOutput()

	if execErr != nil {
		return "", 0, 0, fmt.Errorf("%s: %w\noutput: %s", command, execErr, string(raw))
	}

	clean, warnings := d.leakDetector.Scan(string(raw))
	if len(warnings) > 0 {
		log.Printf("Dispatcher: leak warnings: %+v", warnings)
	}

	var result struct {
		Content      string `json:"content"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		output = clean
		tokensIn = len(prompt) / 4
		tokensOut = len(output) / 4
	} else {
		output = result.Content
		tokensIn = result.InputTokens
		tokensOut = result.OutputTokens
	}

	return output, tokensIn, tokensOut, nil
}

func (d *Dispatcher) executeAPI(ctx context.Context, dest *db.Destination, prompt string) (output string, tokensIn, tokensOut int, err error) {
	apiKey := ""
	if dest.APIKeyRef != "" {
		apiKey = d.getAPIKey(dest.APIKeyRef)
	}

	var reqBody interface{}
	var endpoint string
	var headers map[string]string

	if strings.Contains(dest.Endpoint, "generativelanguage.googleapis.com") {
		endpoint = dest.Endpoint + "/models/gemini-2.0-flash:generateContent?key=" + apiKey
		reqBody = map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": prompt}}},
			},
		}
		headers = map[string]string{
			"Content-Type": "application/json",
		}
	} else if strings.Contains(dest.Endpoint, "deepseek.com") {
		endpoint = dest.Endpoint + "/chat/completions"
		reqBody = map[string]interface{}{
			"model": "deepseek-chat",
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		}
		headers = map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + apiKey,
		}
	} else {
		endpoint = dest.Endpoint + "/chat/completions"
		reqBody = map[string]interface{}{
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		}
		headers = map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + apiKey,
		}
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", 0, 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	output, tokensIn, tokensOut = d.parseAPIResponse(dest.Endpoint, respBody)

	clean, warnings := d.leakDetector.Scan(output)
	if len(warnings) > 0 {
		log.Printf("Dispatcher: leak warnings: %+v", warnings)
	}

	return clean, tokensIn, tokensOut, nil
}

func (d *Dispatcher) getAPIKey(keyRef string) string {
	if d.vault != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		key, err := d.vault.GetSecret(ctx, keyRef)
		if err == nil {
			return key
		}
		log.Printf("Dispatcher: failed to get API key %s from vault: %v", keyRef, err)
	}
	return ""
}

func (d *Dispatcher) parseAPIResponse(endpoint string, body []byte) (output string, tokensIn, tokensOut int) {
	if strings.Contains(endpoint, "generativelanguage.googleapis.com") {
		var gemini struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(body, &gemini); err == nil && len(gemini.Candidates) > 0 && len(gemini.Candidates[0].Content.Parts) > 0 {
			return gemini.Candidates[0].Content.Parts[0].Text,
				gemini.UsageMetadata.PromptTokenCount,
				gemini.UsageMetadata.CandidatesTokenCount
		}
	}

	var generic struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &generic); err == nil && len(generic.Choices) > 0 {
		return generic.Choices[0].Message.Content, generic.Usage.PromptTokens, generic.Usage.CompletionTokens
	}

	return string(body), 0, 0
}

func (d *Dispatcher) timeUntilMidnight() time.Duration {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return midnight.Sub(now)
}

func (d *Dispatcher) handleFailure(ctx context.Context, task types.Task) {
	taskPtr, err := d.db.GetTaskByID(ctx, task.ID)
	if err != nil {
		log.Printf("Dispatcher: failed to get task %s for failure handling: %v", task.ID[:8], err)
		return
	}

	escalate := taskPtr.Attempts+1 >= taskPtr.MaxAttempts

	if err := d.db.ResetTask(ctx, task.ID, escalate); err != nil {
		log.Printf("Dispatcher: failed to reset task %s: %v", task.ID[:8], err)
		return
	}

	if escalate {
		log.Printf("Dispatcher: task %s escalated (max attempts)", task.ID[:8])
	} else {
		log.Printf("Dispatcher: task %s returned to queue", task.ID[:8])
	}
}

func (d *Dispatcher) dispatchToCourier(ctx context.Context, task types.Task) {
	if d.courier == nil {
		log.Printf("Dispatcher: courier not configured for %s", task.ID[:8])
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}

	if err := d.db.ClaimTask(ctx, task.ID, "courier"); err != nil {
		log.Printf("Dispatcher: courier claim failed for %s: %v", task.ID[:8], err)
		d.finalize(task.ID, task.SliceID)
		return
	}

	d.ensureTaskBranch(ctx, &task)

	d.courier.Enqueue(task)
}

func (d *Dispatcher) OnCourierResult(result courier.Result) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	success := result.Status == "success"
	status := "success"
	var taskResult interface{}
	if !success {
		status = "failed"
		taskResult = map[string]interface{}{"error": result.Error, "output": result.Output}
	} else {
		taskResult = map[string]interface{}{"output": result.Output, "chat_url": result.ChatURL}
	}

	runID, err := d.db.RecordTaskRun(ctx, &db.TaskRunInput{
		TaskID:    result.TaskID,
		ModelID:   "courier",
		Courier:   "browser-use",
		Platform:  "web",
		Status:    status,
		Result:    taskResult,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	})
	if err != nil {
		log.Printf("Dispatcher: failed to record courier run for %s: %v", result.TaskID[:8], err)
	} else {
		if err := d.db.CallROIRPC(ctx, runID); err != nil {
			log.Printf("Dispatcher: ROI RPC failed for courier run %s: %v", runID[:8], err)
		}
	}

	if !success {
		task, _ := d.db.GetTaskByID(ctx, result.TaskID)
		if task != nil {
			d.handleFailure(ctx, *task)
			d.finalize(task.ID, task.SliceID)
		}
		return
	}

	task, err := d.db.GetTaskByID(ctx, result.TaskID)
	if err != nil || task == nil {
		log.Printf("Dispatcher: courier task %s not found for completion", result.TaskID[:8])
		return
	}

	if d.completer != nil {
		d.completer.OnTaskComplete(ctx, result.TaskID, taskResult)
	} else {
		if err := d.db.UpdateTaskStatus(ctx, result.TaskID, types.StatusReview, taskResult); err != nil {
			log.Printf("Dispatcher: failed to update courier task status %s: %v", result.TaskID[:8], err)
		}
	}

	d.finalize(task.ID, task.SliceID)
	log.Printf("Dispatcher: courier task %s completed successfully", result.TaskID[:8])
}
