package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/gitree"
	"github.com/vibepilot/governor/internal/tester"
	"github.com/vibepilot/governor/internal/visual"
	"github.com/vibepilot/governor/pkg/types"
)

type Orchestrator struct {
	db           *db.DB
	gitree       *gitree.Gitree
	tester       *tester.Tester
	visualTester *visual.VisualTester

	pendingTests chan string
	ctx          context.Context
}

func New(database *db.DB, git *gitree.Gitree, test *tester.Tester, visTest *visual.VisualTester) *Orchestrator {
	return &Orchestrator{
		db:           database,
		gitree:       git,
		tester:       test,
		visualTester: visTest,
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

	if task.Type == "supervisor_review" {
		o.handleSupervisorResult(ctx, task, result)
		return
	}

	if task.Type == "researcher_analysis" {
		o.handleResearcherResult(ctx, task, result)
		return
	}

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

	o.queueSupervisorReview(ctx, task)
}

func (o *Orchestrator) queueSupervisorReview(ctx context.Context, task *types.Task) {
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

	reviewTaskID := generateID()
	reviewPrompt := buildSupervisorPrompt(task, packet, output)

	reviewPacket := &types.PromptPacket{
		TaskID:       reviewTaskID,
		Prompt:       reviewPrompt,
		Title:        "Supervisor review for " + taskID[:8],
		Objectives:   []string{"Review task output quality", "Approve, reject, or route to human"},
		Deliverables: []string{"decision.json"},
		Context:      fmt.Sprintf("Parent task: %s\nType: %s", taskID, task.Type),
		OutputFormat: map[string]interface{}{
			"type": "json",
			"schema": map[string]interface{}{
				"action": "approve|reject|human",
				"notes":  "string",
				"issues": "[]string",
				"reason": "string",
			},
		},
	}

	if err := o.db.CreateTask(ctx, &types.Task{
		ID:           reviewTaskID,
		Title:        "Review: " + task.Title,
		Type:         "supervisor_review",
		Priority:     task.Priority,
		Status:       types.StatusAvailable,
		RoutingFlag:  types.RoutingInternal,
		SliceID:      task.SliceID,
		ParentTaskID: taskID,
		BranchName:   task.BranchName,
		MaxAttempts:  1,
		PromptPacket: reviewPacket,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		log.Printf("Orchestrator: failed to create supervisor task for %s: %v", taskID[:8], err)
		return
	}

	o.db.LogOrchestratorEvent(ctx, "supervisor_queued", taskID, reviewTaskID, "", "", "", "Supervisor review queued", nil)
	log.Printf("Orchestrator: queued supervisor review for %s", taskID[:8])
}

func buildSupervisorPrompt(task *types.Task, packet *types.PromptPacket, output []string) string {
	var sb strings.Builder

	sb.WriteString("# Supervisor Review Task\n\n")
	sb.WriteString("You are the Supervisor. Review this task output and decide: approve, reject, or route to human.\n\n")

	sb.WriteString("## Original Task\n")
	sb.WriteString(fmt.Sprintf("- ID: %s\n", task.ID))
	sb.WriteString(fmt.Sprintf("- Type: %s\n", task.Type))
	sb.WriteString(fmt.Sprintf("- Title: %s\n\n", task.Title))

	sb.WriteString("## Expected Deliverables\n")
	for _, d := range packet.Deliverables {
		sb.WriteString(fmt.Sprintf("- %s\n", d))
	}
	sb.WriteString("\n")

	sb.WriteString("## Actual Output Files\n")
	for _, f := range output {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	sb.WriteString("\n")

	sb.WriteString("## Quality Checks\n")
	sb.WriteString("1. Does output match what was requested?\n")
	sb.WriteString("2. All deliverables created?\n")
	sb.WriteString("3. No scope creep (extra files)?\n")
	sb.WriteString("4. No security issues (secrets, injections)?\n")
	sb.WriteString("5. Code quality (no TODOs, proper structure)?\n\n")

	if task.Type == "ui_ux" {
		sb.WriteString("**This is a UI/UX task.** If quality passes, route to human for visual review.\n\n")
	}

	sb.WriteString("## Output Format\n")
	sb.WriteString("Return JSON:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"action\": \"approve|reject|human\",\n")
	sb.WriteString("  \"notes\": \"Explanation of decision\",\n")
	sb.WriteString("  \"issues\": [\"list of issues if rejecting\"],\n")
	sb.WriteString("  \"reason\": \"Reason if routing to human\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
}

func (o *Orchestrator) handleSupervisorResult(ctx context.Context, task *types.Task, result interface{}) {
	parentID := task.ParentTaskID
	if parentID == "" {
		log.Printf("Orchestrator: supervisor task %s has no parent", task.ID[:8])
		return
	}

	parent, err := o.db.GetTaskByID(ctx, parentID)
	if err != nil {
		log.Printf("Orchestrator: parent task %s not found", parentID[:8])
		return
	}

	decision := parseSupervisorResult(result)
	taskID := parent.ID

	o.db.LogOrchestratorEvent(ctx, "supervisor_decision", parentID, task.ID, "", "", "", decision.Action, map[string]interface{}{
		"notes":  decision.Notes,
		"issues": decision.Issues,
	})

	switch decision.Action {
	case "approve":
		log.Printf("Orchestrator: %s approved by supervisor", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusApproval, nil)

		if parent.Type != "test" && parent.Type != "docs" {
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusTesting, nil)
			o.queueTest(taskID)
		} else {
			o.mergeTaskToModule(ctx, parent)
		}

	case "reject":
		log.Printf("Orchestrator: %s rejected: %s", taskID[:8], decision.Notes)
		o.handleRejection(ctx, parent, decision.Notes)

	case "human":
		reason := decision.Reason
		if reason == "" {
			reason = decision.Notes
		}

		if parent.Type == "ui_ux" && o.visualTester != nil {
			packet, _ := o.db.GetTaskPacket(ctx, taskID)
			if packet != nil {
				visualResult := o.visualTester.TestVisual(ctx, parent.BranchName, packet.Deliverables)
				if !visualResult.Passed {
					notes := "Visual testing failed: " + strings.Join(visualResult.Failures, "; ")
					o.handleRejection(ctx, parent, notes)
					return
				}
				log.Printf("Orchestrator: %s visual testing passed", taskID[:8])
			}
		}

		log.Printf("Orchestrator: %s awaiting human review", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason": reason,
		})

	default:
		log.Printf("Orchestrator: %s unknown supervisor action: %s", taskID[:8], decision.Action)
		o.handleRejection(ctx, parent, "Unknown supervisor decision")
	}
}

type SupervisorDecision struct {
	Action string
	Notes  string
	Issues []string
	Reason string
}

func parseSupervisorResult(result interface{}) SupervisorDecision {
	decision := SupervisorDecision{Action: "reject"}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		decision.Notes = "Could not parse supervisor result"
		return decision
	}

	if action, ok := resultMap["action"].(string); ok {
		decision.Action = action
	}
	if notes, ok := resultMap["notes"].(string); ok {
		decision.Notes = notes
	}
	if issues, ok := resultMap["issues"].([]interface{}); ok {
		for _, i := range issues {
			if s, ok := i.(string); ok {
				decision.Issues = append(decision.Issues, s)
			}
		}
	}
	if reason, ok := resultMap["reason"].(string); ok {
		decision.Reason = reason
	}

	return decision
}

func (o *Orchestrator) handleResearcherResult(ctx context.Context, task *types.Task, result interface{}) {
	parentID := task.ParentTaskID
	if parentID == "" {
		log.Printf("Orchestrator: researcher task %s has no parent", task.ID[:8])
		return
	}

	parent, err := o.db.GetTaskByID(ctx, parentID)
	if err != nil {
		log.Printf("Orchestrator: parent task %s not found", parentID[:8])
		return
	}

	analysis := parseResearcherResult(result)
	taskID := parent.ID

	o.db.LogOrchestratorEvent(ctx, "analysis_complete", parentID, task.ID, "", "", analysis.Category, analysis.RootCause, map[string]interface{}{
		"suggestions": analysis.Suggestions,
		"auto_retry":  analysis.AutoRetry,
		"new_model":   analysis.NewModel,
	})

	switch analysis.Category {
	case "model_issue":
		if analysis.AutoRetry && analysis.NewModel != "" {
			log.Printf("Orchestrator: Auto-retrying %s with model %s", taskID[:8], analysis.NewModel)
			o.db.UpdateTaskStatus(ctx, taskID, types.StatusAvailable, map[string]interface{}{
				"attempts":        0,
				"assigned_to":     nil,
				"failure_notes":   "",
				"suggested_model": analysis.NewModel,
			})
			return
		}
		fallthrough

	case "task_definition", "dependency_issue":
		log.Printf("Orchestrator: %s requires human review", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason":           "Escalated: " + analysis.RootCause,
			"research_summary": formatAnalysis(analysis),
		})

	case "infrastructure":
		log.Printf("Orchestrator: Infrastructure issue for %s - retrying", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAvailable, map[string]interface{}{
			"attempts":      0,
			"failure_notes": "",
		})

	default:
		log.Printf("Orchestrator: Unknown category for %s", taskID[:8])
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusAwaitingHuman, map[string]interface{}{
			"reason":           "Escalated - unknown category",
			"research_summary": formatAnalysis(analysis),
		})
	}
}

