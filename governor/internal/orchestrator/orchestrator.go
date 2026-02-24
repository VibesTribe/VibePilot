package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/maintenance"
	"github.com/vibepilot/governor/internal/researcher"
	"github.com/vibepilot/governor/internal/supervisor"
	"github.com/vibepilot/governor/internal/tester"
	"github.com/vibepilot/governor/internal/visual"
	"github.com/vibepilot/governor/pkg/types"
)

type Orchestrator struct {
	db           *db.DB
	maintenance  *maintenance.Maintenance
	supervisor   *supervisor.Supervisor
	tester       *tester.Tester
	visualTester *visual.VisualTester
	researcher   *researcher.Researcher

	pendingTests chan string
}

func New(database *db.DB, maint *maintenance.Maintenance, sup *supervisor.Supervisor, test *tester.Tester, visTest *visual.VisualTester, res *researcher.Researcher) *Orchestrator {
	return &Orchestrator{
		db:           database,
		maintenance:  maint,
		supervisor:   sup,
		tester:       test,
		visualTester: visTest,
		researcher:   res,
		pendingTests: make(chan string, 20),
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
		}
	}
}

func (o *Orchestrator) OnTaskComplete(ctx context.Context, taskID string, result interface{}) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Printf("Orchestrator: task %s not found: %v", taskID[:8], err)
		return
	}

	o.db.LogOrchestratorEvent(ctx, "task_complete", taskID, "", "", "", task.AssignedTo, "Task execution completed", nil)

	if task.BranchName == "" {
		branchName := o.generateBranchName(task)
		if err := o.maintenance.CreateBranch(ctx, branchName); err != nil {
			log.Printf("Orchestrator: failed to create branch for %s: %v", taskID[:8], err)
			o.db.LogOrchestratorEvent(ctx, "branch_failed", taskID, "", "", "", "", "Branch creation failed", map[string]interface{}{"error": err.Error()})
			return
		}
		task.BranchName = branchName
		o.db.UpdateTaskBranch(ctx, taskID, branchName)
		o.db.LogOrchestratorEvent(ctx, "branch_created", taskID, "", "", "", "", "Branch created", map[string]interface{}{"branch": branchName})
	}

	if err := o.maintenance.CommitOutput(ctx, task.BranchName, result); err != nil {
		log.Printf("Orchestrator: commit failed for %s: %v", taskID[:8], err)
		o.db.LogOrchestratorEvent(ctx, "commit_failed", taskID, "", "", "", "", "Commit failed", map[string]interface{}{"error": err.Error()})
		o.handleRejection(ctx, task, fmt.Sprintf("Commit failed: %v", err))
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

	reviewInput := &supervisor.ReviewInput{
		TaskID:         taskID,
		TaskType:       task.Type,
		ExpectedFiles:  packet.Deliverables,
		ActualFiles:    output,
		VisualChange:   task.Type == "ui_ux",
		SecurityImpact: false,
	}

	decision := o.supervisor.Review(ctx, reviewInput)

	switch decision.Action {
	case supervisor.ActionApprove:
		o.db.LogOrchestratorEvent(ctx, "supervisor_approve", taskID, "", "", "", "", "Supervisor approved", nil)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusApproval, nil)

		mergeTaskID := generateID()
		if err := o.db.CreateMergeTask(ctx, mergeTaskID, taskID, task.SliceID, task.BranchName, task.Title); err != nil {
			log.Printf("Orchestrator: failed to create merge task for %s: %v", taskID[:8], err)
		} else {
			log.Printf("Orchestrator: %s approved, created merge task %s", taskID[:8], mergeTaskID[:8])
		}

		if task.Type != "test" && task.Type != "docs" {
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusTesting, result)
			o.queueTest(taskID)
		}

	case supervisor.ActionReject:
		log.Printf("Orchestrator: %s rejected: %s", taskID[:8], decision.Notes)
		o.db.LogOrchestratorEvent(ctx, "supervisor_reject", taskID, "", "", "", "", "Supervisor rejected", map[string]interface{}{"notes": decision.Notes})
		o.handleRejection(ctx, task, decision.Notes)

	case supervisor.ActionHuman:
		reason := decision.Reason
		if reason == "" {
			reason = decision.Notes
		}

		if task.Type == "ui_ux" && o.visualTester != nil {
			visualResult := o.visualTester.TestVisual(ctx, task.BranchName, packet.Deliverables)
			if !visualResult.Passed {
				log.Printf("Orchestrator: %s visual testing failed: %v", taskID[:8], visualResult.Failures)
				notes := "Visual testing failed: " + strings.Join(visualResult.Failures, "; ")
				o.db.LogOrchestratorEvent(ctx, "visual_test_failed", taskID, "", "", "", "", notes, nil)
				o.handleRejection(ctx, task, notes)
				return
			}
			log.Printf("Orchestrator: %s visual testing passed, routing to human", taskID[:8])
			o.db.LogOrchestratorEvent(ctx, "visual_test_passed", taskID, "", "", "", "", "Visual tests passed", nil)
		}

		log.Printf("Orchestrator: %s awaiting human review", taskID[:8])
		o.db.LogOrchestratorEvent(ctx, "awaiting_human", taskID, "", "", "", "", reason, nil)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason": reason,
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

