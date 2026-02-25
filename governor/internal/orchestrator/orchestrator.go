package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/researcher"
	"github.com/vibepilot/governor/internal/supervisor"
	"github.com/vibepilot/governor/internal/tester"
	"github.com/vibepilot/governor/internal/visual"
	"github.com/vibepilot/governor/pkg/types"
)

type Orchestrator struct {
	db           *db.DB
	gitree       *gitree.Gitree
	supervisor   *supervisor.Supervisor
	tester       *tester.Tester
	visualTester *visual.VisualTester
	researcher   *researcher.Researcher

	pendingTests chan string
	ctx          context.Context
}

func New(database *db.DB, git *gitree.Gitree, sup *supervisor.Supervisor, test *tester.Tester, visTest *visual.VisualTester, res *researcher.Researcher) *Orchestrator {
	return &Orchestrator{
		db:           database,
		gitree:       git,
		supervisor:   sup,
		tester:       test,
		visualTester: visTest,
		researcher:   res,
		pendingTests: make(chan string, 20),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	o.ctx = ctx
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
		log.Printf("Orchestrator: task %s has no branch - branch should be created at assignment", taskID[:8])
		o.handleRejection(ctx, task, "Task has no branch assigned")
		return
	}

	if err := o.gitree.CommitOutput(ctx, task.BranchName, result); err != nil {
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

	output, err := o.gitree.ReadBranchOutput(ctx, task.BranchName)
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

		if task.Type != "test" && task.Type != "docs" {
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusTesting, result)
			o.queueTest(taskID)
		} else {
			o.mergeTaskToModule(ctx, task)
		}

	case supervisor.ActionReject:
		log.Printf("Orchestrator: %s rejected: %s", taskID[:8], decision.Notes)
		o.db.LogOrchestratorEvent(ctx, "supervisor_reject", taskID, "", "", "", "", "Supervisor rejected", map[string]interface{}{"notes": decision.Notes})
		o.createSupervisorRulesFromRejection(ctx, taskID, task.Type, decision.Issues)
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

	result := o.tester.RunTestsWithType(ctx, task.BranchName, task.Type)

	if result.Passed {
		log.Printf("Orchestrator: %s tests passed, merging to module", taskID[:8])
		o.mergeTaskToModule(ctx, task)
	} else {
		log.Printf("Orchestrator: %s tests failed: %v", taskID[:8], result.Failures)
		notes := "Tests failed: " + strings.Join(result.Failures, "; ")
		o.handleRejection(ctx, task, notes)
	}
}

func (o *Orchestrator) mergeTaskToModule(ctx context.Context, task *types.Task) {
	taskID := task.ID

	if task.BranchName == "" {
		log.Printf("Orchestrator: %s has no branch, cannot merge", taskID[:8])
		return
	}

	if task.SliceID == "" {
		log.Printf("Orchestrator: %s has no slice_id, merging directly to main", taskID[:8])
		if err := o.gitree.MergeBranch(ctx, task.BranchName, "main"); err != nil {
			log.Printf("Orchestrator: failed to merge %s to main: %v", taskID[:8], err)
			return
		}
		o.gitree.DeleteBranch(ctx, task.BranchName)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusMerged, nil)
		o.db.LogOrchestratorEvent(ctx, "merged_to_main", taskID, "", "", "", "", "Merged to main", nil)
		return
	}

	moduleBranch := "module/" + task.SliceID
	if err := o.gitree.CreateModuleBranch(ctx, task.SliceID); err != nil {
		log.Printf("Orchestrator: failed to create module branch %s: %v", moduleBranch, err)
	}

	if err := o.gitree.MergeBranch(ctx, task.BranchName, moduleBranch); err != nil {
		log.Printf("Orchestrator: failed to merge %s to module %s: %v", taskID[:8], moduleBranch, err)
		return
	}

	o.gitree.DeleteBranch(ctx, task.BranchName)
	o.db.UpdateTaskStatus(ctx, taskID, types.StatusMerged, nil)
	o.db.LogOrchestratorEvent(ctx, "merged_to_module", taskID, "", "", "", "", "Merged to module", map[string]interface{}{
		"module_branch": moduleBranch,
	})

	log.Printf("Orchestrator: %s merged to %s", taskID[:8], moduleBranch)

	o.checkModuleCompletion(ctx, task.SliceID)
}

func (o *Orchestrator) checkModuleCompletion(ctx context.Context, sliceID string) {
	tasks, err := o.db.GetTasksBySlice(ctx, sliceID)
	if err != nil {
		log.Printf("Orchestrator: failed to get tasks for slice %s: %v", sliceID[:8], err)
		return
	}

	allMerged := true
	for _, t := range tasks {
		if t.Status != types.StatusMerged && t.Type != "merge" {
			allMerged = false
			break
		}
	}

	if !allMerged {
		log.Printf("Orchestrator: slice %s has pending tasks, waiting", sliceID[:8])
		return
	}

	moduleBranch := "module/" + sliceID
	log.Printf("Orchestrator: slice %s complete, merging module to main", sliceID[:8])

	if err := o.gitree.MergeBranch(ctx, moduleBranch, "main"); err != nil {
		log.Printf("Orchestrator: failed to merge module %s to main: %v", sliceID[:8], err)
		o.db.LogOrchestratorEvent(ctx, "module_merge_failed", "", "", "", "", "", "Module merge failed", map[string]interface{}{
			"slice_id":      sliceID,
			"module_branch": moduleBranch,
			"error":         err.Error(),
		})
		return
	}

	o.gitree.DeleteBranch(ctx, moduleBranch)
	o.db.LogOrchestratorEvent(ctx, "module_merged", "", "", "", "", "", "Module merged to main", map[string]interface{}{
		"slice_id":      sliceID,
		"module_branch": moduleBranch,
	})

	log.Printf("Orchestrator: module %s merged to main", sliceID[:8])
}

