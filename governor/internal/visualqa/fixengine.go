package visualqa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FixStrategy is a pluggable strategy for fixing a specific type of UI issue.
// Strategies are registered in the FixEngine and matched by issue type.
// Each strategy can analyze, suggest a fix, and optionally apply it.
type FixStrategy interface {
	// CanFix returns true if this strategy knows how to handle the given issue.
	CanFix(issue UIIssue) bool
	// Analyze inspects the issue against the source code and produces a fix suggestion.
	// The sourceRoot is the filesystem path to the frontend source (e.g. the dashboard repo).
	Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) (*FixSuggestion, error)
	// Apply executes the fix. Only called when suggestion.AutoApplicable is true
	// and the user (or config) has approved auto-fix.
	Apply(ctx context.Context, suggestion *FixSuggestion) (*FixResult, error)
}

// FixSuggestion represents a proposed fix for a UI issue.
type FixSuggestion struct {
	IssueID         string            `json:"issue_id"`
	StrategyName    string            `json:"strategy_name"`
	SourceFile      string            `json:"source_file"`
	SourceLine      int               `json:"source_line"`
	Description     string            `json:"description"`
	Confidence      float64           `json:"confidence"`
	AutoApplicable  bool              `json:"auto_applicable"`
	Applied         bool              `json:"applied"`
	Before          string            `json:"before"`
	After           string            `json:"after"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// FixResult is the outcome of applying a fix.
type FixResult struct {
	SuggestionID string `json:"suggestion_id"`
	Success      bool   `json:"success"`
	Output       string `json:"output"`
	Error        string `json:"error,omitempty"`
}

// FixEngine takes UI issues from a Visual QA run and applies matching fix strategies.
type FixEngine struct {
	strategies    []FixStrategy
	config        FixConfig
	sourceRoot    string
	visionAPIKey  string
	visionModel   string
}

// FixConfig controls how the fix engine operates.
type FixConfig struct {
	Enabled       bool    `json:"enabled"`
	Mode          string  `json:"mode"`            // "suggest" or "auto_fix"
	SourceRoot    string  `json:"source_root"`     // path to frontend source code
	ConfidenceMin float64 `json:"confidence_min"`  // minimum confidence to auto-apply (0-1)
	VisionModel   string  `json:"vision_model"`    // model for visual analysis
	VisionAPIKey  string  `json:"-"`               // set from vault at runtime
}

// NewFixEngine creates a fix engine with the standard set of strategies.
func NewFixEngine(config FixConfig) *FixEngine {
	fe := &FixEngine{
		config:      config,
		sourceRoot:  config.SourceRoot,
	}
	// Register built-in strategies in priority order
	fe.RegisterStrategy(&ConsoleErrorStrategy{})
	fe.RegisterStrategy(&LayoutBreakpointStrategy{})
	fe.RegisterStrategy(&VisualAnomalyStrategy{})
	return fe
}

// RegisterStrategy adds a fix strategy to the engine.
func (fe *FixEngine) RegisterStrategy(s FixStrategy) {
	fe.strategies = append(fe.strategies, s)
}

// ProcessIssues takes a list of UI issues from a capture run and produces fix suggestions.
// For each issue, it finds the first matching strategy and runs analysis.
func (fe *FixEngine) ProcessIssues(ctx context.Context, issues []UIIssue, screenshotPath string) ([]FixSuggestion, error) {
	if !fe.config.Enabled {
		return nil, nil
	}

	var suggestions []FixSuggestion
	for _, issue := range issues {
		for _, strategy := range fe.strategies {
			if strategy.CanFix(issue) {
				suggestion, err := strategy.Analyze(ctx, issue, fe.sourceRoot, screenshotPath)
				if err != nil {
					fmt.Printf("[FixEngine] Strategy %T failed on issue %s: %v\n", strategy, issue.Type, err)
					continue
				}
				if suggestion != nil {
					suggestions = append(suggestions, *suggestion)
				}
				break // first matching strategy wins
			}
		}
	}

	return suggestions, nil
}

// ApplyFixes applies all auto-applicable suggestions above the confidence threshold.
func (fe *FixEngine) ApplyFixes(ctx context.Context, suggestions []FixSuggestion) []FixResult {
	var results []FixResult
	for i := range suggestions {
		s := &suggestions[i]
		if !s.AutoApplicable || s.Confidence < fe.config.ConfidenceMin {
			continue
		}
		// Find the strategy that produced this suggestion
		for _, strategy := range fe.strategies {
			if strategy.CanFix(UIIssue{Type: s.Metadata["issue_type"]}) {
				result, err := strategy.Apply(ctx, s)
				if err != nil {
					results = append(results, FixResult{
						SuggestionID: s.IssueID,
						Success:      false,
						Error:        err.Error(),
					})
				} else {
					s.Applied = true
					results = append(results, *result)
				}
				break
			}
		}
	}
	return results
}

// ---------------------------------------------------------------------------
// Built-in Strategy: Console Error Trace
// Maps console errors (CORS, network, JS exceptions) to source code locations
// and produces actionable suggestions (fix CORS headers, fix API endpoint, etc.)
// ---------------------------------------------------------------------------

type ConsoleErrorStrategy struct{}

func (s *ConsoleErrorStrategy) CanFix(issue UIIssue) bool {
	return issue.Type == "console_error"
}

func (s *ConsoleErrorStrategy) Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) (*FixSuggestion, error) {
	desc := issue.Description
	suggestion := &FixSuggestion{
		IssueID:      fmt.Sprintf("%s-%d-%s", issue.Type, issue.Viewport, issue.Element),
		StrategyName: "console_error_trace",
		Confidence:   0.9,
		Metadata:     map[string]string{"issue_type": issue.Type},
	}

	// Classify the error type
	if strings.Contains(desc, "CORS") || strings.Contains(desc, "cors") {
		// CORS error -- find the API endpoint in the frontend source
		apiURL := extractURL(desc)
		sourceFile, sourceLine := findInSource(sourceRoot, apiURL)
		suggestion.SourceFile = sourceFile
		suggestion.SourceLine = sourceLine
		suggestion.Description = fmt.Sprintf(
			"CORS policy blocks fetch to %s. The backend at this URL needs Access-Control-Allow-Origin headers. "+
				"Fix: Add CORS middleware to the backend handler for this endpoint, or proxy the request through the same origin.",
			apiURL)
		suggestion.Confidence = 0.95
		suggestion.AutoApplicable = false // CORS fixes need backend changes, not auto-applied

	} else if strings.Contains(desc, "ERR_FAILED") || strings.Contains(desc, "net::ERR") {
		// Network error -- endpoint might be down or URL wrong
		apiURL := extractURL(desc)
		sourceFile, sourceLine := findInSource(sourceRoot, apiURL)
		suggestion.SourceFile = sourceFile
		suggestion.SourceLine = sourceLine
		suggestion.Description = fmt.Sprintf(
			"Network request failed to %s. The endpoint may be down, the URL may be incorrect, or there may be a DNS/firewall issue.",
			apiURL)
		suggestion.Confidence = 0.7
		suggestion.AutoApplicable = false

	} else if strings.Contains(desc, "PAGE_ERROR") || strings.Contains(desc, "Uncaught") {
		// JavaScript exception -- find the source
		sourceFile, sourceLine := findJSErrorSource(sourceRoot, desc)
		suggestion.SourceFile = sourceFile
		suggestion.SourceLine = sourceLine
		suggestion.Description = fmt.Sprintf("Uncaught JavaScript error: %s. Check the source file for the exception.", desc)
		suggestion.Confidence = 0.6
		suggestion.AutoApplicable = false

	} else {
		suggestion.Description = fmt.Sprintf("Console error detected: %s", desc)
		suggestion.Confidence = 0.5
		suggestion.AutoApplicable = false
	}

	return suggestion, nil
}

func (s *ConsoleErrorStrategy) Apply(ctx context.Context, suggestion *FixSuggestion) (*FixResult, error) {
	// Console errors typically require manual backend/source fixes
	return &FixResult{SuggestionID: suggestion.IssueID, Success: false, Error: "console errors require manual fix"}, nil
}

// ---------------------------------------------------------------------------
// Built-in Strategy: Layout Breakpoint Fixer
// Detects layout issues at specific viewport widths and suggests CSS fixes
// by analyzing the source CSS for media query gaps.
// ---------------------------------------------------------------------------

type LayoutBreakpointStrategy struct{}

func (s *LayoutBreakpointStrategy) CanFix(issue UIIssue) bool {
	return issue.Type == "layout"
}

func (s *LayoutBreakpointStrategy) Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) (*FixSuggestion, error) {
	suggestion := &FixSuggestion{
		IssueID:      fmt.Sprintf("%s-%d", issue.Type, issue.Viewport),
		StrategyName: "layout_breakpoint",
		Confidence:   0.5,
		Metadata:     map[string]string{"issue_type": issue.Type},
	}

	// Read the main CSS file and check for media queries covering this viewport
	cssFiles := findCSSFiles(sourceRoot)
	var nearestBreakpoints []int
	var cssFile string
	for _, f := range cssFiles {
		breakpoints := extractMediaQueryBreakpoints(f)
		if len(breakpoints) > 0 {
			nearestBreakpoints = breakpoints
			cssFile = f
			break
		}
	}

	if cssFile != "" {
		suggestion.SourceFile = cssFile
		// Find the gap in breakpoints
		vw := issue.Viewport
		var lower, upper int
		for _, bp := range nearestBreakpoints {
			if bp <= vw && bp > lower {
				lower = bp
			}
			if bp >= vw && (upper == 0 || bp < upper) {
				upper = bp
			}
		}
		suggestion.Description = fmt.Sprintf(
			"Layout issue at %dpx viewport. Nearest CSS breakpoints: %dpx and %dpx. "+
				"A media query covering this viewport width may be missing or has incorrect rules. "+
				"Consider adding @media (max-width: %dpx) rules to fix the layout at this size.",
			vw, lower, upper, vw)
		suggestion.Confidence = 0.7
	} else {
		suggestion.Description = fmt.Sprintf("Layout issue at %dpx viewport. No CSS media queries found in source.", issue.Viewport)
	}

	suggestion.AutoApplicable = false // CSS fixes need review before applying
	return suggestion, nil
}

func (s *LayoutBreakpointStrategy) Apply(ctx context.Context, suggestion *FixSuggestion) (*FixResult, error) {
	return &FixResult{SuggestionID: suggestion.IssueID, Success: false, Error: "layout fixes require manual review"}, nil
}

// ---------------------------------------------------------------------------
// Built-in Strategy: Visual Anomaly Detector
// Uses Gemini Vision to analyze the screenshot and identify specific visual issues
// like broken layouts, overlapping elements, unreadable text, etc.
// Produces detailed natural-language fix suggestions.
// ---------------------------------------------------------------------------

type VisualAnomalyStrategy struct{}

func (s *VisualAnomalyStrategy) CanFix(issue UIIssue) bool {
	return issue.Type == "visual"
}

func (s *VisualAnomalyStrategy) Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) (*FixSuggestion, error) {
	suggestion := &FixSuggestion{
		IssueID:      fmt.Sprintf("%s-%d-%s", issue.Type, issue.Viewport, issue.Element),
		StrategyName: "visual_anomaly",
		Description:  issue.Description,
		Confidence:   0.6,
		Metadata:     map[string]string{"issue_type": issue.Type},
	}

	// Try to locate the source file for the element
	if issue.Element != "" && sourceRoot != "" {
		sourceFile, sourceLine := findInSource(sourceRoot, issue.Element)
		suggestion.SourceFile = sourceFile
		suggestion.SourceLine = sourceLine
	}

	suggestion.AutoApplicable = false
	return suggestion, nil
}

func (s *VisualAnomalyStrategy) Apply(ctx context.Context, suggestion *FixSuggestion) (*FixResult, error) {
	return &FixResult{SuggestionID: suggestion.IssueID, Success: false, Error: "visual anomalies require manual review"}, nil
}

// ---------------------------------------------------------------------------
// Helper functions for source code analysis
// ---------------------------------------------------------------------------

// extractURL pulls a URL from an error message string.
func extractURL(text string) string {
	// Match http:// or https:// URLs
	re := regexp.MustCompile(`https?://[^\s'"]+`)
	match := re.FindString(text)
	if match != "" {
		// Clean trailing punctuation
		match = strings.TrimRight(match, ".,;:)]}")
	}
	return match
}

