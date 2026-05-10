package visualqa

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	Enabled                  bool          `json:"enabled"`
	CapturePages             []PageConfig  `json:"capture_pages"`
	BaselineDir              string        `json:"baseline_dir"`
	ManifestFile             string        `json:"manifest_file"`
	ConnectorID              string        `json:"connector_id"`
	Model                    string        `json:"model"`
	CaptureTimeoutSeconds    int           `json:"capture_timeout_seconds"`
	ComparisonTimeoutSeconds int           `json:"comparison_timeout_seconds"`
	AutoApproveFirstBaseline bool          `json:"auto_approve_first_baseline"`
	GitCommitBaselines       bool          `json:"git_commit_baselines"`
	TempDir                  string        `json:"temp_dir"`
	RepoPath                 string        `json:"repo_path"`
	GeminiVaultKey           string        `json:"gemini_vault_key"`
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
	PageResults  []PageResult `json:"page_results"`
	DurationMs   int          `json:"duration_ms"`
}

type PageResult struct {
	PageName    string       `json:"page_name"`
	Viewport    int          `json:"viewport"`
	Passed      bool         `json:"passed"`
	Confidence  float64      `json:"confidence"`
	Summary     string       `json:"summary"`
	Differences []Difference `json:"differences"`
	CapturePath string       `json:"capture_path"`
	BaselineNew bool         `json:"baseline_new"`
}

// DB is the interface the visual QA package needs for persistence.
// Uses the same pattern as the governor's DB layer.
type DB interface {
	Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
}

// VisualQA is the main visual regression testing engine.
type VisualQA struct {
	config  Config
	apiKey string
	db     DB
}

// NewVisualQA creates a new VisualQA instance.
func NewVisualQA(config Config, apiKey string, db DB) *VisualQA {
	if db == nil {
		fmt.Println("[VisualQA] WARNING: DB interface is nil.")
	}
	return &VisualQA{
		config:  config,
		apiKey:  apiKey,
		db:      db,
	}
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

	// Insert initial run record into DB
	initialResultsJSON, _ := json.Marshal([]PageResult{})
	_, err := v.db.Exec(ctx, `
		INSERT INTO visual_qa_runs (id, triggered_by, status, pages_checked, pages_passed, pages_failed, results, error_message, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, runID, triggeredBy, "running", 0, 0, 0, initialResultsJSON, "", startedAt)
	if err != nil {
		return RunResult{}, fmt.Errorf("[VisualQA] Failed to insert initial visual_qa_run: %w", err)
	}

	var pageResults []PageResult
	pagesChecked := 0
	pagesPassed := 0
	pagesFailed := 0
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
			pageResults = append(pageResults, pageResult)
		}
	}

	completedAt := time.Now()
	durationMs := int(completedAt.Sub(startedAt).Milliseconds())

	status := "completed"
	errorMessage := ""
	if overallError != nil {
		status = "failed"
		errorMessage = overallError.Error()
	}

	resultsJSON, _ := json.Marshal(pageResults)
	_, err = v.db.Exec(ctx, `
		UPDATE visual_qa_runs
		SET status = $1, pages_checked = $2, pages_passed = $3, pages_failed = $4, results = $5, error_message = $6, completed_at = $7, duration_ms = $8
		WHERE id = $9
	`, status, pagesChecked, pagesPassed, pagesFailed, resultsJSON, errorMessage, completedAt, durationMs, runID)
	if err != nil {
		fmt.Printf("[VisualQA] Failed to update visual_qa_run %s: %v\n", runID, err)
	}

	return RunResult{
		RunID:        runID,
		TriggeredBy:  triggeredBy,
		PagesChecked: pagesChecked,
		PagesPassed:  pagesPassed,
		PagesFailed:  pagesFailed,
		PageResults:  pageResults,
		DurationMs:   durationMs,
	}, overallError
}

// GetEnabled returns whether VisualQA is enabled.
func (v *VisualQA) GetEnabled() bool {
	return v != nil && v.config.Enabled
}
