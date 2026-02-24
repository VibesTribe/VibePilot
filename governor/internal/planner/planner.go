package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type Planner struct {
	runtime         *Runtime
	minPacketLength int
	maxRetries      int
}

type Runtime struct {
	ExecuteFunc func(ctx context.Context, roleID string, context map[string]interface{}) (*Result, error)
}

type Result struct {
	Success bool
	Output  interface{}
	Error   string
	Tokens  TokenUsage
}

type TokenUsage struct {
	Input  int
	Output int
	Total  int
}

type PlanInput struct {
	PRD          string
	ProjectID    string
	CodebaseInfo string
	LearnedRules []LearnedRule
}

type LearnedRule struct {
	ID        string                 `json:"id"`
	AppliesTo string                 `json:"applies_to"`
	RuleType  string                 `json:"rule_type"`
	RuleText  string                 `json:"rule_text"`
	Source    string                 `json:"source"`
	Priority  int                    `json:"priority"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type Plan struct {
	PRDRef               string              `json:"prd_ref"`
	PlanningPrinciples   []string            `json:"planning_principles"`
	Slices               []Slice             `json:"slices"`
	DependencyGraph      map[string][]string `json:"dependency_graph"`
	CrossSliceInterfaces []CrossInterface    `json:"cross_slice_interfaces"`
	ParallelGroups       [][]string          `json:"parallel_groups"`
	IsolationValidation  IsolationValidation `json:"isolation_validation"`
}

type Slice struct {
	SliceID     string   `json:"slice_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Owns        []string `json:"owns"`
	Exposes     []string `json:"exposes"`
	Consumes    []string `json:"consumes"`
	Phases      []Phase  `json:"phases"`
}

type Phase struct {
	PhaseID string `json:"phase_id"`
	Name    string `json:"name"`
	Tasks   []Task `json:"tasks"`
}

type Task struct {
	TaskID              string                 `json:"task_id"`
	SliceID             string                 `json:"slice_id"`
	Phase               string                 `json:"phase"`
	Title               string                 `json:"title"`
	Purpose             string                 `json:"purpose"`
	Objectives          []string               `json:"objectives"`
	Deliverables        []string               `json:"deliverables"`
	ExpectedOutput      map[string]interface{} `json:"expected_output"`
	Dependencies        []Dependency           `json:"dependencies"`
	DependencyCount     int                    `json:"dependency_count"`
	RoutingFlag         string                 `json:"routing_flag"`
	RoutingFlagReason   string                 `json:"routing_flag_reason"`
	Confidence          float64                `json:"confidence"`
	ConfidenceReasoning string                 `json:"confidence_reasoning"`
	SuggestedAgent      string                 `json:"suggested_agent"`
	EstimatedContext    string                 `json:"estimated_context"`
	PromptPacket        string                 `json:"prompt_packet"`
	SliceBoundary       *SliceBoundary         `json:"slice_boundary,omitempty"`
}

type Dependency struct {
	TaskID string `json:"task_id"`
	Type   string `json:"type"`
}

type SliceBoundary struct {
	TouchesSlices      []string `json:"touches_slices,omitempty"`
	ExposesToSlices    []string `json:"exposes_to_slices,omitempty"`
	ReceivesFromSlices []string `json:"receives_from_slices,omitempty"`
}

type CrossInterface struct {
	FromSlice string `json:"from_slice"`
	ToSlice   string `json:"to_slice"`
	Interface string `json:"interface"`
	Purpose   string `json:"purpose"`
}

