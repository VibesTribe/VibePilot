package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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

	pendingReviews chan string
	pendingTests   chan string
	pendingMerges  chan string

	state sync.Map
}

func New(database *db.DB, maint *maintenance.Maintenance, sup *supervisor.Supervisor, test *tester.Tester) *Orchestrator {
	return &Orchestrator{
		db:             database,
		maintenance:    maint,
		supervisor:     sup,
		tester:         test,
		pendingReviews: make(chan string, 20),
		pendingTests:   make(chan string, 20),
		pendingMerges:  make(chan string, 20),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	log.Println("Orchestrator started")

	go o.pollReviews(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Orchestrator shutting down")
			return

		case taskID := <-o.pendingReviews:
			go o.processReview(ctx, taskID)

		case taskID := <-o.pendingTests:
			go o.processTest(ctx, taskID)

		case taskID := <-o.pendingMerges:
			go o.processMerge(ctx, taskID)
		}
	}
}

func (o *Orchestrator) pollReviews(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tasks, err := o.db.GetTasksByStatus(ctx, string(types.StatusReview), 10)
			if err != nil {
				continue
			}
			for _, t := range tasks {
				select {
				case o.pendingReviews <- t.ID:
				default:
				}
			}
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

	o.db.UpdateTaskStatus(ctx, taskID, types.StatusReview, result)

	select {
	case o.pendingReviews <- taskID:
		log.Printf("Orchestrator: queued %s for review", taskID[:8])
	default:
		log.Printf("Orchestrator: review queue full, %s will be polled", taskID[:8])
	}
}

func (o *Orchestrator) processReview(ctx context.Context, taskID string) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil || task == nil {
		return
	}

	if task.Status != types.StatusReview {
		return
	}

	packet, err := o.db.GetTaskPacket(ctx, taskID)
	if err != nil || packet == nil {
		log.Printf("Orchestrator: no packet for review %s", taskID[:8])
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
		log.Printf("Orchestrator: %s approved, routing to testing", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusTesting, nil)
		select {
		case o.pendingTests <- taskID:
		default:
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
		select {
		case o.pendingMerges <- taskID:
		default:
		}
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
	taskPtr, _ := o.db.GetTaskByID(ctx, task.ID)
	if taskPtr == nil {
		return
	}

	newAttempts := taskPtr.Attempts + 1
	escalate := newAttempts >= taskPtr.MaxAttempts

	if escalate {
		log.Printf("Orchestrator: %s escalated (3+ failures) - AI will analyze and resolve", task.ID[:8])
		o.db.UpdateTaskStatus(ctx, task.ID, types.StatusEscalated, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		go o.handleEscalation(ctx, task.ID, notes)
	} else {
		o.db.ResetTask(ctx, task.ID, false)
		o.db.UpdateTaskStatus(ctx, task.ID, types.StatusAvailable, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		log.Printf("Orchestrator: %s returned to queue (attempt %d/%d)", task.ID[:8], newAttempts, taskPtr.MaxAttempts)
	}
}

func (o *Orchestrator) handleEscalation(ctx context.Context, taskID string, failureNotes string) {
	log.Printf("Orchestrator: Analyzing escalation for %s: %s", taskID[:8], failureNotes)

	// TODO: Route to research/solution model to analyze failure and propose fix
	// For now, log the escalation for analysis
	// Future:
	// - Check if platform was down -> retry with different platform
	// - Check if task too large -> call planner to split
	// - Check if prompt was bad -> regenerate prompt
	// - Send to research model for solution
}

func (o *Orchestrator) generateBranchName(task *types.Task) string {
	taskNum := task.TaskNumber
	if taskNum == "" {
		taskNum = task.ID[:8]
	}
	return fmt.Sprintf("task/%s", taskNum)
}
