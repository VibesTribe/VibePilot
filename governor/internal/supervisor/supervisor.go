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
	ActionCouncil Action = "council"
)

type Decision struct {
	Action   Action
	Notes    string
	Issues   []string
	Warnings []string
	Reason   string
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

func (s *Supervisor) NeedsCouncil(taskType string, title string, priority int) bool {
	if taskType == "security" {
		return true
	}

	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "auth") ||
		strings.Contains(titleLower, "authentication") ||
		strings.Contains(titleLower, "architecture") ||
		strings.Contains(titleLower, "refactor") {
		return true
	}

	if priority <= 1 {
		return true
	}

	return false
}
