package runtime

import "encoding/json"

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
	Issues []struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
	} `json:"issues"`
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
	Action      string `json:"action"`
	TaskID      string `json:"task_id"`
	TaskNumber  string `json:"task_number"`
	TestOutcome string `json:"test_outcome"`
	NextAction  string `json:"next_action"`
}

type InitialReviewDecision struct {
	Action     string   `json:"action"`
	PlanID     string   `json:"plan_id"`
	Decision   string   `json:"decision"`
	Complexity string   `json:"complexity"`
	Reasoning  string   `json:"reasoning"`
	Concerns   []string `json:"concerns"`
	TaskCount  int      `json:"task_count"`
}

func ParseSupervisorDecision(output string) (*SupervisorDecision, error) {
	var d SupervisorDecision
	if err := json.Unmarshal([]byte(output), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func ParseCouncilVote(output string) (*CouncilVote, error) {
	var v CouncilVote
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func ParsePlannerOutput(output string) (*PlannerOutput, error) {
	var p PlannerOutput
	if err := json.Unmarshal([]byte(output), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func ParseTestResults(output string) (*TestResults, error) {
	var t TestResults
	if err := json.Unmarshal([]byte(output), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func ParseInitialReview(output string) (*InitialReviewDecision, error) {
	var r InitialReviewDecision
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return nil, err
	}
	return &r, nil
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