type IsolationValidation struct {
	AllTasksSingleSlice  bool `json:"all_tasks_single_slice"`
	NoCrossSliceCodeDeps bool `json:"no_cross_slice_code_deps"`
	InterfacesExplicit   bool `json:"interfaces_explicit"`
	CascadeImpossible    bool `json:"cascade_impossible"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func New(runtime *Runtime, minPacketLength int) *Planner {
	if minPacketLength <= 0 {
		minPacketLength = 200
	}
	return &Planner{
		runtime:         runtime,
		minPacketLength: minPacketLength,
		maxRetries:      2,
	}
}

func (p *Planner) Plan(ctx context.Context, input *PlanInput) (*Plan, error) {
	context := map[string]interface{}{
		"task":     "plan",
		"prd":      input.PRD,
		"codebase": input.CodebaseInfo,
	}

	if len(input.LearnedRules) > 0 {
		context["learned_rules"] = input.LearnedRules
	}

	var plan *Plan
	var err error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		result, execErr := p.runtime.ExecuteFunc(ctx, "planner", context)
		if execErr != nil {
			return nil, fmt.Errorf("execute: %w", execErr)
		}

		if !result.Success {
			return nil, fmt.Errorf("planning failed: %s", result.Error)
		}

		plan, err = p.parsePlan(result)
		if err != nil {
			log.Printf("Planner: parse attempt %d failed: %v", attempt+1, err)
			if attempt < p.maxRetries {
				context["retry_hint"] = "Previous response could not be parsed. Return ONLY valid JSON."
				continue
			}
			return nil, err
		}

		validation := p.ValidatePlan(plan)
		if !validation.Valid {
			log.Printf("Planner: validation failed: %v", validation.Errors)
			if attempt < p.maxRetries {
				context["validation_errors"] = validation.Errors
				context["retry_hint"] = "Fix the validation errors and return corrected JSON."
				continue
			}
			return plan, fmt.Errorf("plan validation failed: %v", validation.Errors)
		}

		break
	}

	plan = p.EnsurePromptPackets(plan, input.PRD)

	return plan, nil
}

func (p *Planner) parsePlan(result *Result) (*Plan, error) {
	outputStr, ok := result.Output.(string)
	if !ok {
		return nil, fmt.Errorf("output is not string")
	}

	jsonStr := p.extractJSON(outputStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in output")
	}

	var plan Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return &plan, nil
}

func (p *Planner) extractJSON(output string) string {
	output = strings.TrimSpace(output)

	if (strings.HasPrefix(output, "{") && strings.HasSuffix(output, "}")) ||
		(strings.HasPrefix(output, "[") && strings.HasSuffix(output, "]")) {
		return output
	}

	codeBlockIdx := strings.Index(output, "```")
	if codeBlockIdx == -1 {
		braceStart := strings.Index(output, "{")
		braceEnd := strings.LastIndex(output, "}")
		if braceStart != -1 && braceEnd != -1 && braceEnd > braceStart {
			return output[braceStart : braceEnd+1]
		}
		return ""
	}

	afterBlock := output[codeBlockIdx+3:]

	newlineIdx := strings.Index(afterBlock, "\n")
	if newlineIdx != -1 {
		afterBlock = afterBlock[newlineIdx+1:]
	}

	blockEnd := strings.Index(afterBlock, "```")
	if blockEnd == -1 {
		return ""
	}

	return strings.TrimSpace(afterBlock[:blockEnd])
}

func (p *Planner) ValidatePlan(plan *Plan) *ValidationResult {
	var errors []string
	var warnings []string

	if len(plan.Slices) == 0 {
		errors = append(errors, "Plan has no slices")
	}

	taskMap := make(map[string]*Task)
	sliceTaskCount := make(map[string]int)

	for i := range plan.Slices {
		slice := &plan.Slices[i]
		if slice.SliceID == "" {
			errors = append(errors, fmt.Sprintf("Slice %d has no slice_id", i))
		}

		for j := range slice.Phases {
			phase := &slice.Phases[j]
			for k := range phase.Tasks {
				task := &phase.Tasks[k]

				if task.TaskID == "" {
					errors = append(errors, fmt.Sprintf("Task in %s-%s has no task_id", slice.SliceID, phase.PhaseID))
				} else {
					taskMap[task.TaskID] = task
					sliceTaskCount[slice.SliceID]++
				}

				if task.SliceID != slice.SliceID {
					warnings = append(warnings, fmt.Sprintf("Task %s slice_id mismatch: got %s, expected %s", task.TaskID, task.SliceID, slice.SliceID))
				}

				if task.Confidence < 0.95 {
					warnings = append(warnings, fmt.Sprintf("Task %s confidence %.2f < 0.95 - should be split", task.TaskID, task.Confidence))
				}

				if len(task.Dependencies) >= 2 && task.RoutingFlag == "web" {
					errors = append(errors, fmt.Sprintf("Task %s has %d dependencies but routing_flag=web (should be internal)", task.TaskID, len(task.Dependencies)))
				}

				if task.PromptPacket == "" || len(task.PromptPacket) < p.minPacketLength {
					warnings = append(warnings, fmt.Sprintf("Task %s prompt_packet too short or missing", task.TaskID))
				}
			}
		}
	}

	for taskID, deps := range plan.DependencyGraph {
		task, exists := taskMap[taskID]
		if !exists {
			warnings = append(warnings, fmt.Sprintf("Dependency graph references unknown task %s", taskID))
			continue
		}

		for _, depID := range deps {
			depTask, depExists := taskMap[depID]
			if !depExists {
				warnings = append(warnings, fmt.Sprintf("Task %s depends on unknown task %s", taskID, depID))
				continue
			}

			if depTask.SliceID != task.SliceID {
				hasInterfaceDep := false
				for _, iface := range plan.CrossSliceInterfaces {
					if iface.FromSlice == depTask.SliceID && iface.ToSlice == task.SliceID {
						hasInterfaceDep = true
						break
					}
				}
				if !hasInterfaceDep {
					warnings = append(warnings, fmt.Sprintf("Task %s (%s) depends on %s (%s) - cross-slice without interface",
						taskID, task.SliceID, depID, depTask.SliceID))
				}
			}
		}
	}

	return &ValidationResult{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}
}

