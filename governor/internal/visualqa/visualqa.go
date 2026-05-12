package visualqa

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Config struct {
	Enabled                  bool         `json:"enabled"`
	CapturePages             []PageConfig `json:"capture_pages"`
	BaselineDir              string       `json:"baseline_dir"`
	ManifestFile             string       `json:"manifest_file"`
	ConnectorID              string       `json:"connector_id"`
	Model                    string       `json:"model"`
	CaptureTimeoutSeconds    int          `json:"capture_timeout_seconds"`
	ComparisonTimeoutSeconds int          `json:"comparison_timeout_seconds"`
	AutoApproveFirstBaseline bool         `json:"auto_approve_first_baseline"`
	GitCommitBaselines       bool         `json:"git_commit_baselines"`
	TempDir                  string       `json:"temp_dir"`
	RepoPath                 string       `json:"repo_path"`
	GeminiVaultKey           string       `json:"gemini_vault_key"`
	FixConfig                FixConfig    `json:"fix_config"`
}

type PageConfig struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	Viewports []int  `json:"viewports"`
}

type ComparisonResult struct {
	Passed      bool         `json:"passed"`
	Confidence  float64      `json:"confidence"`
	Summary     string       `json:"summary"`
	Differences []Difference `json:"differences"`
}

type Difference struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Region      string `json:"region"`
	Description string `json:"description"`
}

type RunResult struct {
	RunID        string       `json:"run_id"`
	TriggeredBy  string       `json:"triggered_by"`
	PagesChecked int          `json:"pages_checked"`
	PagesPassed  int          `json:"pages_passed"`
	PagesFailed  int          `json:"pages_failed"`
	IssuesFound  int          `json:"issues_found"`
	PageResults  []PageResult `json:"page_results"`
	DurationMs   int          `json:"duration_ms"`
}

type PageResult struct {
	PageName    string       `json:"page_name"`
	Viewport    int          `json:"viewport"`
	URL         string       `json:"url"`
	Passed      bool         `json:"passed"`
	Confidence  float64      `json:"confidence"`
	Summary     string       `json:"summary"`
	Differences []Difference `json:"differences"`
	CapturePath string       `json:"capture_path"`
	BaselineNew bool         `json:"baseline_new"`
	LoadTimeMs  int          `json:"load_time_ms"`
	Title       string       `json:"title"`
}

// DB is the interface the visual QA package needs for persistence.
type DB interface {
	Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
}

// VisualQA is the main visual regression testing engine.
type VisualQA struct {
	config     Config
	apiKey     string
	db         DB
	lastReport *UICaptureReport // set during captureScreenshot, read by Run
	fixEngine  *FixEngine
}

// NewVisualQA creates a new VisualQA instance.
func NewVisualQA(config Config, apiKey string, db DB) *VisualQA {
	if db == nil {
		fmt.Println("[VisualQA] WARNING: DB interface is nil.")
	}
	vqa := &VisualQA{
		config:  config,
		apiKey:  apiKey,
		db:      db,
	}
	// Initialize fix engine if configured
	if config.FixConfig.Enabled {
		vqa.fixEngine = NewFixEngine(config.FixConfig)
		if config.FixConfig.VisionAPIKey == "" && apiKey != "" {
			vqa.fixEngine.visionAPIKey = apiKey
		}
		fmt.Printf("[VisualQA] Fix engine enabled: mode=%s, source=%s\n", config.FixConfig.Mode, config.FixConfig.SourceRoot)
	}
	return vqa
}

