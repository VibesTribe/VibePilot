package main

type RecoveryConfig struct {
	OrphanThresholdSeconds int
	MaxTaskAttempts        int
	ModelFailureThreshold  int
}

type TaskData struct {
	TaskNumber       string
	Title            string
	Type             string
	Confidence       float64
	Category         string
	Dependencies     []string
	RequiresCodebase bool
	PromptPacket     string
	ExpectedOutput   string
}

type ValidationError struct {
	TaskNumber string
	Issue      string
	Severity   string
}

func (e *ValidationError) Error() string {
	return "task " + e.TaskNumber + ": " + e.Issue
}

type ValidationFailedError struct {
	Errors []ValidationError
}

func (e *ValidationFailedError) Error() string {
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Issue+" ("+err.TaskNumber+")")
	}
	return msgs[0]
}
