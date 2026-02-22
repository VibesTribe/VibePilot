package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/pkg/types"
)

type Dispatcher struct {
	db           *db.DB
	cfg          *config.Config
	leakDetector *security.LeakDetector
}

func New(database *db.DB, cfg *config.Config, leakDetector *security.LeakDetector) *Dispatcher {
	return &Dispatcher{
		db:           database,
		cfg:          cfg,
		leakDetector: leakDetector,
	}
}

func (d *Dispatcher) Run(ctx context.Context, dispatchCh <-chan types.Task) {
	log.Println("Dispatcher started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Dispatcher shutting down")
			return
		case task := <-dispatchCh:
			go d.dispatch(ctx, task)
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, task types.Task) {
	log.Printf("Dispatcher: processing task %s (routing=%s)", task.ID, task.RoutingFlag)

	var result *types.DispatchResult
	var err error

	switch task.RoutingFlag {
	case types.RoutingWeb:
		result, err = d.dispatchToGitHub(ctx, task)
	case types.RoutingInternal:
		result, err = d.dispatchLocal(ctx, task)
	default:
		err = fmt.Errorf("unknown routing flag: %s", task.RoutingFlag)
	}

	if err != nil {
		log.Printf("Dispatcher: task %s dispatch failed: %v", task.ID, err)
		d.handleFailure(ctx, task, err.Error())
		return
	}

	if result != nil && result.Success {
		log.Printf("Dispatcher: task %s completed successfully", task.ID)
	} else if result != nil {
		log.Printf("Dispatcher: task %s failed: %s", task.ID, result.Error)
		d.handleFailure(ctx, task, result.Error)
	}
}

func (d *Dispatcher) dispatchToGitHub(ctx context.Context, task types.Task) (*types.DispatchResult, error) {
	if d.cfg.GitHub.Token == "" {
		return nil, fmt.Errorf("GitHub token not configured")
	}

	branchName := fmt.Sprintf("task/%s", task.ID)

	packet, err := d.db.GetTaskPacket(ctx, task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task packet: %w", err)
	}

	if packet == nil {
		return nil, fmt.Errorf("no task packet found for task %s", task.ID)
	}

	platform := task.AssignedTo
	if platform == "" {
		platform = "chatgpt"
	}

	payload := map[string]interface{}{
		"task_id":     task.ID,
		"prompt":      packet.Prompt,
		"platform":    platform,
		"branch_name": branchName,
	}

	payloadJSON, _ := json.Marshal(payload)
	log.Printf("Dispatcher: would dispatch to GitHub: %s", string(payloadJSON))

	return &types.DispatchResult{
		TaskID:     task.ID,
		Success:    true,
		BranchName: branchName,
	}, nil
}

func (d *Dispatcher) dispatchLocal(ctx context.Context, task types.Task) (*types.DispatchResult, error) {
	if len(d.cfg.Runners.Internal) == 0 {
		return nil, fmt.Errorf("no internal runners configured")
	}

	runner := d.cfg.Runners.Internal[0]

	packet, err := d.db.GetTaskPacket(ctx, task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task packet: %w", err)
	}

	if packet == nil {
		return nil, fmt.Errorf("no task packet found for task %s", task.ID)
	}

	taskPacket := map[string]interface{}{
		"task_id": task.ID,
		"prompt":  packet.Prompt,
		"title":   packet.Title,
	}

	packetJSON, _ := json.Marshal(taskPacket)

	log.Printf("Dispatcher: running local command: %s", runner.Command)

	cmd := exec.CommandContext(ctx, runner.Command, "--task-packet", string(packetJSON))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &types.DispatchResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Sprintf("command failed: %v\noutput: %s", err, string(output)),
		}, nil
	}

	cleanOutput, warnings := d.leakDetector.Scan(string(output))
	if len(warnings) > 0 {
		log.Printf("Dispatcher: leak warnings for task %s: %+v", task.ID, warnings)
	}

	return &types.DispatchResult{
		TaskID:  task.ID,
		Success: true,
		Error:   "",
	}, nil
}

func (d *Dispatcher) handleFailure(ctx context.Context, task types.Task, errMsg string) {
	task.Status = types.StatusAvailable
	task.Attempts++

	if task.Attempts >= task.MaxAttempts {
		log.Printf("Dispatcher: task %s exceeded max attempts (%d), escalating", task.ID, task.Attempts)
		
		if err := d.db.ResetTask(ctx, task.ID, true); err != nil {
			log.Printf("Dispatcher: failed to escalate task %s: %v", task.ID, err)
		}
		return
	}

	log.Printf("Dispatcher: retrying task %s (attempt %d/%d)", task.ID, task.Attempts, task.MaxAttempts)
	
	if err := d.db.ResetTask(ctx, task.ID, false); err != nil {
		log.Printf("Dispatcher: failed to reset task %s: %v", task.ID, err)
	}
}

func (d *Dispatcher) createTaskRun(ctx context.Context, task types.Task) (*types.TaskRun, error) {
	run := &types.TaskRun{
		TaskID:   task.ID,
		Courier:  string(task.RoutingFlag),
		Platform: task.AssignedTo,
		ModelID:  task.AssignedTo,
		Status:   "running",
	}

	if err := d.db.CreateTaskRun(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func (d *Dispatcher) completeTaskRun(ctx context.Context, run *types.TaskRun, output string, errStr string) error {
	resultJSON := []byte(output)
	return d.db.CompleteTaskRun(ctx, run.ID, resultJSON, errStr)
}

func (d *Dispatcher) recordMetrics(ctx context.Context, modelID string, tokensIn, tokensOut int, success bool) {
	start := time.Now()
	
	if err := d.db.IncrementModelUsage(ctx, modelID, tokensIn, tokensOut, success); err != nil {
		log.Printf("Dispatcher: failed to record metrics: %v", err)
	}

	log.Printf("Dispatcher: recorded metrics for %s in %v", modelID, time.Since(start))
}
