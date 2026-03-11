package runtime

import (
	"encoding/json"
	"strings"
)

type Issue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type SupervisorDecision struct {
	Action     string `json:"action"`
	TaskID     string `json:"task_id"`
	TaskNumber string `json:"task_number"`
	Decision   string `json:"decision"`
	NextAction string `json:"next_action"`
	Checks     struct {
		AllDeliverablesPresent bool `json:"all_deliverables_present"`
		TestsWritten           bool `json:"tests_written"`
		NoHardcodedSecrets     bool `json:"no_hardcoded_secrets"`
		PatternConsistency     bool `json:"pattern_consistency"`
		ErrorHandlingPresent   bool `json:"error_handling_present"`
		UnexpectedChanges      bool `json:"unexpected_changes"`
	} `json:"checks"`
	IssuesRaw      json.RawMessage `json:"issues"`
	Issues         []Issue         `json:"-"`
	ReturnFeedback struct {
		Summary        string   `json:"summary"`
		SpecificIssues []string `json:"specific_issues"`
		Suggestions    []string `json:"suggestions"`
	} `json:"return_feedback"`
	Notes string `json:"notes"`
}

type CouncilVote struct {
	ReviewID   string  `json:"review_id"`
	Round      int     `json:"round"`
	Lens       string  `json:"lens"`
	ModelID    string  `json:"model_id"`
	Vote       string  `json:"vote"`
	Confidence float64 `json:"confidence"`
	Concerns   []struct {
		Severity   string `json:"severity"`
		Category   string `json:"category"`
		TaskID     string `json:"task_id"`
		Issue      string `json:"description"`
		Suggestion string `json:"suggestion"`
	} `json:"concerns"`
	Suggestions []string `json:"suggestions"`
	Reasoning   string   `json:"reasoning"`
}

type PlannerOutput struct {
	Action      string `json:"action"`
	PlanID      string `json:"plan_id"`
	PlanPath    string `json:"plan_path"`
	PlanContent string `json:"plan_content"`
	TotalTasks  int    `json:"total_tasks"`
	Status      string `json:"status"`
}

type TestResults struct {
	Action        string `json:"action"`
	TaskID        string `json:"task_id"`
	TaskNumber    string `json:"task_number"`
	TestOutcome   string `json:"test_outcome"`
	OverallResult string `json:"overall_result"`
	NextAction    string `json:"next_action"`
}

type InitialReviewDecision struct {
	Action               string   `json:"action"`
	PlanID               string   `json:"plan_id"`
	Decision             string   `json:"decision"`
	Complexity           string   `json:"complexity"`
	Reasoning            string   `json:"reasoning"`
	Concerns             []string `json:"concerns"`
	TaskCount            int      `json:"task_count"`
	TasksReviewed        []string `json:"tasks_reviewed"`
	TasksNeedingRevision []string `json:"tasks_needing_revision"`
}

type ResearchReviewDecision struct {
	Action             string              `json:"action"`
	SuggestionID       string              `json:"suggestion_id"`
	Decision           string              `json:"decision"`
	Complexity         string              `json:"complexity"`
	Reasoning          string              `json:"reasoning"`
	MaintenanceCommand *MaintenanceCommand `json:"maintenance_command,omitempty"`
	Urgency            string              `json:"urgency,omitempty"`
	Notes              string              `json:"notes,omitempty"`
}

type MaintenanceCommand struct {
	Action  string                 `json:"action"`
	Details map[string]interface{} `json:"details"`
}

func ParseResearchReview(output string) (*ResearchReviewDecision, error) {
	var r ResearchReviewDecision
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TaskRunnerOutput struct {
	TaskID   string          `json:"task_id"`
	Status   string          `json:"status"`
	Summary  string          `json:"summary"`
	FilesRaw json.RawMessage `json:"files_created,omitempty"`
	Files    []File          `json:"-"`
	TestsRaw json.RawMessage `json:"tests,omitempty"`
	Tests    TestSection     `json:"-"`
	Notes    string          `json:"notes"`
}

type TestSection struct {
	Files   []File `json:"files_created"`
	Summary string `json:"summary"`
}

func ParseSupervisorDecision(output string) (*SupervisorDecision, error) {
	var d SupervisorDecision
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, err
	}

	if len(d.IssuesRaw) > 0 {
		d.Issues = parseIssues(d.IssuesRaw)
	}

	return &d, nil
}

func parseIssues(raw json.RawMessage) []Issue {
	if len(raw) == 0 {
		return nil
	}

	var issues []Issue
	if err := json.Unmarshal(raw, &issues); err == nil {
		return issues
	}

	var issueStr string
	if err := json.Unmarshal(raw, &issueStr); err == nil && issueStr != "" {
		return []Issue{{Type: "general", Description: issueStr, Severity: "medium"}}
	}

	var issueArr []string
	if err := json.Unmarshal(raw, &issueArr); err == nil {
		for _, s := range issueArr {
			issues = append(issues, Issue{Type: "general", Description: s, Severity: "medium"})
		}
		return issues
	}

	return nil
}

func ParseCouncilVote(output string) (*CouncilVote, error) {
	var v CouncilVote
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func ParsePlannerOutput(output string) (*PlannerOutput, error) {
	var p PlannerOutput
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func ParseTestResults(output string) (*TestResults, error) {
	var t TestResults
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func ParseInitialReview(output string) (*InitialReviewDecision, error) {
	var r InitialReviewDecision
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func ParseTaskRunnerOutput(output string) (*TaskRunnerOutput, error) {
	var t TaskRunnerOutput
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(jsonStr), &t); err != nil {
		return nil, err
	}

	if len(t.FilesRaw) > 0 {
		t.Files = parseFilesArray(t.FilesRaw)
	}

	if len(t.TestsRaw) > 0 {
		var testsWrapper struct {
			Files   json.RawMessage `json:"files_created"`
			Summary string          `json:"summary"`
		}
		if err := json.Unmarshal(t.TestsRaw, &testsWrapper); err == nil {
			t.Tests.Files = parseFilesArray(testsWrapper.Files)
			t.Tests.Summary = testsWrapper.Summary
		}
	}

	return &t, nil
}

func parseFilesArray(raw json.RawMessage) []File {
	if len(raw) == 0 {
		return nil
	}

	var files []File

	var objectFiles []File
	if err := json.Unmarshal(raw, &objectFiles); err == nil {
		return objectFiles
	}

	var stringFiles []string
	if err := json.Unmarshal(raw, &stringFiles); err == nil {
		for _, path := range stringFiles {
			files = append(files, File{Path: path})
		}
		return files
	}

	return files
}

func extractJSON(output string) string {
	output = strings.TrimSpace(output)

	if strings.Contains(output, "```") {
		lines := strings.Split(output, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				if inBlock {
					break
				}
				inBlock = true
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		result := strings.Join(jsonLines, "\n")
		if result != "" {
			return result
		}
	}

	firstBrace := strings.Index(output, "{")
	lastBrace := strings.LastIndex(output, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return output[firstBrace : lastBrace+1]
	}

	return output
}

func CategorizeFailure(issueType string) string {
	switch issueType {
	case "truncation", "context_exceeded", "incomplete":
		return "model_issue"
	case "drift", "wrong_output", "unexpected_changes":
		return "quality_issue"
	case "security", "secrets", "no_hardcoded_secrets":
		return "security_issue"
	case "timeout", "rate_limited":
		return "platform_issue"
	default:
		return "task_issue"
	}
}