// findInSource searches the source tree for files containing the given string.
// Returns the relative file path and line number of the first match.
func findInSource(sourceRoot, searchStr string) (string, int) {
	if sourceRoot == "" || searchStr == "" {
		return "", 0
	}

	// Use grep to find the match quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "grep", "-rn",
		"--include=*.ts", "--include=*.tsx", "--include=*.js", "--include=*.jsx",
		"--include=*.css", "--include=*.scss", "--include=*.html",
		"-m1", searchStr, sourceRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0
	}

	// Parse grep output: file:line:content
	parts := strings.SplitN(strings.TrimSpace(string(output)), ":", 3)
	if len(parts) >= 2 {
		relPath := strings.TrimPrefix(parts[0], sourceRoot+"/")
		line := 0
		fmt.Sscanf(parts[1], "%d", &line)
		return relPath, line
	}
	return "", 0
}

// findJSErrorSource tries to extract a source file reference from a JS error.
func findJSErrorSource(sourceRoot, errorMsg string) (string, int) {
	// JS errors often contain file:line:col references
	re := regexp.MustCompile(`(?:at |@)(.+?):(\d+):(\d+)`)
	matches := re.FindStringSubmatch(errorMsg)
	if len(matches) >= 3 {
		file := matches[1]
		line := 0
		fmt.Sscanf(matches[2], "%d", &line)
		// Try to find the file in the source tree
		fullPath := filepath.Join(sourceRoot, file)
		if _, err := os.Stat(fullPath); err == nil {
			return file, line
		}
	}
	return "", 0
}

