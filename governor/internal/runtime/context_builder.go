package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type RPCQuerier interface {
	Query(ctx context.Context, table string, filters map[string]any) (json.RawMessage, error)
	RPC(ctx context.Context, name string, params map[string]any) ([]byte, error)
}

type ContextBuilder struct {
	db RPCQuerier
}

func NewContextBuilder(db RPCQuerier) *ContextBuilder {
	return &ContextBuilder{db: db}
}

func (b *ContextBuilder) BuildPlannerContext(ctx context.Context, projectType string) (string, error) {
	var contextBuilder strings.Builder

	// Query incomplete slices for task numbering context
	slices, err := b.db.RPC(ctx, "get_slice_task_info", nil)
	if err == nil {
		var sliceList []map[string]any
		if err := json.Unmarshal(slices, &sliceList); err == nil && len(sliceList) > 0 {
			contextBuilder.WriteString("## Incomplete Slices\n\n")
			contextBuilder.WriteString("If your PRD continues an existing slice, use that slice_id and continue numbering from the last task.\n")
			contextBuilder.WriteString("Otherwise, create a new slice_id and start at T001.\n\n")
			for _, s := range sliceList {
				sliceID, _ := s["slice_id"].(string)
				lastTask, _ := s["last_task_number"].(string)
				count, _ := s["task_count"].(float64)
				if sliceID != "" {
					// Calculate next task number
					nextNum := int(count) + 1
					contextBuilder.WriteString(fmt.Sprintf("- %s: %d tasks, last %s → continue at T%03d\n", sliceID, int(count), lastTask, nextNum))
				}
			}
			contextBuilder.WriteString("\n")
		}
	}

	rules, err := b.db.RPC(ctx, "get_planner_rules", map[string]any{
		"p_applies_to": projectType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				source, _ := rule["source"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s (from %s)\n", ruleText, source))
			}
		}
	}

	failures, err := b.db.RPC(ctx, "get_recent_failures", map[string]any{
		"p_task_type": projectType,
		"p_since":     "NOW() - INTERVAL '7 days'",
	})
	if err == nil {
		var failureList []map[string]any
		if err := json.Unmarshal(failures, &failureList); err == nil && len(failureList) > 0 {
			contextBuilder.WriteString("\n## Recent Failures to Avoid\n\n")
			for _, f := range failureList {
				failureType, _ := f["failure_type"].(string)
				modelID, _ := f["model_id"].(string)
				count, _ := f["failure_count"].(float64)
				if modelID != "" {
					contextBuilder.WriteString(fmt.Sprintf("- %s on %s (%d occurrences)\n", failureType, modelID, int(count)))
				} else {
					contextBuilder.WriteString(fmt.Sprintf("- %s (%d occurrences)\n", failureType, int(count)))
				}
			}
		}
	}

	return contextBuilder.String(), nil
}

func (b *ContextBuilder) BuildSupervisorContext(ctx context.Context, taskType string) (string, error) {
	var contextBuilder strings.Builder

	rules, err := b.db.RPC(ctx, "get_supervisor_rules", map[string]any{
		"p_applies_to": taskType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Review Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", ruleText))
			}
		}
	}

	return contextBuilder.String(), nil
}

func (b *ContextBuilder) BuildTesterContext(ctx context.Context, taskType string) (string, error) {
	var contextBuilder strings.Builder

	rules, err := b.db.RPC(ctx, "get_tester_rules", map[string]any{
		"p_applies_to": taskType,
		"p_limit":      20,
	})
	if err == nil {
		var rulesList []map[string]any
		if err := json.Unmarshal(rules, &rulesList); err == nil && len(rulesList) > 0 {
			contextBuilder.WriteString("\n## Learned Testing Rules\n\n")
			for _, rule := range rulesList {
				ruleText, _ := rule["rule_text"].(string)
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", ruleText))
			}
		}
	}

	return contextBuilder.String(), nil
}

func (b *ContextBuilder) GetRoutingHeuristic(ctx context.Context, taskType string) (modelID string, action map[string]any) {
	result, err := b.db.RPC(ctx, "get_heuristic", map[string]any{
		"p_task_type": taskType,
		"p_condition": map[string]any{},
	})
	if err != nil {
		return "", nil
	}

	var heuristics []map[string]any
	if err := json.Unmarshal(result, &heuristics); err != nil || len(heuristics) == 0 {
		return "", nil
	}

	h := heuristics[0]
	modelID, _ = h["preferred_model"].(string)
	action, _ = h["action"].(map[string]any)
	return modelID, action
}

func (b *ContextBuilder) GetProblemSolution(ctx context.Context, failureType, taskType string) (solutionType string, solutionModel string, details map[string]any) {
	result, err := b.db.RPC(ctx, "get_problem_solution", map[string]any{
		"p_failure_type": failureType,
		"p_task_type":    taskType,
		"p_keywords":     []string{},
	})
	if err != nil {
		return "", "", nil
	}

	var solutions []map[string]any
	if err := json.Unmarshal(result, &solutions); err != nil || len(solutions) == 0 {
		return "", "", nil
	}

	s := solutions[0]
	solutionType, _ = s["solution_type"].(string)
	solutionModel, _ = s["solution_model"].(string)
	details, _ = s["solution_details"].(map[string]any)
	return solutionType, solutionModel, details
}
