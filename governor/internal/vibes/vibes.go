package vibes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Vibes struct {
	runtime *Runtime
}

type Runtime struct {
	ExecuteFunc func(ctx context.Context, roleID string, context map[string]interface{}) (*Result, error)
}

type Result struct {
	Success bool
	Output  interface{}
	Error   string
}

type StatusReport struct {
	Summary         string   `json:"summary"`
	TasksTotal      int      `json:"tasks_total"`
	TasksComplete   int      `json:"tasks_complete"`
	TasksActive     int      `json:"tasks_active"`
	TokensUsed      int64    `json:"tokens_used"`
	VirtualCost     float64  `json:"virtual_cost"`
	Issues          []Issue  `json:"issues"`
	Recommendations []string `json:"recommendations"`
}

type Issue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type ROIReport struct {
	Spent          Spending       `json:"spent"`
	Produced       Production     `json:"produced"`
	Efficiency     []ModelMetrics `json:"efficiency"`
	Recommendation string         `json:"recommendation"`
}

type Spending struct {
	Tokens int64   `json:"tokens"`
	Time   string  `json:"time"`
	Money  float64 `json:"money"`
}

type Production struct {
	TasksComplete int     `json:"tasks_complete"`
	QualityScore  float64 `json:"quality_score"`
}

type ModelMetrics struct {
	ModelID       string  `json:"model_id"`
	TasksComplete int     `json:"tasks_complete"`
	AvgDurationMs int64   `json:"avg_duration_ms"`
	TokensPerTask int64   `json:"tokens_per_task"`
	CostPerTask   float64 `json:"cost_per_task"`
	SuccessRate   float64 `json:"success_rate"`
}

type PlatformStatus struct {
	PlatformID  string  `json:"platform_id"`
	Used        int     `json:"used"`
	Limit       int     `json:"limit"`
	PercentUsed float64 `json:"percent_used"`
	ResetsIn    string  `json:"resets_in"`
	Status      string  `json:"status"`
}

type DailyBriefing struct {
	Date            string            `json:"date"`
	Completed       CompletionStats   `json:"completed"`
	Issues          []IssueResolution `json:"issues"`
	PlatformStatus  []PlatformStatus  `json:"platform_status"`
	Alerts          []Alert           `json:"alerts"`
	Recommendations []string          `json:"recommendations"`
	ResearchNote    string            `json:"research_note"`
}

type CompletionStats struct {
	TasksTotal    int      `json:"tasks_total"`
	PlatformsUsed int      `json:"platforms_used"`
	TopPerformers []string `json:"top_performers"`
	TokensBurned  int64    `json:"tokens_burned"`
	VirtualCost   float64  `json:"virtual_cost"`
}

type IssueResolution struct {
	Issue      string `json:"issue"`
	Resolution string `json:"resolution"`
}