type ResearcherAnalysis struct {
	Category    string
	RootCause   string
	Suggestions []string
	AutoRetry   bool
	NewModel    string
}

func parseResearcherResult(result interface{}) ResearcherAnalysis {
	analysis := ResearcherAnalysis{
		Category:    "unknown",
		RootCause:   "Could not parse analysis",
		Suggestions: []string{"Manual review required"},
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return analysis
	}

	if cat, ok := resultMap["category"].(string); ok {
		analysis.Category = cat
	}
	if cause, ok := resultMap["root_cause"].(string); ok {
		analysis.RootCause = cause
	}
	if suggestions, ok := resultMap["suggestions"].([]interface{}); ok {
		for _, s := range suggestions {
			if str, ok := s.(string); ok {
				analysis.Suggestions = append(analysis.Suggestions, str)
			}
		}
	}
	if autoRetry, ok := resultMap["auto_retry"].(bool); ok {
		analysis.AutoRetry = autoRetry
	}
	if newModel, ok := resultMap["new_model"].(string); ok {
		analysis.NewModel = newModel
	}

	return analysis
}

func formatAnalysis(analysis ResearcherAnalysis) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Category: %s\n", analysis.Category))
	sb.WriteString(fmt.Sprintf("Root Cause: %s\n", analysis.RootCause))
	sb.WriteString("Suggestions:\n")
	for i, s := range analysis.Suggestions {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
	}
	return sb.String()
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
		if t.Status != types.StatusMerged && t.Type != "merge" && t.Type != "supervisor_review" && t.Type != "researcher_analysis" {
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
		log.Printf("Orchestrator: %s escalated (%d/%d failures)", taskID[:8], newAttempts, currentTask.MaxAttempts)
		o.db.UpdateTaskStatus(ctx, taskID, types.StatusEscalated, map[string]interface{}{
			"attempts":      newAttempts,
			"failure_notes": notes,
		})
		go o.queueResearcherAnalysis(ctx, taskID, notes)
	} else {
		if currentTask.BranchName != "" && o.gitree != nil {
			if err := o.gitree.ClearBranch(ctx, currentTask.BranchName); err != nil {
				log.Printf("Orchestrator: failed to clear branch %s: %v", currentTask.BranchName, err)
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

func (o *Orchestrator) queueResearcherAnalysis(ctx context.Context, taskID string, failureNotes string) {
	analysisTaskID := generateID()

	analysisPrompt := buildResearcherPrompt(taskID, failureNotes)

	analysisPacket := &types.PromptPacket{
		TaskID:       analysisTaskID,
		Prompt:       analysisPrompt,
		Title:        "Researcher analysis for " + taskID[:8],
		Objectives:   []string{"Analyze failure", "Categorize root cause", "Suggest resolution"},
		Deliverables: []string{"analysis.json"},
		Context:      fmt.Sprintf("Escalated task: %s\nFailure: %s", taskID, failureNotes),
		OutputFormat: map[string]interface{}{
			"type": "json",
			"schema": map[string]interface{}{
				"category":    "model_issue|task_definition|dependency_issue|infrastructure|unknown",
				"root_cause":  "string",
				"suggestions": "[]string",
				"auto_retry":  "bool",
				"new_model":   "string (optional)",
			},
		},
	}

	if err := o.db.CreateTask(ctx, &types.Task{
		ID:           analysisTaskID,
		Title:        "Analyze: " + taskID[:8],
		Type:         "researcher_analysis",
		Priority:     1,
		Status:       types.StatusAvailable,
		RoutingFlag:  types.RoutingInternal,
		ParentTaskID: taskID,
		MaxAttempts:  1,
		PromptPacket: analysisPacket,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}); err != nil {
		log.Printf("Orchestrator: failed to create researcher task for %s: %v", taskID[:8], err)
		return
	}

	o.db.LogOrchestratorEvent(ctx, "researcher_queued", taskID, analysisTaskID, "", "", "", "Researcher analysis queued", nil)
	log.Printf("Orchestrator: queued researcher analysis for %s", taskID[:8])
}

func buildResearcherPrompt(taskID string, failureNotes string) string {
	var sb strings.Builder

	sb.WriteString("# Researcher Analysis Task\n\n")
	sb.WriteString("You are the Researcher. Analyze this task failure and recommend resolution.\n\n")

	sb.WriteString("## Task ID\n")
	sb.WriteString(taskID)
	sb.WriteString("\n\n")

	sb.WriteString("## Failure Notes\n")
	sb.WriteString(failureNotes)
	sb.WriteString("\n\n")

	sb.WriteString("## Categories\n")
	sb.WriteString("- **model_issue**: Model timeout, context exceeded, poor output quality\n")
	sb.WriteString("- **task_definition**: Unclear prompt, missing deliverables, ambiguous requirements\n")
	sb.WriteString("- **dependency_issue**: Missing dependencies, version conflicts\n")
	sb.WriteString("- **infrastructure**: Rate limits, platform down, git failures\n")
	sb.WriteString("- **unknown**: Cannot determine from available information\n\n")

	sb.WriteString("## Output Format\n")
	sb.WriteString("Return JSON:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"category\": \"one of the categories above\",\n")
	sb.WriteString("  \"root_cause\": \"Clear explanation of what went wrong\",\n")
	sb.WriteString("  \"suggestions\": [\"Specific actionable suggestions\"],\n")
	sb.WriteString("  \"auto_retry\": true/false,\n")
	sb.WriteString("  \"new_model\": \"model_id if auto_retry and different model suggested\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
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

	return "unknown", "task_issue"
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
