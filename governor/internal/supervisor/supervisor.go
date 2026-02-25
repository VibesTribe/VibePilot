package supervisor

import (
	"context"
	"log"
	"regexp"
	"strings"
)

type Action string

const (
	ActionApprove Action = "approve"
	ActionReject  Action = "reject"
	ActionHuman   Action = "human"
)

type PlanAction string

const (
	PlanActionApprove PlanAction = "approve"
	PlanActionReject  PlanAction = "reject"
	PlanActionCouncil PlanAction = "council"
)

type Decision struct {
	Action        Action
	Notes         string
	Issues        []string
	Warnings      []string
	Reason        string
	LearnedCaught []string
}

type PlanDecision struct {
	Action     PlanAction
	Notes      string
	Issues     []string
	Warnings   []string
	CouncilFor string
}

type ReviewInput struct {
	TaskID         string
	TaskType       string
	ExpectedFiles  []string
	ActualFiles    []string
	OutputContent  string
	VisualChange   bool
	SecurityImpact bool
}

type PlanReviewInput struct {
	PlanID       string
	PlanType     string
	Title        string
	Description  string
	TaskCount    int
	HasAuth      bool
	HasSecurity  bool
	HasMigration bool
	Priority     int
}

type ResearchReviewInput struct {
	SuggestionID   string
	SuggestionType string
	Title          string
	Description    string
	IsSimple       bool
	AffectsCore    bool
}

type ResearchDecision struct {
	Action     PlanAction
	Notes      string
	Issues     []string
	Warnings   []string
	CouncilFor string
}

type SupervisorRule struct {
	ID               string
	TriggerPattern   string
	TriggerCondition map[string]interface{}
	Action           string
	Reason           string
	TimesCaughtIssue int
}

type RuleProvider interface {
	GetSupervisorRules(ctx context.Context, taskType string, limit int) ([]SupervisorRule, error)
	RecordSupervisorRuleTriggered(ctx context.Context, ruleID string, caughtIssue bool) error
}

type Supervisor struct {
	rules    RuleProvider
	maxRules int
}

func New() *Supervisor {
	return &Supervisor{maxRules: 20}
}

func (s *Supervisor) SetRuleProvider(provider RuleProvider) {
	s.rules = provider
}

func (s *Supervisor) SetMaxRules(max int) {
	if max > 0 {
		s.maxRules = max
	}
}

func (s *Supervisor) Review(ctx context.Context, input *ReviewInput) Decision {
	var issues []string
	var warnings []string
	var learnedCaught []string

	issues, warnings = s.checkDeliverables(input, issues, warnings)

	issues, warnings, learnedCaught = s.checkCodeQuality(ctx, input, issues, warnings)

	if len(issues) > 0 {
		return Decision{
			Action:        ActionReject,
			Notes:         s.formatNotes(issues),
			Issues:        issues,
			LearnedCaught: learnedCaught,
		}
	}

	if input.TaskType == "ui_ux" || input.VisualChange {
		return Decision{
			Action: ActionHuman,
			Reason: "Visual changes require human approval",
			Notes:  "UI/UX changes must be reviewed by human before merge",
		}
	}

	if input.SecurityImpact {
		warnings = append(warnings, "Security-impacting change detected")
	}

	return Decision{
		Action:        ActionApprove,
		Warnings:      warnings,
		LearnedCaught: learnedCaught,
	}
}

