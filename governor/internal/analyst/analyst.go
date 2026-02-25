package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/db"
)

type Analyst struct {
	db       *db.DB
	config   config.AnalystConfig
	ghConfig config.GitHubConfig
	repoPath string
}

type AnalysisReport struct {
	Date             string            `json:"date"`
	ModelUpdates     []ModelUpdate     `json:"model_updates,omitempty"`
	HeuristicUpdates []HeuristicUpdate `json:"heuristic_updates,omitempty"`
	RuleUpdates      []RuleUpdate      `json:"rule_updates,omitempty"`
	Summary          string            `json:"summary"`
	Recommendations  []string          `json:"recommendations,omitempty"`
}

type ModelUpdate struct {
	ModelID string `json:"model_id"`
	Action  string `json:"action"` // "boost", "archive", "revive"
	Reason  string `json:"reason"`
}

type HeuristicUpdate struct {
	TaskType       string                 `json:"task_type,omitempty"`
	Condition      map[string]interface{} `json:"condition,omitempty"`
	PreferredModel string                 `json:"preferred_model"`
	Confidence     float64                `json:"confidence"`
	Reason         string                 `json:"reason"`
}

type RuleUpdate struct {
	RuleType string `json:"rule_type"` // "planner", "tester", "supervisor"
	RuleID   string `json:"rule_id"`
	Action   string `json:"action"` // "deactivate"
	Reason   string `json:"reason"`
}

func New(database *db.DB, cfg config.AnalystConfig, ghCfg config.GitHubConfig, repoPath string) *Analyst {
	return &Analyst{
		db:       database,
		config:   cfg,
		ghConfig: ghCfg,
		repoPath: repoPath,
	}
}

func (a *Analyst) Run(ctx context.Context) {
	if !a.config.Enabled {
		log.Println("Analyst: disabled, skipping")
		return
	}

	schedule := a.parseSchedule()
	durationUntil := a.timeUntil(schedule)

	log.Printf("Analyst: scheduled for %s (in %v)", schedule.Format(time.RFC3339), durationUntil)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	var lastRunDate string

	for {
		select {
		case <-ctx.Done():
			log.Println("Analyst shutting down")
			return
		case <-ticker.C:
			now := time.Now().UTC()
			today := now.Format("2006-01-02")

			if today == lastRunDate {
				continue
			}

			scheduledTime := a.parseSchedule()
			scheduledToday := time.Date(now.Year(), now.Month(), now.Day(),
				scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, time.UTC)

			if now.After(scheduledToday) || now.Equal(scheduledToday) {
				log.Println("Analyst: starting daily analysis")
				if err := a.runAnalysis(ctx); err != nil {
					log.Printf("Analyst: analysis failed: %v", err)
				} else {
					lastRunDate = today
					log.Println("Analyst: daily analysis complete")
				}
			}
		}
	}
}

func (a *Analyst) parseSchedule() time.Time {
	parts := strings.Split(a.config.Schedule, ":")
	hour, minute := 0, 0
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &hour)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minute)
	}
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
}

func (a *Analyst) timeUntil(target time.Time) time.Duration {
	now := time.Now().UTC()
	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}
	return target.Sub(now)
}

