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
	"github.com/vibepilot/governor/internal/pool"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/pkg/types"
)

type Dispatcher struct {
	db           *db.DB
	cfg          *config.Config
	pool         *pool.Pool
	leakDetector *security.LeakDetector
}

func New(database *db.DB, cfg *config.Config, leakDetector *security.LeakDetector) *Dispatcher {
	return &Dispatcher{
		db:           database,
		cfg:          cfg,
		pool:         pool.New(database),
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
			go d.execute(ctx, task)
		}
	}
}

func (d *Dispatcher) execute(ctx context.Context, task types.Task) {
	log.Printf("Dispatcher: task %s (routing=%s, type=%s)", task.ID[:8], task.RoutingFlag, task.Type)

	runner, err := d.pool.SelectBest(ctx, string(task.RoutingFlag), task.Type)
	if err != nil {
		log.Printf("Dispatcher: pool error for %s: %v", task.ID[:8], err)
		d.handleFailure(ctx, task)
		return
	}
	if runner == nil {
		log.Printf("Dispatcher: no runner available for %s (routing=%s)", task.ID[:8], task.RoutingFlag)
		d.handleFailure(ctx, task)
		return
	}

	log.Printf("Dispatcher: selected runner %s (model=%s, priority=%d)", runner.ID[:8], runner.ModelID, runner.CostPriority)

	if err := d.db.ClaimTask(ctx, task.ID, runner.ModelID); err != nil {
		log.Printf("Dispatcher: claim failed for %s: %v", task.ID[:8], err)
		return
	}

	packet, err := d.db.GetTaskPacket(ctx, task.ID)
	if err != nil || packet == nil {
		log.Printf("Dispatcher: no packet for %s: %v", task.ID[:8], err)
		d.handleFailure(ctx, task)
		d.recordResult(ctx, runner.ID, task.Type, false, 0)
		return
	}

	output, tokensIn, tokensOut, execErr := d.runTool(ctx, runner.ToolID, packet.Prompt, 300)

	success := execErr == nil
	status := "success"
	var result interface{}
	if !success {
		status = "failed"
		result = map[string]interface{}{"error": execErr.Error()}
	} else {
		result = map[string]interface{}{"output": output}
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
		return
	}

	if d.pool.ShouldThrottle(runner) {
		d.pool.SetCooldown(ctx, runner.ID, d.timeUntilMidnight())
		log.Printf("Dispatcher: runner %s at 80%% daily, cooling down", runner.ID[:8])
	}

	if err := d.db.UpdateTaskStatus(ctx, task.ID, types.StatusReview, result); err != nil {
		log.Printf("Dispatcher: failed to update status for %s: %v", task.ID[:8], err)
		return
	}

	if err := d.db.UnlockDependents(ctx, task.ID); err != nil {
		log.Printf("Dispatcher: failed to unlock dependents for %s: %v", task.ID[:8], err)
	}

	log.Printf("Dispatcher: task %s completed successfully", task.ID[:8])
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
	_ = clean

	var result struct {
		Content      string `json:"content"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		output = string(raw)
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
	if taskPtr == nil {
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
