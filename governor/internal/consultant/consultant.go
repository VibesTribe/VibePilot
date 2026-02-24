package consultant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vibepilot/governor/internal/agent"
)

type Consultant struct {
	runtime *Runtime
	role    *agent.Role
}

type Runtime struct {
	ExecuteFunc func(ctx context.Context, roleID string, context map[string]interface{}) (*agent.Result, error)
}

type ClarificationQuestion struct {
	Question string   `json:"question"`
	Reason   string   `json:"reason"`
	Options  []string `json:"options,omitempty"`
}

type ClarificationResult struct {
	Questions []ClarificationQuestion `json:"questions"`
	Ready     bool                    `json:"ready"`
	Gaps      []string                `json:"gaps"`
}

type PRD struct {
	Title             string    `json:"title"`
	ProblemStatement  string    `json:"problem_statement"`
	TargetUsers       []string  `json:"target_users"`
	SuccessCriteria   []string  `json:"success_criteria"`
	CoreFeatures      []Feature `json:"core_features"`
	OutOfScope        []string  `json:"out_of_scope"`
	TechConstraints   []string  `json:"tech_constraints"`
	Dependencies      []string  `json:"dependencies"`
	Risks             []Risk    `json:"risks"`
	QuestionsAnswered []string  `json:"questions_answered"`
	Approved          bool      `json:"approved"`
}

type Feature struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Inputs      string `json:"inputs,omitempty"`
	Outputs     string `json:"outputs,omitempty"`
}

type Risk struct {
	Risk       string `json:"risk"`
	Mitigation string `json:"mitigation"`
}

type ConsultInput struct {
	UserVision    string
	Conversation  []Message
	ExistingPRD   *PRD
	MarketContext string
	TechStack     string
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func New(runtime *Runtime) *Consultant {
	role := &agent.Role{
		ID:          "consultant",
		Name:        "Consultant",
		Description: "Interactive PRD generation. Works with human to clarify vision and produce zero-ambiguity PRD.",
		Skills:      []string{"analyze", "document", "specify"},
		Tools:       []string{"web_search", "supabase_query"},
	}

	return &Consultant{
		runtime: runtime,
		role:    role,
	}
}

func (c *Consultant) AnalyzeVision(ctx context.Context, vision string) (*ClarificationResult, error) {
	context := map[string]interface{}{
		"task":        "analyze_vision",
		"user_vision": vision,
	}

	result, err := c.runtime.ExecuteFunc(ctx, "consultant", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("analysis failed: %s", result.Error)
	}

	var clarification ClarificationResult
	if err := parseJSONResult(result, &clarification); err != nil {
		outputStr, _ := result.Output.(string)
		return c.parseClarificationFromText(outputStr, vision), nil
	}

	return &clarification, nil
}

func (c *Consultant) GenerateQuestions(ctx context.Context, vision string, answered []string) ([]ClarificationQuestion, error) {
	context := map[string]interface{}{
		"task":          "generate_questions",
		"user_vision":   vision,
		"already_asked": answered,
	}

	result, err := c.runtime.ExecuteFunc(ctx, "consultant", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("question generation failed: %s", result.Error)
	}

	var qr struct {
		Questions []ClarificationQuestion `json:"questions"`
	}

	if err := parseJSONResult(result, &qr); err != nil {
		outputStr, _ := result.Output.(string)
		return c.parseQuestionsFromText(outputStr), nil
	}

	return qr.Questions, nil
}

func (c *Consultant) BuildPRD(ctx context.Context, input *ConsultInput) (*PRD, error) {
	context := map[string]interface{}{
		"task":           "build_prd",
		"user_vision":    input.UserVision,
		"conversation":   input.Conversation,
		"market_context": input.MarketContext,
		"tech_stack":     input.TechStack,
	}

	if input.ExistingPRD != nil {
		context["existing_prd"] = input.ExistingPRD
	}

	result, err := c.runtime.ExecuteFunc(ctx, "consultant", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("prd build failed: %s", result.Error)
	}

	var prd PRD
	if err := parseJSONResult(result, &prd); err != nil {
		outputStr, _ := result.Output.(string)
		return c.parsePRDFromText(outputStr), nil
	}

	return &prd, nil
}

func (c *Consultant) ValidateZeroAmbiguity(prd *PRD) ([]string, bool) {
	var issues []string

	vagueTerms := []string{"simple", "fast", "user-friendly", "scalable", "robust", "flexible", "easy", "intuitive", "modern", "clean"}

	checkForVague := func(text, location string) {
		textLower := strings.ToLower(text)
		for _, term := range vagueTerms {
			if strings.Contains(textLower, term) {
				issues = append(issues, fmt.Sprintf("'%s' in %s is vague - define specifically", term, location))
			}
		}
	}

	if prd.Title == "" {
		issues = append(issues, "PRD has no title")
	}
	if prd.ProblemStatement == "" {
		issues = append(issues, "PRD has no problem statement")
	}
	if len(prd.TargetUsers) == 0 {
		issues = append(issues, "PRD has no target users defined")
	}
	if len(prd.SuccessCriteria) == 0 {
		issues = append(issues, "PRD has no success criteria - how will we know it's done?")
	}
	if len(prd.CoreFeatures) == 0 {
		issues = append(issues, "PRD has no core features defined")
	}

	checkForVague(prd.ProblemStatement, "problem statement")

	for i, feature := range prd.CoreFeatures {
		checkForVague(feature.Description, fmt.Sprintf("feature %d (%s)", i+1, feature.Name))
	}

	for i, criterion := range prd.SuccessCriteria {
		checkForVague(criterion, fmt.Sprintf("success criterion %d", i+1))
	}

	return issues, len(issues) == 0
}

func (c *Consultant) parseClarificationFromText(text, vision string) *ClarificationResult {
	questions := c.parseQuestionsFromText(text)

	gaps := []string{}
	if len(questions) == 0 {
		gaps = append(gaps, "Need more specific requirements")
		gaps = append(gaps, "Success criteria unclear")
		gaps = append(gaps, "Target users not defined")
	}

	return &ClarificationResult{
		Questions: questions,
		Ready:     len(questions) == 0,
		Gaps:      gaps,
	}
}

func (c *Consultant) parseQuestionsFromText(text string) []ClarificationQuestion {
	var questions []ClarificationQuestion

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasSuffix(line, "?") {
			questions = append(questions, ClarificationQuestion{
				Question: line,
				Reason:   "Clarification needed for zero-ambiguity PRD",
			})
		}
	}

	if len(questions) == 0 && strings.Contains(text, "?") {
		start := strings.Index(text, "?")
		for start != -1 {
			qEnd := start + 1
			qStart := 0
			for i := start - 1; i >= 0; i-- {
				if text[i] == '\n' || text[i] == '.' {
					qStart = i + 1
					break
				}
			}
			questions = append(questions, ClarificationQuestion{
				Question: strings.TrimSpace(text[qStart:qEnd]),
				Reason:   "Clarification needed",
			})
			remaining := text[qEnd:]
			start = strings.Index(remaining, "?")
			if start != -1 {
				text = remaining
				start = strings.Index(text, "?")
			}
		}
	}

	return questions
}