// Run executes a full visual QA cycle: capture screenshots, compare to baselines, persist results.
func (v *VisualQA) Run(ctx context.Context, triggeredBy, triggerDetail string) (RunResult, error) {
	if !v.config.Enabled {
		return RunResult{}, fmt.Errorf("[VisualQA] Visual QA is disabled in config")
	}

	runID := uuid.New().String()
	startedAt := time.Now()

	// Create temp dir for this run
	tempRunDir := v.config.TempDir + "/" + runID
	if err := ensureDir(tempRunDir); err != nil {
		return RunResult{}, fmt.Errorf("[VisualQA] Failed to create temp directory %s: %w", tempRunDir, err)
	}
	defer removeDir(tempRunDir)

	// Load false positive patterns from past feedback
	falsePositives := v.GetFalsePositivePatterns(ctx)
	if len(falsePositives) > 0 {
		fmt.Printf("[VisualQA] Loaded %d false-positive patterns from feedback\n", len(falsePositives))
	}

	// Insert initial run record into DB
	initialResultsJSON, _ := json.Marshal([]PageResult{})
	_, err := v.db.Exec(ctx, `
		INSERT INTO visual_qa_runs (id, triggered_by, trigger_detail, status, pages_checked, pages_passed, pages_failed, results, error_message, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, runID, triggeredBy, triggerDetail, "running", 0, 0, 0, initialResultsJSON, "", startedAt)
	if err != nil {
		return RunResult{}, fmt.Errorf("[VisualQA] Failed to insert initial visual_qa_run: %w", err)
	}

	var pageResults []PageResult
	var allIssues []UIIssue // collected across all pages for fix engine
	pagesChecked := 0
	pagesPassed := 0
	pagesFailed := 0
	totalIssuesFound := 0
	var overallError error

	for _, pageConfig := range v.config.CapturePages {
		for _, viewportWidth := range pageConfig.Viewports {
			pagesChecked++
			pageResult := PageResult{
				PageName: pageConfig.Name,
				Viewport: viewportWidth,
			}

			captureOutputPath := fmt.Sprintf("%s/%s_%d_current.png", tempRunDir, pageConfig.Name, viewportWidth)
			fmt.Printf("[VisualQA] Capturing screenshot for %s at %dpx...\n", pageConfig.Name, viewportWidth)

			captureRes, err := v.captureScreenshot(ctx, pageConfig.URL, captureOutputPath, viewportWidth)
			if err != nil {
				fmt.Printf("[VisualQA] Error capturing screenshot for %s at %dpx: %v\n", pageConfig.Name, viewportWidth, err)
				overallError = err
				pageResult.Passed = false
				pageResult.Summary = fmt.Sprintf("Screenshot capture failed: %v", err)
				pagesFailed++
				pageResults = append(pageResults, pageResult)
				continue
			}
			pageResult.CapturePath = captureRes.Path
			pageResult.URL = pageConfig.URL
			// Pull UI interaction data from capture report
			var captureIssues []UIIssue
			if v.lastReport != nil {
				captureIssues = v.lastReport.Issues
				pageResult.Title = v.lastReport.Title
				pageResult.LoadTimeMs = v.lastReport.LoadTimeMs
				if len(v.lastReport.ConsoleErrors) > 0 {
					fmt.Printf("[VisualQA] Console errors on %s at %dpx: %d\n", pageConfig.Name, viewportWidth, len(v.lastReport.ConsoleErrors))
				}
			}

			baselinePath := v.GetBaselinePath(pageConfig.Name, viewportWidth)
			baselineExists := fileExists(baselinePath)

			if !baselineExists && v.config.AutoApproveFirstBaseline {
				fmt.Printf("[VisualQA] No baseline found for %s at %dpx. Auto-approving new baseline.\n", pageConfig.Name, viewportWidth)
				err := v.SaveBaseline(ctx, pageConfig.Name, viewportWidth, captureOutputPath)
				if err != nil {
					fmt.Printf("[VisualQA] Error saving auto-approved baseline for %s at %dpx: %v\n", pageConfig.Name, viewportWidth, err)
					overallError = err
					pageResult.Passed = false
					pageResult.Summary = fmt.Sprintf("Auto-approval failed: %v", err)
					pagesFailed++
				} else {
					pageResult.Passed = true
					pageResult.BaselineNew = true
					pageResult.Summary = "Automatically approved new baseline."
					pagesPassed++
				}

				// Run visual audit even for new baselines to catch pre-existing issues
				fmt.Printf("[VisualQA] Running visual audit for %s at %dpx...\n", pageConfig.Name, viewportWidth)
				auditRes, auditErr := v.auditImage(ctx, captureOutputPath, viewportWidth, nil)
				if auditErr != nil {
					fmt.Printf("[VisualQA] Visual audit failed for %s at %dpx: %v\n", pageConfig.Name, viewportWidth, auditErr)
				} else if len(auditRes.Issues) > 0 {
					fmt.Printf("[VisualQA] Audit found %d issues for %s at %dpx (severity: %s)\n",
						len(auditRes.Issues), pageConfig.Name, viewportWidth, auditRes.Severity)
					for _, ai := range auditRes.Issues {
						captureIssues = append(captureIssues, UIIssue{
							Type:        ai.Type,
							Severity:    ai.Severity,
							Description: ai.Description + " Suggestion: " + ai.Suggestion,
							Element:     ai.Element,
							Viewport:    viewportWidth,
						})
					}
					if auditRes.Severity == "critical" {
						pageResult.Passed = false
						pageResult.Summary = "Visual audit found critical issues: " + auditRes.Summary
					}
				}

				// Persist issues to visual_qa_issues table
				for _, issue := range captureIssues {
					v.insertIssue(ctx, runID, pageConfig.Name, viewportWidth, issue)
				}
				totalIssuesFound += len(captureIssues)
				allIssues = append(allIssues, captureIssues...)

				pageResults = append(pageResults, pageResult)
				continue
			}

			if !baselineExists {
				fmt.Printf("[VisualQA] No baseline found for %s at %dpx. Skipping comparison.\n", pageConfig.Name, viewportWidth)
				pageResult.Passed = false
				pageResult.Summary = "No baseline found. Manual approval needed."
				pagesFailed++
				pageResults = append(pageResults, pageResult)
				continue
			}

			fmt.Printf("[VisualQA] Comparing %s at %dpx...\n", pageConfig.Name, viewportWidth)
			compareRes, err := v.compareImages(ctx, baselinePath, captureOutputPath)
			if err != nil {
				fmt.Printf("[VisualQA] Error comparing images for %s at %dpx: %v\n", pageConfig.Name, viewportWidth, err)
				overallError = err
				pageResult.Passed = false
				pageResult.Summary = fmt.Sprintf("Image comparison failed: %v", err)
				pagesFailed++
			} else {
				pageResult.Passed = compareRes.Passed
				pageResult.Confidence = compareRes.Confidence
				pageResult.Summary = compareRes.Summary
				pageResult.Differences = compareRes.Differences
				if compareRes.Passed {
					pagesPassed++
				} else {
					pagesFailed++
				}
			}

			// Run standalone visual audit (finds issues baseline comparison misses)
			fmt.Printf("[VisualQA] Running visual audit for %s at %dpx...\n", pageConfig.Name, viewportWidth)
			auditRes, auditErr := v.auditImage(ctx, captureOutputPath, viewportWidth, nil)
			if auditErr != nil {
				fmt.Printf("[VisualQA] Visual audit failed for %s at %dpx: %v\n", pageConfig.Name, viewportWidth, auditErr)
			} else if len(auditRes.Issues) > 0 {
				// Filter out known false positives from past feedback
				auditRes.Issues = filterKnownFalsePositives(auditRes.Issues, falsePositives)
				if len(auditRes.Issues) > 0 {
					fmt.Printf("[VisualQA] Audit found %d issues for %s at %dpx (severity: %s)\n",
						len(auditRes.Issues), pageConfig.Name, viewportWidth, auditRes.Severity)
					for _, ai := range auditRes.Issues {
						patternKey := issuePatternKey(ai.Type, ai.Element)
						captureIssues = append(captureIssues, UIIssue{
							Type:        ai.Type,
							Severity:    ai.Severity,
							Description: ai.Description + " Suggestion: " + ai.Suggestion,
							Element:     ai.Element,
							Viewport:    viewportWidth,
							PatternKey:  patternKey,
						})
					}
					if auditRes.Severity == "critical" {
						pageResult.Passed = false
						pageResult.Summary = "Visual audit found critical issues: " + auditRes.Summary
					}
				}
			}

			// Persist issues to visual_qa_issues table
			for _, issue := range captureIssues {
				v.insertIssue(ctx, runID, pageConfig.Name, viewportWidth, issue)
			}
			totalIssuesFound += len(captureIssues)
			allIssues = append(allIssues, captureIssues...)

			pageResults = append(pageResults, pageResult)
		}
	}

	completedAt := time.Now()
	durationMs := int(completedAt.Sub(startedAt).Milliseconds())

	// Run fix engine on collected issues
	var allFixSuggestions []FixSuggestion
	if v.fixEngine != nil && len(allIssues) > 0 {
		suggestions, err := v.fixEngine.ProcessIssues(ctx, allIssues, "")
		if err != nil {
			fmt.Printf("[VisualQA] Fix engine error: %v\n", err)
		} else if len(suggestions) > 0 {
			fmt.Printf("[VisualQA] Fix engine produced %d suggestions\n", len(suggestions))
			allFixSuggestions = append(allFixSuggestions, suggestions...)
		}
		// Auto-apply fixes if configured
		if v.config.FixConfig.Mode == "auto_fix" && len(allFixSuggestions) > 0 {
			results := v.fixEngine.ApplyFixes(ctx, allFixSuggestions)
			for _, r := range results {
				if r.Success {
					fmt.Printf("[VisualQA] Fix applied successfully: %s\n", r.SuggestionID)
				} else if r.Error != "" {
					fmt.Printf("[VisualQA] Fix failed for %s: %s\n", r.SuggestionID, r.Error)
				}
			}
		}
		// Persist fix suggestions
		if err := v.saveFixSuggestions(ctx, runID, allFixSuggestions); err != nil {
			fmt.Printf("[VisualQA] Failed to save fix suggestions: %v\n", err)
		}
	}

	status := "completed"
	errorMessage := ""
	if overallError != nil {
		status = "failed"
		errorMessage = overallError.Error()
	}

	resultsJSON, _ := json.Marshal(pageResults)
	_, err = v.db.Exec(ctx, `
		UPDATE visual_qa_runs
		SET status = $1, pages_checked = $2, pages_passed = $3, pages_failed = $4, results = $5, error_message = $6, completed_at = $7, duration_ms = $8, issues_found = $9
		WHERE id = $10
	`, status, pagesChecked, pagesPassed, pagesFailed, resultsJSON, errorMessage, completedAt, durationMs, totalIssuesFound, runID)
	if err != nil {
		fmt.Printf("[VisualQA] Failed to update visual_qa_run %s: %v\n", runID, err)
	}

	return RunResult{
		RunID:        runID,
		TriggeredBy:  triggeredBy,
		PagesChecked: pagesChecked,
		PagesPassed:  pagesPassed,
		PagesFailed:  pagesFailed,
		IssuesFound:  totalIssuesFound,
		PageResults:  pageResults,
		DurationMs:   durationMs,
	}, overallError
}

// insertIssue persists a single UI issue to the visual_qa_issues table.
func (v *VisualQA) insertIssue(ctx context.Context, runID, pageName string, viewport int, issue UIIssue) {
	if v.db == nil {
		return
	}
	_, err := v.db.Exec(ctx, `
		INSERT INTO visual_qa_issues (run_id, type, severity, element, description, suggestion, page_name, viewport)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, runID, issue.Type, issue.Severity, issue.Element, issue.Description, "", pageName, viewport)
	if err != nil {
		fmt.Printf("[VisualQA] Failed to insert issue: %v\n", err)
	}
}

