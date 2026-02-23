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
	}

	if err := o.maintenance.CommitOutput(ctx, task.BranchName, result); err != nil {
		log.Printf("Orchestrator: failed to commit output for %s: %v", taskID[:8], err)
		return
	}

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

	escalate := taskPtr.Attempts+1 >= taskPtr.MaxAttempts

	o.db.ResetTask(ctx, task.ID, escalate)

	if escalate {
		log.Printf("Orchestrator: %s escalated after rejection", task.ID[:8])
	} else {
		log.Printf("Orchestrator: %s returned to queue", task.ID[:8])
	}
}

func (o *Orchestrator) generateBranchName(task *types.Task) string {
	taskNum := task.TaskNumber
	if taskNum == "" {
		taskNum = task.ID[:8]
	}
	return fmt.Sprintf("task/%s", taskNum)
}
