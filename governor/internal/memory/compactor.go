package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

// SessionSummary is a compressed record of what happened during an agent session.
type SessionSummary struct {
	SessionID   string            `json:"session_id"`
	TaskID      string            `json:"task_id,omitempty"`
	AgentID     string            `json:"agent_id"`
	ConnectorID string            `json:"connector_id"`
	Action      string            `json:"action"`       // approve, reject, revise, etc.
	Outcome     string            `json:"outcome"`       // 1-2 sentence summary
	KeyDecisions []string         `json:"key_decisions"` // important decisions made
	FilesChanged []string         `json:"files_changed,omitempty"`
	IssuesFound []string          `json:"issues_found,omitempty"`
	TokensIn    int               `json:"tokens_in"`
	TokensOut   int               `json:"tokens_out"`
	DurationMs  int64             `json:"duration_ms"`
	Success     bool              `json:"success"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Compactor compresses session results into summaries and stores them.
type Compactor struct {
	db db.Database
}

// NewCompactor creates a new session compactor.
func NewCompactor(dbClient db.Database) *Compactor {
	return &Compactor{db: dbClient}
}

// Compact compresses a session result into a summary and stores it.
// It extracts key information from the typed decision structs.
func (c *Compactor) Compact(ctx context.Context, result *runtime.SessionResult, taskID string) (*SessionSummary, error) {
	summary := &SessionSummary{
		SessionID:   fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		TaskID:      taskID,
		AgentID:     result.AgentID,
		ConnectorID: result.ConnectorID,
		TokensIn:    result.TokensIn,
		TokensOut:   result.TokensOut,
		DurationMs:  result.Duration.Milliseconds(),
		Success:     true,
		Timestamp:   time.Now(),
	}

	// Parse the output based on agent type to extract structured info
	c.extractFromOutput(result.Output, summary)

	// Store in short-term memory (session context)
	if c.db != nil {
		summaryJSON, _ := json.Marshal(summary)
		_, err := c.db.RPC(ctx, "store_memory", map[string]interface{}{
			"p_layer":   "short_term",
			"p_key":     fmt.Sprintf("session:%s:%s", summary.AgentID, summary.SessionID),
			"p_value":   string(summaryJSON),
			"p_ttl_sec": 3600, // 1 hour TTL for session summaries
		})
		if err != nil {
			// Non-fatal: compaction shouldn't break the session flow
			fmt.Printf("[Compactor] Warning: failed to store summary: %v\n", err)
		}
	}

	return summary, nil
}

// CompactSession implements runtime.SessionCompactor.
// Non-blocking: logs errors but never fails.
func (c *Compactor) CompactSession(ctx context.Context, result *runtime.SessionResult, taskID string) {
	_, err := c.Compact(ctx, result, taskID)
	if err != nil {
		fmt.Printf("[Compactor] Warning: compaction failed: %v\n", err)
	}
}

// extractFromOutput parses the raw agent output and fills in summary fields.
// It tries multiple decision types since we don't know which agent produced the output.
func (c *Compactor) extractFromOutput(output string, summary *SessionSummary) {
	// Try SupervisorDecision
	if d, err := runtime.ParseSupervisorDecision(output); err == nil {
		summary.Action = d.Decision
		summary.Outcome = d.ReturnFeedback.Summary
		if summary.Outcome == "" {
			summary.Outcome = d.Notes
		}
		for _, issue := range d.Issues {
			summary.IssuesFound = append(summary.IssuesFound, issue.Description)
		}
		summary.KeyDecisions = append(summary.KeyDecisions, d.NextAction)
		return
	}

	// Try TaskRunnerOutput
	if t, err := runtime.ParseTaskRunnerOutput(output); err == nil {
		summary.Action = t.Status
		summary.Outcome = t.Summary
		for _, f := range t.Files {
			summary.FilesChanged = append(summary.FilesChanged, f.Path)
		}
		return
	}

	// Try PlannerOutput
	if p, err := runtime.ParsePlannerOutput(output); err == nil {
		summary.Action = p.Action
		summary.Outcome = fmt.Sprintf("Plan %s with %d tasks, status: %s", p.PlanID, p.TotalTasks, p.Status)
		return
	}

	// Try CouncilVote
	if v, err := runtime.ParseCouncilVote(output); err == nil {
		summary.Action = v.Vote
		summary.Outcome = v.Reasoning
		for _, c := range v.Concerns {
			summary.IssuesFound = append(summary.IssuesFound, c.Issue)
		}
		return
	}

	// Try TestResults
	if tr, err := runtime.ParseTestResults(output); err == nil {
		summary.Action = tr.TestOutcome
		summary.Outcome = tr.OverallResult
		summary.Success = tr.OverallResult == "pass" || tr.OverallResult == "passed"
		return
	}

	// Try InitialReviewDecision
	if r, err := runtime.ParseInitialReview(output); err == nil {
		summary.Action = r.Decision
		summary.Outcome = r.Reasoning
		summary.KeyDecisions = append(summary.KeyDecisions, fmt.Sprintf("Complexity: %s, Tasks: %d", r.Complexity, r.TaskCount))
		return
	}

	// Try ResearchReviewDecision
	if rr, err := runtime.ParseResearchReview(output); err == nil {
		summary.Action = rr.Decision
		summary.Outcome = rr.Reasoning
		return
	}

	// Fallback: raw output truncated
	raw := strings.TrimSpace(output)
	if len(raw) > 500 {
		raw = raw[:500] + "..."
	}
	summary.Outcome = raw
}

// GetRecentSummaries retrieves recent session summaries for context building.
func (c *Compactor) GetRecentSummaries(ctx context.Context, agentID string, limit int) ([]SessionSummary, error) {
	if c.db == nil {
		return nil, nil
	}

	result, err := c.db.RPC(ctx, "recall_memories", map[string]interface{}{
		"p_layer": "short_term",
		"p_query": fmt.Sprintf("session:%s:", agentID),
		"p_limit": limit,
	})
	if err != nil {
		return nil, err
	}

	var records []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &records); err != nil {
		return nil, err
	}

	var summaries []SessionSummary
	for _, rec := range records {
		var s SessionSummary
		if err := json.Unmarshal([]byte(rec.Value), &s); err == nil {
			summaries = append(summaries, s)
		}
	}

	return summaries, nil
}

// BuildCompactionContext creates a compressed context string from recent summaries.
// This gets prepended to agent prompts so they know what happened recently.
func (c *Compactor) BuildCompactionContext(ctx context.Context, taskID string, maxSummaries int) (string, error) {
	if c.db == nil {
		return "", nil
	}

	// Get recent summaries for this task
	result, err := c.db.RPC(ctx, "recall_memories", map[string]interface{}{
		"p_layer": "short_term",
		"p_query": "session:",
		"p_limit": maxSummaries,
	})
	if err != nil {
		return "", err
	}

	var records []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &records); err != nil {
		return "", err
	}

	var summaries []SessionSummary
	for _, rec := range records {
		var s SessionSummary
		if err := json.Unmarshal([]byte(rec.Value), &s); err == nil {
			// Filter to only summaries for this task
			if taskID == "" || s.TaskID == taskID {
				summaries = append(summaries, s)
			}
		}
	}

	if len(summaries) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("## Recent Session History (compacted)\n\n")
	for _, s := range summaries {
		b.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", s.AgentID, s.Action, s.Outcome))
		if len(s.IssuesFound) > 0 {
			b.WriteString(fmt.Sprintf("  Issues: %s\n", strings.Join(s.IssuesFound, "; ")))
		}
		if len(s.FilesChanged) > 0 {
			b.WriteString(fmt.Sprintf("  Files: %s\n", strings.Join(s.FilesChanged, ", ")))
		}
	}
	b.WriteString("\n")

	return b.String(), nil
}