func (s *Supervisor) checkDeliverables(input *ReviewInput, issues []string, warnings []string) ([]string, []string) {
	if len(input.ExpectedFiles) == 0 {
		return issues, warnings
	}

	expectedSet := make(map[string]bool)
	for _, f := range input.ExpectedFiles {
		expectedSet[f] = true
	}

	actualSet := make(map[string]bool)
	for _, f := range input.ActualFiles {
		actualSet[f] = true
	}

	var missing []string
	for f := range expectedSet {
		if !actualSet[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		issues = append(issues, "Missing deliverables: "+strings.Join(missing, ", "))
	}

	var extra []string
	for f := range actualSet {
		if !expectedSet[f] {
			extra = append(extra, f)
		}
	}
	if len(extra) > 0 {
		warnings = append(warnings, "Extra files created (scope creep): "+strings.Join(extra, ", "))
	}

	return issues, warnings
}

func (s *Supervisor) checkCodeQuality(ctx context.Context, input *ReviewInput, issues []string, warnings []string) ([]string, []string, []string) {
	var learnedCaught []string
	content := input.OutputContent
	contentLower := strings.ToLower(content)

	if strings.Contains(content, "TODO") || strings.Contains(content, "FIXME") {
		warnings = append(warnings, "Code contains TODO/FIXME comments")
	}

	if strings.Contains(content, "print(") && strings.Contains(content, "def ") {
		warnings = append(warnings, "Code contains print statements - consider logging")
	}

	hardcodedSecrets := []struct {
		name    string
		pattern string
	}{
		{"api_key", "sk-"},
		{"github_token", "ghp_"},
		{"aws_key", "AKIA"},
		{"password_literal", `password = "`},
		{"password_literal_single", `password='`},
	}

	for _, p := range hardcodedSecrets {
		if strings.Contains(contentLower, strings.ToLower(p.pattern)) {
			issues = append(issues, "Potential secret detected: "+p.name)
		}
	}

	if len(content) > 0 && len(content) < 50 {
		warnings = append(warnings, "Output appears truncated (very short)")
	}

	if s.rules != nil {
		learnedIssues, learnedWarnings, caught := s.applyLearnedRules(ctx, input.TaskType, content)
		issues = append(issues, learnedIssues...)
		warnings = append(warnings, learnedWarnings...)
		learnedCaught = caught
	}

	return issues, warnings, learnedCaught
}

func (s *Supervisor) applyLearnedRules(ctx context.Context, taskType, content string) (issues []string, warnings []string, caught []string) {
	rules, err := s.rules.GetSupervisorRules(ctx, taskType, s.maxRules)
	if err != nil {
		log.Printf("Supervisor: failed to get learned rules: %v", err)
		return nil, nil, nil
	}

	for _, rule := range rules {
		matched, err := regexp.MatchString("(?i)"+rule.TriggerPattern, content)
		if err != nil {
			if strings.Contains(strings.ToLower(content), strings.ToLower(rule.TriggerPattern)) {
				matched = true
			} else {
				continue
			}
		}

		if !matched {
			continue
		}

		caught = append(caught, rule.ID)

		issueText := rule.Reason
		if issueText == "" {
			issueText = "Learned rule triggered: " + rule.TriggerPattern
		}

		switch rule.Action {
		case "reject":
			issues = append(issues, issueText)
			if err := s.rules.RecordSupervisorRuleTriggered(ctx, rule.ID, true); err != nil {
				log.Printf("Supervisor: failed to record rule triggered: %v", err)
			}
		case "warn":
			warnings = append(warnings, issueText)
			if err := s.rules.RecordSupervisorRuleTriggered(ctx, rule.ID, false); err != nil {
				log.Printf("Supervisor: failed to record rule triggered: %v", err)
			}
		case "human_review":
			warnings = append(warnings, "Requires human review: "+issueText)
		}
	}

	return issues, warnings, caught
}

func (s *Supervisor) formatNotes(issues []string) string {
	if len(issues) == 0 {
		return ""
	}

	var notes []string
	notes = append(notes, "WHY: Quality check failed")
	notes = append(notes, "ISSUES: "+strings.Join(issues, "; "))
	notes = append(notes, "SUGGESTION: Fix the issues above and retry")
	return strings.Join(notes, " | ")
}

func (s *Supervisor) ReviewPlan(ctx context.Context, input *PlanReviewInput) PlanDecision {
	var issues []string
	var warnings []string

	if input.Title == "" {
		issues = append(issues, "Plan has no title")
	}
	if input.Description == "" {
		issues = append(issues, "Plan has no description")
	}
	if input.TaskCount == 0 {
		issues = append(issues, "Plan has no tasks defined")
	}

	if len(issues) > 0 {
		return PlanDecision{
			Action: PlanActionReject,
			Notes:  s.formatNotes(issues),
			Issues: issues,
		}
	}

	if s.needsCouncil(input) {
		reason := s.determineCouncilReason(input)
		return PlanDecision{
			Action:     PlanActionCouncil,
			CouncilFor: reason,
			Warnings:   warnings,
		}
	}

	return PlanDecision{
		Action:   PlanActionApprove,
		Warnings: warnings,
	}
}

func (s *Supervisor) needsCouncil(input *PlanReviewInput) bool {
	if input.PlanType == "system_improvement" {
		return true
	}
	if input.HasAuth || input.HasSecurity {
		return true
	}
	if input.HasMigration {
		return true
	}
	if input.TaskCount > 5 {
		return true
	}
	if input.Priority <= 1 {
		return true
	}

	titleLower := strings.ToLower(input.Title)
	if strings.Contains(titleLower, "architecture") ||
		strings.Contains(titleLower, "refactor") ||
		strings.Contains(titleLower, "redesign") {
		return true
	}

	return false
}

func (s *Supervisor) determineCouncilReason(input *PlanReviewInput) string {
	switch {
	case input.PlanType == "system_improvement":
		return "System improvement requires council vetting"
	case input.HasAuth:
		return "Authentication changes require security review"
	case input.HasSecurity:
		return "Security-related changes require review"
	case input.HasMigration:
		return "Data migration requires reversibility review"
	case input.TaskCount > 5:
		return "Complex plan with many tasks requires review"
	case input.Priority <= 1:
		return "Critical priority requires council oversight"
	default:
		return "Significant change requires council review"
	}
}

func (s *Supervisor) ReviewResearch(ctx context.Context, input *ResearchReviewInput) ResearchDecision {
	var issues []string
	var warnings []string

	if input.Title == "" {
		issues = append(issues, "Research suggestion has no title")
	}
	if input.Description == "" {
		issues = append(issues, "Research suggestion has no description")
	}

	if len(issues) > 0 {
		return ResearchDecision{
			Action: PlanActionReject,
			Notes:  s.formatNotes(issues),
			Issues: issues,
		}
	}

	if s.researchNeedsCouncil(input) {
		reason := s.determineResearchCouncilReason(input)
		return ResearchDecision{
			Action:     PlanActionCouncil,
			CouncilFor: reason,
			Warnings:   warnings,
		}
	}

	return ResearchDecision{
		Action:   PlanActionApprove,
		Warnings: warnings,
		Notes:    "Ready for Planner",
	}
}

func (s *Supervisor) researchNeedsCouncil(input *ResearchReviewInput) bool {
	if input.IsSimple {
		return false
	}

	if input.AffectsCore {
		return true
	}

	switch input.SuggestionType {
	case "new_platform", "add_model", "add_api_key":
		return false
	case "new_strategy", "architecture_change", "new_technique":
		return true
	default:
		return true
	}
}

func (s *Supervisor) determineResearchCouncilReason(input *ResearchReviewInput) string {
	switch {
	case input.AffectsCore:
		return "Change affects core system - requires council vetting"
	case input.SuggestionType == "new_strategy":
		return "New strategy requires principle alignment review"
	case input.SuggestionType == "architecture_change":
		return "Architecture changes require integration and reversibility review"
	case input.SuggestionType == "new_technique":
		return "New technique requires technical and principle review"
	default:
		return "Significant improvement requires council review"
	}
}