func (p *Planner) EnsurePromptPackets(plan *Plan, prd string) *Plan {
	for i := range plan.Slices {
		for j := range plan.Slices[i].Phases {
			for k := range plan.Slices[i].Phases[j].Tasks {
				task := &plan.Slices[i].Phases[j].Tasks[k]

				if task.PromptPacket == "" || len(task.PromptPacket) < p.minPacketLength {
					task.PromptPacket = p.generatePromptPacket(task, prd)
				}
			}
		}
	}
	return plan
}

func (p *Planner) generatePromptPacket(task *Task, prd string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# TASK: %s - %s\n\n", task.TaskID, task.Title))
	sb.WriteString("## CONTEXT\n\n")
	sb.WriteString(fmt.Sprintf("This task is part of the %s slice, phase %s. ", task.SliceID, task.Phase))
	if task.Purpose != "" {
		sb.WriteString(task.Purpose)
	}
	sb.WriteString("\n\n")

	sb.WriteString("## WHAT TO BUILD\n\n")
	if len(task.Objectives) > 0 {
		sb.WriteString("### Objectives\n")
		for _, obj := range task.Objectives {
			sb.WriteString(fmt.Sprintf("- %s\n", obj))
		}
		sb.WriteString("\n")
	}

	if len(task.Deliverables) > 0 {
		sb.WriteString("### Deliverables\n")
		for _, d := range task.Deliverables {
			sb.WriteString(fmt.Sprintf("- %s\n", d))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## TECHNICAL SPECIFICATIONS\n\n")
	sb.WriteString(fmt.Sprintf("### Slice: %s\n", task.SliceID))
	sb.WriteString(fmt.Sprintf("### Phase: %s\n", task.Phase))
	sb.WriteString(fmt.Sprintf("### Routing: %s (%s)\n\n", task.RoutingFlag, task.RoutingFlagReason))

	sb.WriteString("## ACCEPTANCE CRITERIA\n\n")
	sb.WriteString("- [ ] All objectives completed\n")
	sb.WriteString("- [ ] All deliverables produced\n")
	sb.WriteString("- [ ] Code follows project conventions\n")
	sb.WriteString("- [ ] Tests pass (if applicable)\n\n")

	if len(task.ExpectedOutput) > 0 {
		sb.WriteString("## EXPECTED OUTPUT\n\n")
		expectedJSON, _ := json.MarshalIndent(task.ExpectedOutput, "", "  ")
		sb.WriteString("```json\n")
		sb.WriteString(string(expectedJSON))
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString(fmt.Sprintf("## CONFIDENCE\n\n%.2f\n\n", task.Confidence))

	sb.WriteString("## OUTPUT FORMAT\n\n")
	sb.WriteString("Return JSON:\n")
	sb.WriteString("```json\n")
	sb.WriteString(fmt.Sprintf("{\n"))
	sb.WriteString(fmt.Sprintf("  \"task_id\": \"%s\",\n", task.TaskID))
	sb.WriteString("  \"model_name\": \"[your model name]\",\n")
	sb.WriteString("  \"files_created\": [\"path1\", \"path2\"],\n")
	sb.WriteString("  \"files_modified\": [\"path1\"],\n")
	sb.WriteString("  \"summary\": \"Brief description of what was built\",\n")
	sb.WriteString("  \"tests_written\": [\"path/to/test.py\"],\n")
	sb.WriteString("  \"notes\": \"Any important decisions or things to know\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## DO NOT\n\n")
	sb.WriteString("- Add features not listed in this task\n")
	sb.WriteString("- Modify files not listed\n")
	sb.WriteString("- Add dependencies not specified\n")
	sb.WriteString("- Leave TODO comments\n")

	return sb.String()
}

func (p *Planner) ExtractAllTasks(plan *Plan) []Task {
	var tasks []Task
	for _, slice := range plan.Slices {
		for _, phase := range slice.Phases {
			tasks = append(tasks, phase.Tasks...)
		}
	}
	return tasks
}

func (p *Planner) GetTaskByID(plan *Plan, taskID string) *Task {
	for _, slice := range plan.Slices {
		for _, phase := range slice.Phases {
			for _, task := range phase.Tasks {
				if task.TaskID == taskID {
					return &task
				}
			}
		}
	}
	return nil
}

func (p *Planner) GetTasksBySlice(plan *Plan, sliceID string) []Task {
	var tasks []Task
	for _, slice := range plan.Slices {
		if slice.SliceID == sliceID {
			for _, phase := range slice.Phases {
				tasks = append(tasks, phase.Tasks...)
			}
		}
	}
	return tasks
}

func (p *Planner) GetTasksByPhase(plan *Plan, sliceID, phaseID string) []Task {
	var tasks []Task
	for _, slice := range plan.Slices {
		if slice.SliceID == sliceID {
			for _, phase := range slice.Phases {
				if phase.PhaseID == phaseID {
					tasks = append(tasks, phase.Tasks...)
				}
			}
		}
	}
	return tasks
}

func (p *Planner) GetReadyTasks(plan *Plan, completedTasks map[string]bool) []Task {
	var ready []Task
	taskMap := make(map[string]*Task)

	for i := range plan.Slices {
		for j := range plan.Slices[i].Phases {
			for k := range plan.Slices[i].Phases[j].Tasks {
				task := &plan.Slices[i].Phases[j].Tasks[k]
				taskMap[task.TaskID] = task
			}
		}
	}

	for _, slice := range plan.Slices {
		for _, phase := range slice.Phases {
			for _, task := range phase.Tasks {
				if completedTasks[task.TaskID] {
					continue
				}

				allDepsComplete := true
				for _, dep := range task.Dependencies {
					if !completedTasks[dep.TaskID] {
						allDepsComplete = false
						break
					}
				}

				if allDepsComplete {
					ready = append(ready, task)
				}
			}
		}
	}

	return ready
}

func (p *Planner) ValidateRoutingFlags(plan *Plan) *ValidationResult {
	var errors []string
	var warnings []string

	for _, slice := range plan.Slices {
		for _, phase := range slice.Phases {
			for _, task := range phase.Tasks {
				if task.RoutingFlag == "" {
					errors = append(errors, fmt.Sprintf("Task %s has no routing_flag", task.TaskID))
					continue
				}

				validFlags := map[string]bool{"internal": true, "web": true, "mcp": true}
				if !validFlags[task.RoutingFlag] {
					errors = append(errors, fmt.Sprintf("Task %s has invalid routing_flag: %s", task.TaskID, task.RoutingFlag))
				}

				if len(task.Dependencies) >= 2 && task.RoutingFlag == "web" {
					errors = append(errors, fmt.Sprintf("Task %s has %d dependencies but routing_flag=web", task.TaskID, len(task.Dependencies)))
				}

				if task.RoutingFlag == "web" && strings.Contains(task.RoutingFlagReason, "codebase") {
					warnings = append(warnings, fmt.Sprintf("Task %s flagged web but reason mentions codebase", task.TaskID))
				}
			}
		}
	}

	return &ValidationResult{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}
}
