package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/courier"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/pool"
	"github.com/vibepilot/governor/internal/security"
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
	maintenance  MaintenanceExecutor
}

type MaintenanceExecutor interface {
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

func (d *Dispatcher) SetMaintenance(m MaintenanceExecutor) {
	d.maintenance = m
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

	if task.Type == "merge" {
		d.executeMerge(ctx, task)
		return
	}

	if task.RoutingFlag == types.RoutingWeb && d.courier != nil && d.cfg.Courier.Enabled {
		d.dispatchToCourier(ctx, task)
		return
	}

	runner, err := d.pool.SelectBest(ctx, string(task.RoutingFlag), task.Type)
	if err != nil {
		log.Printf("Dispatcher: pool error for %s: %v", task.ID[:8], err)
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}
	if runner == nil {
		log.Printf("Dispatcher: no runner available for %s (routing=%s)", task.ID[:8], task.RoutingFlag)
		d.handleFailure(ctx, task)
		d.finalize(task.ID, task.SliceID)
		return
	}

	log.Printf("Dispatcher: selected runner %s (model=%s, priority=%d)", runner.ID[:8], runner.ModelID, runner.CostPriority)

	if err := d.db.ClaimTask(ctx, task.ID, runner.ModelID); err != nil {
		log.Printf("Dispatcher: claim failed for %s: %v", task.ID[:8], err)
		d.finalize(task.ID, task.SliceID)
		return
	}

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

func (d *Dispatcher) runTool(ctx context.Context, toolID string, prompt string, timeoutSec int) (output string, tokensIn, tokensOut int, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmdName := d.resolveToolCommand(toolID)
	cmd := exec.CommandContext(ctx, cmdName, "run", "--format", "json", prompt)
	raw, execErr := cmd.CombinedOutput()

	if execErr != nil {
		return "", 0, 0, fmt.Errorf("%s: %w\noutput: %s", cmdName, execErr, string(raw))
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

func (d *Dispatcher) resolveToolCommand(toolID string) string {
	switch toolID {
	case "opencode":
		return "opencode"
	case "kimi-cli":
		return "kimi"
	default:
		return "opencode"
	}
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

	d.courier.Enqueue(task)
}

func (d *Dispatcher) OnCourierResult(result courier.Result) {
	ctx := context.Background()

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

func (d *Dispatcher) executeMerge(ctx context.Context, task types.Task) {
	log.Printf("Dispatcher: executing merge task %s for parent %s", task.ID[:8], task.ParentTaskID[:8])

	d.db.UpdateTaskStatus(ctx, task.ID, types.StatusInProgress, nil)

	parentTask, err := d.db.GetTaskByID(ctx, task.ParentTaskID)
	if err != nil || parentTask == nil {
		log.Printf("Dispatcher: parent task %s not found for merge", task.ParentTaskID[:8])
		d.handleMergeFailure(ctx, task, "Parent task not found")
		return
	}

	if parentTask.BranchName == "" {
		log.Printf("Dispatcher: parent task %s has no branch", task.ParentTaskID[:8])
		d.handleMergeFailure(ctx, task, "Parent task has no branch")
		return
	}

	if d.maintenance == nil {
		log.Printf("Dispatcher: maintenance not configured")
		d.handleMergeFailure(ctx, task, "Maintenance not configured")
		return
	}

	if err := d.maintenance.ExecuteMerge(ctx, task.ParentTaskID, parentTask.BranchName); err != nil {
		log.Printf("Dispatcher: merge failed for %s: %v", task.ID[:8], err)
		d.handleMergeFailure(ctx, task, err.Error())
		return
	}

	d.db.UpdateTaskStatus(ctx, task.ID, types.StatusMerged, nil)
	d.db.UpdateTaskStatus(ctx, task.ParentTaskID, types.StatusMerged, map[string]interface{}{
		"merged_to": "main",
	})
	d.db.UnlockDependents(ctx, task.ParentTaskID)

	log.Printf("Dispatcher: merge task %s completed, parent %s merged", task.ID[:8], task.ParentTaskID[:8])
}

func (d *Dispatcher) handleMergeFailure(ctx context.Context, task types.Task, errMsg string) {
	attempts := task.Attempts + 1

	if attempts >= task.MaxAttempts {
		log.Printf("Dispatcher: merge task %s escalated after %d attempts", task.ID[:8], attempts)
		d.db.UpdateTaskStatus(ctx, task.ID, types.StatusEscalated, map[string]interface{}{
			"attempts":      attempts,
			"failure_notes": errMsg,
		})
	} else {
		d.db.ResetTask(ctx, task.ID, false)
		d.db.UpdateTaskStatus(ctx, task.ID, types.StatusAvailable, map[string]interface{}{
			"attempts":      attempts,
			"failure_notes": errMsg,
		})
		log.Printf("Dispatcher: merge task %s returned to queue (attempt %d/%d)", task.ID[:8], attempts, task.MaxAttempts)
	}
}
