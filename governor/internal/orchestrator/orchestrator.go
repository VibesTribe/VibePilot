package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/maintenance"
	"github.com/vibepilot/governor/internal/supervisor"
	"github.com/vibepilot/governor/internal/tester"
	"github.com/vibepilot/governor/pkg/types"
)

type Orchestrator struct {
	db          *db.DB
	maintenance *maintenance.Maintenance
	supervisor  *supervisor.Supervisor
	tester      *tester.Tester

	pendingTests  chan string
	pendingMerges chan string
}

func New(database *db.DB, maint *maintenance.Maintenance, sup *supervisor.Supervisor, test *tester.Tester) *Orchestrator {
	return &Orchestrator{
		db:            database,
		maintenance:   maint,
		supervisor:    sup,
		tester:        test,
		pendingTests:  make(chan string, 20),
		pendingMerges: make(chan string, 20),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	log.Println("Orchestrator started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Orchestrator shutting down")
			return

		case taskID := <-o.pendingTests:
			go o.processTest(ctx, taskID)

		case taskID := <-o.pendingMerges:
			go o.processMerge(ctx, taskID)
		}
	}
}

func (o *Orchestrator) OnTaskComplete(ctx context.Context, taskID string, result interface{}) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil || task == nil {
		log.Printf("Orchestrator: task %s not found", taskID[:8])
		return
	}

	if task.BranchName == "" {
		branchName := o.generateBranchName(task)
		if err := o.maintenance.CreateBranch(ctx, branchName); err != nil {
			log.Printf("Orchestrator: failed to create branch for %s: %v", taskID[:8], err)
			return
		}
		task.BranchName = branchName
		o.db.UpdateTaskBranch(ctx, taskID, branchName)
	}

	if err := o.maintenance.CommitOutput(ctx, task.BranchName, result); err != nil {
		log.Printf("Orchestrator: failed to commit output for %s: %v", taskID[:8], err)
		return
	}

	o.processSupervisorDecision(ctx, task, result)
}

func (o *Orchestrator) processSupervisorDecision(ctx context.Context, task *types.Task, result interface{}) {
	taskID := task.ID

	packet, err := o.db.GetTaskPacket(ctx, taskID)
	if err != nil || packet == nil {
		log.Printf("Orchestrator: no packet for task %s", taskID[:8])
		return
	}

	output, err := o.maintenance.ReadBranchOutput(ctx, task.BranchName)
	if err != nil {
		log.Printf("Orchestrator: failed to read branch output %s: %v", taskID[:8], err)
		return
	}

	decision := o.supervisor.Review(ctx, task, packet, output)

	switch decision.Action {
	case supervisor.ActionApprove:
		if task.Type == "test" || task.Type == "docs" {
			log.Printf("Orchestrator: %s approved (no testing needed for %s type)", taskID[:8], task.Type)
			o.queueMerge(taskID)
		} else {
			log.Printf("Orchestrator: %s approved, routing to testing", taskID[:8])
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusTesting, result)
			o.queueTest(taskID)
		}

	case supervisor.ActionReject:
		log.Printf("Orchestrator: %s rejected: %s", taskID[:8], decision.Notes)
		o.handleRejection(ctx, task, decision.Notes)

	case supervisor.ActionHuman:
		log.Printf("Orchestrator: %s awaiting human review", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason": decision.Notes,
		})

	case supervisor.ActionCouncil:
		log.Printf("Orchestrator: %s needs council review (not yet implemented)", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason": "Council review needed - pending implementation",
		})
	}
}

func (o *Orchestrator) queueTest(taskID string) {
	select {
	case o.pendingTests <- taskID:
	default:
		log.Printf("Orchestrator: test queue full, %s will retry", taskID[:8])
	}
}

func (o *Orchestrator) queueMerge(taskID string) {
	select {
	case o.pendingMerges <- taskID:
	default:
		log.Printf("Orchestrator: merge queue full, %s will retry", taskID[:8])
	}
}

func (o *Orchestrator) processTest(ctx context.Context, taskID string) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil || task == nil {
		return
	}

	if task.Status != types.StatusTesting {
		return
	}

	result := o.tester.RunTests(ctx, task.BranchName)

	if result.Passed {
		log.Printf("Orchestrator: %s tests passed, queueing for merge", taskID[:8])
		o.queueMerge(taskID)
	} else {
		log.Printf("Orchestrator: %s tests failed: %v", taskID[:8], result.Failures)
		notes := "Tests failed: " + strings.Join(result.Failures, "; ")
		o.handleRejection(ctx, task, notes)
	}
}

func (o *Orchestrator) processMerge(ctx context.Context, taskID string) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil || task == nil {
		return
	}

	if task.BranchName == "" {
		log.Printf("Orchestrator: %s has no branch to merge", taskID[:8])
		return
	}

	targetBranch := "main"

	if err := o.maintenance.MergeBranch(ctx, task.BranchName, targetBranch); err != nil {
		log.Printf("Orchestrator: failed to merge %s: %v", taskID[:8], err)
		return
	}

	o.maintenance.DeleteBranch(ctx, task.BranchName)

	o.db.UpdateTaskStatus(ctx, taskID, types.StatusMerged, map[string]interface{}{
		"merged_to": targetBranch,
	})

	o.db.UnlockDependents(ctx, taskID)

	log.Printf("Orchestrator: %s merged successfully", taskID[:8])
}

func (o *Orchestrator) handleRejection(ctx context.Context, task *types.Task, notes string) {
	taskID := task.ID

	currentTask, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil || currentTask == nil {
		return
	}

	newAttempts := currentTask.Attempts + 1
	escalate := newAttempts >= currentTask.MaxAttempts

	if escalate {
		log.Printf("Orchestrator: %s escalated (%d/%d failures) - AI will analyze and resolve", taskID[:8], newAttempts, currentTask.MaxAttempts)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusEscalated, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		go o.handleEscalation(ctx, taskID, notes)
	} else {
		o.db.ResetTask(ctx, taskID, false)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAvailable, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		log.Printf("Orchestrator: %s returned to queue (attempt %d/%d)", taskID[:8], newAttempts, currentTask.MaxAttempts)
	}
}

func (o *Orchestrator) handleEscalation(ctx context.Context, taskID string, failureNotes string) {
	log.Printf("Orchestrator: Analyzing escalation for %s: %s", taskID[:8], failureNotes)
}

func (o *Orchestrator) generateBranchName(task *types.Task) string {
	taskNum := task.TaskNumber
	if taskNum == "" {
		taskNum = task.ID[:8]
	}
	return fmt.Sprintf("task/%s", taskNum)
}
