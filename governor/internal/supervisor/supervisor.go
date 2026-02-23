package supervisor

import (
	"context"
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
	Action   Action
	Notes    string
	Issues   []string
	Warnings []string
	Reason   string
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

type Supervisor struct{}

func New() *Supervisor {
	return &Supervisor{}
}

func (s *Supervisor) Review(ctx context.Context, input *ReviewInput) Decision {
	var issues []string
	var warnings []string

	issues, warnings = s.checkDeliverables(input, issues, warnings)

	issues = s.checkCodeQuality(input.OutputContent, issues, warnings)

	if len(issues) > 0 {
		return Decision{
			Action: ActionReject,
			Notes:  s.formatNotes(issues),
			Issues: issues,
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
		Action:   ActionApprove,
		Warnings: warnings,
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

func (s *Supervisor) checkCodeQuality(content string, issues []string, warnings []string) []string {
	contentLower := strings.ToLower(content)

	if strings.Contains(content, "TODO") || strings.Contains(content, "FIXME") {
		warnings = append(warnings, "Code contains TODO/FIXME comments")
	}

	if strings.Contains(content, "print(") && strings.Contains(content, "def ") {
		warnings = append(warnings, "Code contains print statements - consider logging")
	}

	secretPatterns := []struct {
		name    string
		pattern string
	}{
		{"api_key", "sk-"},
		{"github_token", "ghp_"},
		{"aws_key", "AKIA"},
		{"password_literal", `password = "`},
		{"password_literal_single", `password='`},
	}

	for _, p := range secretPatterns {
		if strings.Contains(contentLower, strings.ToLower(p.pattern)) {
			issues = append(issues, "Potential secret detected: "+p.name)
		}
	}

	if len(content) > 0 && len(content) < 50 {
		warnings = append(warnings, "Output appears truncated (very short)")
	}

	return issues
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