func (c *Consultant) parsePRDFromText(text string) *PRD {
	prd := &PRD{
		Title:             "Untitled Project",
		TargetUsers:       []string{},
		SuccessCriteria:   []string{},
		CoreFeatures:      []Feature{},
		OutOfScope:        []string{},
		TechConstraints:   []string{},
		Dependencies:      []string{},
		Risks:             []Risk{},
		QuestionsAnswered: []string{},
	}

	currentSection := ""
	var currentList *[]string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# ") {
			prd.Title = strings.TrimPrefix(line, "# ")
			continue
		}

		if strings.HasPrefix(line, "## ") {
			sectionName := strings.ToLower(strings.TrimPrefix(line, "## "))
			currentSection = sectionName

			switch sectionName {
			case "target users", "users":
				currentList = nil
			case "success criteria":
				currentList = &prd.SuccessCriteria
			case "out of scope":
				currentList = &prd.OutOfScope
			case "technical constraints", "constraints":
				currentList = &prd.TechConstraints
			case "dependencies":
				currentList = &prd.Dependencies
			default:
				currentList = nil
			}
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")

			if currentList != nil {
				*currentList = append(*currentList, item)
			}
			continue
		}

		if currentSection == "problem statement" || currentSection == "problem" {
			if prd.ProblemStatement != "" {
				prd.ProblemStatement += " "
			}
			prd.ProblemStatement += line
		}
	}

	return prd
}

func parseJSONResult(result *agent.Result, target interface{}) error {
	outputStr, ok := result.Output.(string)
	if !ok {
		return fmt.Errorf("output is not string")
	}

	decoder := json.NewDecoder(strings.NewReader(outputStr))
	return decoder.Decode(target)
}