type Alert struct {
	Type     string `json:"type"`
	Platform string `json:"platform"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

type ConsultationInput struct {
	Question    string
	Context     map[string]interface{}
	History     []Message
	RequestType string
}

type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type ConsultationResponse struct {
	Answer        string   `json:"answer"`
	DataNeeded    []string `json:"data_needed,omitempty"`
	FollowUp      string   `json:"follow_up,omitempty"`
	RequiresHuman bool     `json:"requires_human"`
}

func New(runtime *Runtime) *Vibes {
	return &Vibes{
		runtime: runtime,
	}
}

func (v *Vibes) GetStatus(ctx context.Context, projectID string) (*StatusReport, error) {
	context := map[string]interface{}{
		"task":       "status",
		"project_id": projectID,
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("status failed: %s", result.Error)
	}

	var report StatusReport
	if err := v.parseResult(result, &report); err != nil {
		outputStr, _ := result.Output.(string)
		return v.parseStatusFromText(outputStr), nil
	}

	return &report, nil
}

func (v *Vibes) GetROI(ctx context.Context, projectID string, period string) (*ROIReport, error) {
	context := map[string]interface{}{
		"task":       "roi",
		"project_id": projectID,
		"period":     period,
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("roi failed: %s", result.Error)
	}

	var report ROIReport
	if err := v.parseResult(result, &report); err != nil {
		outputStr, _ := result.Output.(string)
		return v.parseROIFromText(outputStr), nil
	}

	return &report, nil
}

func (v *Vibes) GetDailyBriefing(ctx context.Context) (*DailyBriefing, error) {
	context := map[string]interface{}{
		"task": "daily_briefing",
		"date": time.Now().Format("2006-01-02"),
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("briefing failed: %s", result.Error)
	}

	var briefing DailyBriefing
	if err := v.parseResult(result, &briefing); err != nil {
		outputStr, _ := result.Output.(string)
		return v.parseBriefingFromText(outputStr), nil
	}

	return &briefing, nil
}

func (v *Vibes) CheckPlatformLimits(ctx context.Context) ([]PlatformStatus, error) {
	context := map[string]interface{}{
		"task": "platform_limits",
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("platform check failed: %s", result.Error)
	}

	var statuses []PlatformStatus
	if err := v.parseResult(result, &statuses); err != nil {
		outputStr, _ := result.Output.(string)
		return v.parsePlatformsFromText(outputStr), nil
	}

	return statuses, nil
}

func (v *Vibes) Consult(ctx context.Context, input *ConsultationInput) (*ConsultationResponse, error) {
	context := map[string]interface{}{
		"task":         "consult",
		"question":     input.Question,
		"request_type": input.RequestType,
		"history":      input.History,
	}

	for k, val := range input.Context {
		context[k] = val
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("consultation failed: %s", result.Error)
	}

	var response ConsultationResponse
	if err := v.parseResult(result, &response); err != nil {
		outputStr, _ := result.Output.(string)
		return &ConsultationResponse{
			Answer:        outputStr,
			RequiresHuman: true,
		}, nil
	}

	return &response, nil
}

func (v *Vibes) ShouldSwapModel(ctx context.Context, currentModel, candidateModel string, taskType string) (*ConsultationResponse, error) {
	context := map[string]interface{}{
		"task":            "model_swap_analysis",
		"current_model":   currentModel,
		"candidate_model": candidateModel,
		"task_type":       taskType,
	}

	result, err := v.runtime.ExecuteFunc(ctx, "vibes", context)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("analysis failed: %s", result.Error)
	}

	var response ConsultationResponse
	if err := v.parseResult(result, &response); err != nil {
		outputStr, _ := result.Output.(string)
		return &ConsultationResponse{
			Answer:        outputStr,
			RequiresHuman: true,
		}, nil
	}

	return &response, nil
}

func (v *Vibes) AlertThreshold() float64 {
	return 0.80
}

func (v *Vibes) IsAlertNeeded(status *PlatformStatus) bool {
	return status.PercentUsed >= v.AlertThreshold()
}

func (v *Vibes) parseResult(result *Result, target interface{}) error {
	outputStr, ok := result.Output.(string)
	if !ok {
		return fmt.Errorf("output is not string")
	}

	jsonStr := v.extractJSON(outputStr)
	if jsonStr == "" {
		return fmt.Errorf("no JSON found")
	}

	return json.Unmarshal([]byte(jsonStr), target)
}

func (v *Vibes) extractJSON(output string) string {
	output = strings.TrimSpace(output)

	if (strings.HasPrefix(output, "{") && strings.HasSuffix(output, "}")) ||
		(strings.HasPrefix(output, "[") && strings.HasSuffix(output, "]")) {
		return output
	}

	codeBlockIdx := strings.Index(output, "```")
	if codeBlockIdx == -1 {
		braceStart := strings.Index(output, "{")
		braceEnd := strings.LastIndex(output, "}")
		if braceStart != -1 && braceEnd != -1 && braceEnd > braceStart {
			return output[braceStart : braceEnd+1]
		}
		return ""
	}

	afterBlock := output[codeBlockIdx+3:]

	newlineIdx := strings.Index(afterBlock, "\n")
	if newlineIdx != -1 {
		afterBlock = afterBlock[newlineIdx+1:]
	}

	blockEnd := strings.Index(afterBlock, "```")
	if blockEnd == -1 {
		return ""
	}

	return strings.TrimSpace(afterBlock[:blockEnd])
}

func (v *Vibes) parseStatusFromText(text string) *StatusReport {
	report := &StatusReport{
		Summary:         "",
		Issues:          []Issue{},
		Recommendations: []string{},
	}

	report.Summary = v.extractSection(text, "summary", "status")

	return report
}

func (v *Vibes) parseROIFromText(text string) *ROIReport {
	report := &ROIReport{
		Recommendation: v.extractSection(text, "recommendation", "conclusion"),
	}

	return report
}

func (v *Vibes) parseBriefingFromText(text string) *DailyBriefing {
	briefing := &DailyBriefing{
		Date:            time.Now().Format("2006-01-02"),
		Issues:          []IssueResolution{},
		PlatformStatus:  []PlatformStatus{},
		Alerts:          []Alert{},
		Recommendations: []string{},
		ResearchNote:    v.extractSection(text, "research", "note"),
	}

	return briefing
}

func (v *Vibes) parsePlatformsFromText(text string) []PlatformStatus {
	return []PlatformStatus{}
}

func (v *Vibes) extractSection(text, keyword1, keyword2 string) string {
	lines := strings.Split(text, "\n")
	var collecting bool
	var sectionLines []string

	for _, line := range lines {
		lowerLine := strings.ToLower(line)

		if strings.Contains(lowerLine, keyword1) || strings.Contains(lowerLine, keyword2) {
			collecting = true
			continue
		}

		if collecting {
			if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.HasPrefix(strings.TrimSpace(line), "##") {
				break
			}
			if strings.TrimSpace(line) != "" {
				sectionLines = append(sectionLines, strings.TrimSpace(line))
			}
		}
	}

	return strings.Join(sectionLines, " ")
}