func (a *Analyst) runAnalysis(ctx context.Context) error {
	data, err := a.gatherData(ctx)
	if err != nil {
		return fmt.Errorf("gather data: %w", err)
	}

	prompt := a.buildPrompt(data)

	output, err := a.callLLM(ctx, prompt)
	if err != nil {
		return fmt.Errorf("call LLM: %w", err)
	}

	report, err := a.parseOutput(output)
	if err != nil {
		log.Printf("Analyst: warning - could not parse LLM output: %v", err)
		report = &AnalysisReport{
			Date:    time.Now().UTC().Format("2006-01-02"),
			Summary: output,
		}
	}

	if err := a.applyUpdates(ctx, report); err != nil {
		log.Printf("Analyst: warning - could not apply all updates: %v", err)
	}

	if err := a.writeReport(ctx, report); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

type AnalysisData struct {
	TaskRuns        []map[string]interface{} `json:"task_runs"`
	FailureRecords  []map[string]interface{} `json:"failure_records"`
	Heuristics      []map[string]interface{} `json:"heuristics"`
	Runners         []map[string]interface{} `json:"runners"`
	PlannerRules    []map[string]interface{} `json:"planner_rules"`
	TesterRules     []map[string]interface{} `json:"tester_rules"`
	SupervisorRules []map[string]interface{} `json:"supervisor_rules"`
}

func (a *Analyst) gatherData(ctx context.Context) (*AnalysisData, error) {
	data := &AnalysisData{}

	runsData, err := a.db.REST(ctx, "GET", "task_runs?select=*&order=created_at.desc&limit=100", nil)
	if err == nil {
		json.Unmarshal(runsData, &data.TaskRuns)
	}

	failuresData, err := a.db.REST(ctx, "GET", "failure_records?select=*&order=created_at.desc&limit=50", nil)
	if err == nil {
		json.Unmarshal(failuresData, &data.FailureRecords)
	}

	heuristicsData, err := a.db.REST(ctx, "GET", "learned_heuristics?select=*", nil)
	if err == nil {
		json.Unmarshal(heuristicsData, &data.Heuristics)
	}

	runnersData, err := a.db.REST(ctx, "GET", "runners?select=id,model_id,status,depreciation_score,task_ratings", nil)
	if err == nil {
		json.Unmarshal(runnersData, &data.Runners)
	}

	plannerRulesData, err := a.db.REST(ctx, "GET", "planner_learned_rules?select=*&active=eq.true&limit=50", nil)
	if err == nil {
		json.Unmarshal(plannerRulesData, &data.PlannerRules)
	}

	testerRulesData, err := a.db.REST(ctx, "GET", "tester_learned_rules?select=*&active=eq.true&limit=50", nil)
	if err == nil {
		json.Unmarshal(testerRulesData, &data.TesterRules)
	}

	supervisorRulesData, err := a.db.REST(ctx, "GET", "supervisor_learned_rules?select=*&active=eq.true&limit=50", nil)
	if err == nil {
		json.Unmarshal(supervisorRulesData, &data.SupervisorRules)
	}

	return data, nil
}

func (a *Analyst) buildPrompt(data *AnalysisData) string {
	dataJSON, _ := json.MarshalIndent(data, "", "  ")

	return fmt.Sprintf(`You are the VibePilot Daily Analyst. Analyze the following system data and provide recommendations.

DATA:
%s

Analyze and respond in JSON format:
{
  "summary": "Brief summary of what you observed",
  "model_updates": [
    {"model_id": "xxx", "action": "boost|archive|revive", "reason": "why"}
  ],
  "heuristic_updates": [
    {"task_type": "coding", "preferred_model": "xxx", "confidence": 0.8, "reason": "why"}
  ],
  "rule_updates": [
    {"rule_type": "planner|tester|supervisor", "rule_id": "xxx", "action": "deactivate", "reason": "why"}
  ],
  "recommendations": ["recommendation 1", "recommendation 2"]
}

Rules:
- Only recommend actions you're confident about (success_rate patterns, consistent failures)
- Confidence should be 0.5-1.0 based on data strength
- Be conservative - wrong recommendations hurt the system
- For rule_updates: deactivate rules with high false_positive rates or low effectiveness
- If data is insufficient, just provide summary and skip recommendations

Respond ONLY with valid JSON.`, string(dataJSON))
}

func (a *Analyst) callLLM(ctx context.Context, prompt string) (string, error) {
	dest, err := a.db.GetDestination(ctx, "opencode")
	if err != nil {
		dest, err = a.db.GetDestination(ctx, "gemini-api")
		if err != nil {
			return "", fmt.Errorf("no available destination: %w", err)
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if dest.Type == "cli" {
		cmd := exec.CommandContext(timeoutCtx, dest.Command, "run", "--format", "json", prompt)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%s: %w - %s", dest.Command, err, string(output))
		}

		var result struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			return string(output), nil
		}

		return result.Content, nil
	}

	return "", fmt.Errorf("destination type %s not supported for analyst", dest.Type)
}

func (a *Analyst) parseOutput(output string) (*AnalysisReport, error) {
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("no JSON object found in output")
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var report AnalysisReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if report.Date == "" {
		report.Date = time.Now().UTC().Format("2006-01-02")
	}

	return &report, nil
}

func (a *Analyst) applyUpdates(ctx context.Context, report *AnalysisReport) error {
	for _, update := range report.ModelUpdates {
		switch update.Action {
		case "archive":
			if err := a.db.ArchiveRunner(ctx, update.ModelID, update.Reason); err != nil {
				log.Printf("Analyst: failed to archive %s: %v", update.ModelID, err)
			} else {
				log.Printf("Analyst: archived %s - %s", update.ModelID, update.Reason)
			}
		case "boost":
			if err := a.db.BoostRunner(ctx, update.ModelID); err != nil {
				log.Printf("Analyst: failed to boost %s: %v", update.ModelID, err)
			} else {
				log.Printf("Analyst: boosted %s - %s", update.ModelID, update.Reason)
			}
		case "revive":
			if err := a.db.ReviveRunner(ctx, update.ModelID, update.Reason); err != nil {
				log.Printf("Analyst: failed to revive %s: %v", update.ModelID, err)
			} else {
				log.Printf("Analyst: revived %s - %s", update.ModelID, update.Reason)
			}
		}
	}

	for _, h := range report.HeuristicUpdates {
		_, err := a.db.UpsertHeuristic(ctx, h.TaskType, h.Condition, h.PreferredModel, nil, h.Confidence, "daily_analysis")
		if err != nil {
			log.Printf("Analyst: failed to upsert heuristic: %v", err)
		} else {
			log.Printf("Analyst: updated heuristic for %s -> %s", h.TaskType, h.PreferredModel)
		}
	}

	for _, r := range report.RuleUpdates {
		switch r.RuleType {
		case "planner":
			if err := a.db.DeactivatePlannerRule(ctx, r.RuleID, r.Reason); err != nil {
				log.Printf("Analyst: failed to deactivate planner rule %s: %v", r.RuleID, err)
			} else {
				log.Printf("Analyst: deactivated planner rule %s - %s", r.RuleID, r.Reason)
			}
		case "tester":
			if err := a.db.DeactivateTesterRule(ctx, r.RuleID, r.Reason); err != nil {
				log.Printf("Analyst: failed to deactivate tester rule %s: %v", r.RuleID, err)
			} else {
				log.Printf("Analyst: deactivated tester rule %s - %s", r.RuleID, r.Reason)
			}
		case "supervisor":
			if err := a.db.DeactivateSupervisorRule(ctx, r.RuleID, r.Reason); err != nil {
				log.Printf("Analyst: failed to deactivate supervisor rule %s: %v", r.RuleID, err)
			} else {
				log.Printf("Analyst: deactivated supervisor rule %s - %s", r.RuleID, r.Reason)
			}
		}
	}

	return nil
}

func (a *Analyst) writeReport(ctx context.Context, report *AnalysisReport) error {
	if a.repoPath == "" {
		return nil
	}

	reportsDir := filepath.Join(a.repoPath, "docs", "analysis")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}

	filename := fmt.Sprintf("daily_%s.md", report.Date)
	filepath := filepath.Join(reportsDir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Daily Analysis - %s\n\n", report.Date))
	sb.WriteString("## Summary\n\n")
	sb.WriteString(report.Summary + "\n\n")

	if len(report.ModelUpdates) > 0 {
		sb.WriteString("## Model Updates\n\n")
		for _, u := range report.ModelUpdates {
			sb.WriteString(fmt.Sprintf("- **%s**: %s - %s\n", u.Action, u.ModelID, u.Reason))
		}
		sb.WriteString("\n")
	}

	if len(report.HeuristicUpdates) > 0 {
		sb.WriteString("## Heuristic Updates\n\n")
		for _, h := range report.HeuristicUpdates {
			sb.WriteString(fmt.Sprintf("- **%s**: prefer %s (confidence: %.2f) - %s\n",
				h.TaskType, h.PreferredModel, h.Confidence, h.Reason))
		}
		sb.WriteString("\n")
	}

	if len(report.RuleUpdates) > 0 {
		sb.WriteString("## Rule Updates\n\n")
		for _, r := range report.RuleUpdates {
			sb.WriteString(fmt.Sprintf("- **%s rule** %s: %s - %s\n", r.RuleType, r.RuleID[:8], r.Action, r.Reason))
		}
		sb.WriteString("\n")
	}

	if len(report.Recommendations) > 0 {
		sb.WriteString("## Recommendations\n\n")
		for _, r := range report.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", r))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n*Generated by VibePilot Daily Analyst*\n")

	if err := os.WriteFile(filepath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", a.repoPath, "checkout", "research-considerations")
	if err := cmd.Run(); err != nil {
		cmd = exec.CommandContext(ctx, "git", "-C", a.repoPath, "checkout", "-b", "research-considerations")
		cmd.Run()
	}

	exec.CommandContext(ctx, "git", "-C", a.repoPath, "pull", "origin", "research-considerations").Run()
	exec.CommandContext(ctx, "git", "-C", a.repoPath, "add", filepath).Run()

	commitCmd := exec.CommandContext(ctx, "git", "-C", a.repoPath, "commit", "-m", fmt.Sprintf("Analysis: %s daily report", report.Date))
	if err := commitCmd.Run(); err != nil {
		log.Printf("Analyst: git commit (may be no changes): %v", err)
		return nil
	}

	pushCmd := exec.CommandContext(ctx, "git", "-C", a.repoPath, "push", "origin", "research-considerations")
	if err := pushCmd.Run(); err != nil {
		log.Printf("Analyst: git push: %v", err)
	}

	return nil
}