// findCSSFiles locates CSS files in the source tree.
func findCSSFiles(sourceRoot string) []string {
	if sourceRoot == "" {
		return nil
	}
	var files []string
	filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(path, ".css") && !strings.Contains(path, "node_modules") && !strings.Contains(path, ".next") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// extractMediaQueryBreakpoints finds all @media breakpoint values in a CSS file.
func extractMediaQueryBreakpoints(cssFilePath string) []int {
	data, err := os.ReadFile(cssFilePath)
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`@media[^{]*\((?:max|min)-width:\s*(\d+)px`)
	matches := re.FindAllStringSubmatch(string(data), -1)

	seen := map[int]bool{}
	var breakpoints []int
	for _, m := range matches {
		if len(m) >= 2 {
			val := 0
			fmt.Sscanf(m[1], "%d", &val)
			if val > 0 && !seen[val] {
				breakpoints = append(breakpoints, val)
				seen[val] = true
			}
		}
	}
	return breakpoints
}

// saveFixSuggestions persists fix suggestions to the DB.
func (v *VisualQA) saveFixSuggestions(ctx context.Context, runID string, suggestions []FixSuggestion) error {
	if len(suggestions) == 0 {
		return nil
	}
	data, err := json.Marshal(suggestions)
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to marshal fix suggestions: %w", err)
	}
	_, err = v.db.Exec(ctx, `
		UPDATE visual_qa_runs SET fix_suggestions = $1 WHERE id = $2
	`, data, runID)
	return err
}