func (o *Orchestrator) handleRejection(ctx context.Context, task *types.Task, notes string) {
	taskID := task.ID

	currentTask, err := o.db.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}

	failureType, failureCategory := classifyFailure(notes)
	o.recordFailure(ctx, currentTask, failureType, failureCategory, notes)

	newAttempts := currentTask.Attempts + 1
	escalate := newAttempts >= currentTask.MaxAttempts

	o.db.LogOrchestratorEvent(ctx, "task_rejected", taskID, "", "", "", currentTask.AssignedTo, notes, map[string]interface{}{
		"attempt":          newAttempts,
		"max_attempts":     currentTask.MaxAttempts,
		"escalate":         escalate,
		"failure_type":     failureType,
		"failure_category": failureCategory,
	})

	if escalate {
		log.Printf("Orchestrator: %s escalated (%d/%d failures) - AI will analyze and resolve", taskID[:8], newAttempts, currentTask.MaxAttempts)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusEscalated, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		if o.ctx != nil {
			go o.handleEscalation(o.ctx, taskID, notes)
		}
	} else {
		if currentTask.BranchName != "" && o.gitree != nil {
			if err := o.gitree.ClearBranch(ctx, currentTask.BranchName); err != nil {
				log.Printf("Orchestrator: failed to clear branch %s: %v", currentTask.BranchName, err)
			} else {
				log.Printf("Orchestrator: cleared branch %s for retry", currentTask.BranchName)
			}
		}
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

func (o *Orchestrator) recordFailure(ctx context.Context, task *types.Task, failureType, failureCategory, notes string) {
	record := &db.FailureRecord{
		TaskID:          task.ID,
		FailureType:     failureType,
		FailureCategory: failureCategory,
		TaskType:        task.Type,
		Details: map[string]interface{}{
			"notes":   notes,
			"attempt": task.Attempts + 1,
		},
	}
	if task.AssignedTo != "" {
		record.ModelID = task.AssignedTo
	}

	if _, err := o.db.RecordFailure(ctx, record); err != nil {
		log.Printf("Orchestrator: failed to record failure for %s: %v", task.ID[:8], err)
	}
}

func classifyFailure(notes string) (failureType, failureCategory string) {
	notesLower := strings.ToLower(notes)

	if strings.Contains(notesLower, "timeout") || strings.Contains(notesLower, "timed out") {
		return "timeout", "model_issue"
	}
	if strings.Contains(notesLower, "rate limit") || strings.Contains(notesLower, "429") {
		return "rate_limited", "platform_issue"
	}
	if strings.Contains(notesLower, "context") || strings.Contains(notesLower, "token limit") {
		return "context_exceeded", "model_issue"
	}
	if strings.Contains(notesLower, "platform") && strings.Contains(notesLower, "down") {
		return "platform_down", "platform_issue"
	}
	if strings.Contains(notesLower, "test") && strings.Contains(notesLower, "fail") {
		return "test_failed", "quality_issue"
	}
	if strings.Contains(notesLower, "empty") || strings.Contains(notesLower, "no output") {
		return "empty_output", "model_issue"
	}
	if strings.Contains(notesLower, "deliverable") || strings.Contains(notesLower, "missing") {
		return "quality_rejected", "quality_issue"
	}
	if strings.Contains(notesLower, "latency") || strings.Contains(notesLower, "slow") {
		return "latency_high", "platform_issue"
	}

	return "unknown", "task_issue"
}

func (o *Orchestrator) createSupervisorRulesFromRejection(ctx context.Context, taskID string, taskType string, issues []string) {
	for _, issue := range issues {
		pattern := o.extractPatternFromIssue(issue)
		if pattern == "" {
			continue
		}

		taskTypePtr := &taskType
		if taskType == "" {
			taskTypePtr = nil
		}

		ruleID, err := o.db.CreateRuleFromSupervisorRejection(ctx, taskID, pattern, issue, taskTypePtr)
		if err != nil {
			log.Printf("Orchestrator: failed to create supervisor rule from rejection: %v", err)
			continue
		}
		log.Printf("Orchestrator: created supervisor rule %s from rejection (pattern: %s)", ruleID[:8], pattern)
	}
}

func (o *Orchestrator) extractPatternFromIssue(issue string) string {
	issueLower := strings.ToLower(issue)

	if strings.Contains(issueLower, "secret") || strings.Contains(issueLower, "api_key") || strings.Contains(issueLower, "token") {
		return "secret_pattern"
	}
	if strings.Contains(issueLower, "hardcoded") || strings.Contains(issueLower, "hard code") {
		return "hardcoded_value"
	}
	if strings.Contains(issueLower, "todo") || strings.Contains(issueLower, "fixme") {
		return "todo_comment"
	}
	if strings.Contains(issueLower, "print") {
		return "print_statement"
	}
	if strings.Contains(issueLower, "deliverable") || strings.Contains(issueLower, "missing") {
		return "missing_deliverable"
	}
	if strings.Contains(issueLower, "scope creep") || strings.Contains(issueLower, "extra file") {
		return "scope_creep"
	}

	if len(issue) > 10 {
		words := strings.Fields(issue)
		if len(words) >= 2 {
			return strings.ToLower(words[0] + "_" + words[1])
		}
	}

	return ""
}