func (o *Orchestrator) processTest(ctx context.Context, taskID string) {
	task, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}

	if task.Status != types.StatusTesting {
		return
	}

	result := o.tester.RunTests(ctx, task.BranchName)

	if result.Passed {
		log.Printf("Orchestrator: %s tests passed, creating merge task", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusApproval, nil)

		mergeTaskID := generateID()
		if err := o.db.CreateMergeTask(ctx, mergeTaskID, taskID, task.SliceID, task.BranchName, task.Title); err != nil {
			log.Printf("Orchestrator: failed to create merge task for %s: %v", taskID[:8], err)
		}
	} else {
		log.Printf("Orchestrator: %s tests failed: %v", taskID[:8], result.Failures)
		notes := "Tests failed: " + strings.Join(result.Failures, "; ")
		o.handleRejection(ctx, task, notes)
	}
}

func (o *Orchestrator) handleRejection(ctx context.Context, task *types.Task, notes string) {
	taskID := task.ID

	currentTask, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}

	newAttempts := currentTask.Attempts + 1
	escalate := newAttempts >= currentTask.MaxAttempts

	o.db.LogOrchestratorEvent(ctx, "task_rejected", taskID, "", "", "", currentTask.AssignedTo, notes, map[string]interface{}{
		"attempt":      newAttempts,
		"max_attempts": currentTask.MaxAttempts,
		"escalate":     escalate,
	})

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
	o.db.LogOrchestratorEvent(ctx, "escalated", taskID, "", "", "", "", failureNotes, nil)
	log.Printf("Orchestrator: Analyzing escalation for %s: %s", taskID[:8], failureNotes)

	if o.researcher == nil {
		log.Printf("Orchestrator: No researcher configured, cannot analyze escalation")
		o.db.LogOrchestratorEvent(ctx, "escalation_failed", taskID, "", "", "", "", "no researcher", nil)
		return
	}

	result, err := o.researcher.AnalyzeEscalation(ctx, taskID, failureNotes)
	if err != nil {
		log.Printf("Orchestrator: Researcher analysis failed for %s: %v", taskID[:8], err)
		o.db.LogOrchestratorEvent(ctx, "escalation_failed", taskID, "", "", "", "", err.Error(), nil)
		return
	}

	if err := o.researcher.RecordAnalysis(ctx, taskID, result); err != nil {
		log.Printf("Orchestrator: Failed to record analysis for %s: %v", taskID[:8], err)
	}

	log.Printf("Orchestrator: Analysis complete for %s - Category: %s, RootCause: %s",
		taskID[:8], result.Category, result.RootCause)
	o.db.LogOrchestratorEvent(ctx, "analysis_complete", taskID, "", "", "", result.Category, result.RootCause, map[string]interface{}{
		"suggestions": result.Suggestions,
		"auto_retry":  result.AutoRetry,
		"new_model":   result.NewModel,
	})

	switch result.Category {
	case researcher.CategoryModelIssue:
		if result.AutoRetry && result.NewModel != "" {
			log.Printf("Orchestrator: Auto-retrying %s with alternative model %s", taskID[:8], result.NewModel)
			o.db.AppendRoutingHistory(ctx, taskID, "", result.NewModel, result.RootCause)
			o.db.LogOrchestratorEvent(ctx, "rerouted", taskID, "", "", "", result.NewModel, "auto_retry", nil)
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusAvailable, map[string]interface{}{
				"attempts":        0,
				"assigned_to":     nil,
				"failure_notes":   "",
				"suggested_model": result.NewModel,
			})
			return
		}
		fallthrough

	case researcher.CategoryTaskDefinition, researcher.CategoryDependency:
		log.Printf("Orchestrator: %s requires human review - routing to awaiting_human", taskID[:8])
		o.db.LogOrchestratorEvent(ctx, "human_review", taskID, "", "", "", "", result.RootCause, nil)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason":           "Escalated after analysis: " + result.RootCause,
			"research_summary": o.researcher.FormatAnalysisForHuman(result),
		})

	case researcher.CategoryInfrastructure:
		log.Printf("Orchestrator: Infrastructure issue for %s - will retry after cooldown", taskID[:8])
		o.db.LogOrchestratorEvent(ctx, "infrastructure_retry", taskID, "", "", "", "", result.RootCause, nil)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAvailable, map[string]interface{}{
			"attempts":      0,
			"failure_notes": "",
		})

	default:
		log.Printf("Orchestrator: Unknown category for %s - defaulting to human review", taskID[:8])
		o.db.LogOrchestratorEvent(ctx, "unknown_category", taskID, "", "", "", "", result.RootCause, nil)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason":           "Escalated after 3 failures - unknown category",
			"research_summary": o.researcher.FormatAnalysisForHuman(result),
		})
	}
}

func (o *Orchestrator) generateBranchName(task *types.Task) string {
	taskNum := task.TaskNumber
	if taskNum == "" {
		taskNum = task.ID[:8]
	}
	return fmt.Sprintf("task/%s", taskNum)
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