// GetIssuesForRun returns all issues for a given run ID.
func (v *VisualQA) GetIssuesForRun(ctx context.Context, runID string) ([]map[string]any, error) {
	rows, err := v.db.Query(ctx, `
		SELECT id, type, severity, element, description, suggestion, page_name, viewport, created_at
		FROM visual_qa_issues
		WHERE run_id = $1
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("[VisualQA] Failed to query issues for run %s: %w", runID, err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, issueType, severity, element, description, suggestion, pageName, createdAt string
		var viewport int
		if err := rows.Scan(&id, &issueType, &severity, &element, &description, &suggestion, &pageName, &viewport, &createdAt); err != nil {
			continue
		}
		results = append(results, map[string]any{
			"id":          id,
			"type":        issueType,
			"severity":    severity,
			"element":     element,
			"description": description,
			"suggestion":  suggestion,
			"page_name":   pageName,
			"viewport":    viewport,
			"created_at":  createdAt,
		})
	}
	return results, nil
}

// GetEnabled returns whether VisualQA is enabled.
func (v *VisualQA) GetEnabled() bool {
	return v != nil && v.config.Enabled
}

// issuePatternKey generates a stable pattern key for deduplication and feedback matching.
// Format: "type:element-class" so similar issues across runs map to the same pattern.
func issuePatternKey(issueType, element string) string {
	// Truncate element to first 50 chars and normalize
	el := element
	if len(el) > 50 {
		el = el[:50]
	}
	return issueType + ":" + el
}

// filterKnownFalsePositives removes issues that match previously marked false-positive patterns.
func filterKnownFalsePositives(issues []AuditIssue, falsePositives map[string]bool) []AuditIssue {
	if len(falsePositives) == 0 {
		return issues
	}
	var filtered []AuditIssue
	for _, issue := range issues {
		key := issuePatternKey(issue.Type, issue.Element)
		if falsePositives[key] {
			fmt.Printf("[VisualQA] Suppressing false-positive: %s\n", key)
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// RecordIssueFeedback stores user feedback on an issue (confirmed, false_positive, or wont_fix).
// The pattern_key is used to suppress similar noise in future runs.
func (v *VisualQA) RecordIssueFeedback(ctx context.Context, runID, issueType, issueElement, issueDescription string, viewport int, verdict, userNote, patternKey string) error {
	_, err := v.db.Exec(ctx, `
		INSERT INTO visual_qa_issue_feedback (run_id, issue_type, issue_element, issue_description, viewport, verdict, user_note, pattern_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, runID, issueType, issueElement, issueDescription, viewport, verdict, userNote, patternKey)
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to record feedback: %w", err)
	}
	fmt.Printf("[VisualQA] Feedback recorded: %s -> %s (pattern: %s)\n", issueType, verdict, patternKey)
	return nil
}

// GetIssueFeedback returns all feedback records for calibration.
func (v *VisualQA) GetIssueFeedback(ctx context.Context) ([]map[string]any, error) {
	rows, err := v.db.Query(ctx, `
		SELECT id, run_id, issue_type, issue_element, viewport, verdict, user_note, pattern_key, created_at
		FROM visual_qa_issue_feedback
		ORDER BY created_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, fmt.Errorf("[VisualQA] Failed to query feedback: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, runID, issueType, issueElement, verdict, userNote, patternKey, createdAt string
		var viewport int
		if err := rows.Scan(&id, &runID, &issueType, &issueElement, &viewport, &verdict, &userNote, &patternKey, &createdAt); err != nil {
			continue
		}
		results = append(results, map[string]any{
			"id":            id,
			"run_id":        runID,
			"issue_type":    issueType,
			"issue_element": issueElement,
			"viewport":      viewport,
			"verdict":       verdict,
			"user_note":     userNote,
			"pattern_key":   patternKey,
			"created_at":    createdAt,
		})
	}
	return results, nil
}

// GetFalsePositivePatterns returns pattern_keys that have been marked as false_positive.
func (v *VisualQA) GetFalsePositivePatterns(ctx context.Context) map[string]bool {
	patterns := make(map[string]bool)
	rows, err := v.db.Query(ctx, `
		SELECT DISTINCT pattern_key
		FROM visual_qa_issue_feedback
		WHERE verdict = 'false_positive'
	`)
	if err != nil {
		return patterns
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil {
			patterns[key] = true
		}
	}
	return patterns
}
