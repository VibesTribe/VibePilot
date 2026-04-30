package runtime

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Issue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type AnalystFix struct {
	RouteTo              string   `json:"route_to"`
	ModelExclude         []string `json:"model_exclude"`
	RevisedPromptAdditions string `json:"revised_prompt_additions"`
	TaskSplitSuggestion  string   `json:"task_split_suggestion"`
}

type AnalystDecision struct {
	Action        string      `json:"action"`
	TaskID        string      `json:"task_id"`
	RootCause     string      `json:"root_cause"`
	Reasoning     string      `json:"reasoning"`
	WhatWentWrong string      `json:"what_went_wrong"`
	Fix           AnalystFix `json:"fix"`
	Confidence    float64     `json:"confidence"`
}

func ParseAnalystDecision(output string) (*AnalystDecision, error) {
	var d AnalystDecision
	jsonStr := extractJSON(output)
	if err := json.Unmarshal([]byte(sanitizeJSON(jsonStr)), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func ParseAnalystDecisionFromMap(data map[string]any) (*AnalystDecision, error) {
	var d AnalystDecision
	// Map the fields
	if v, ok := data["action"].(string); ok {
		d.Action = v
	}
	if v, ok := data["task_id"].(string); ok {
		d.TaskID = v
	}
	if v, ok := data["root_cause"].(string); ok {
		d.RootCause = v
	}
	if v, ok := data["reasoning"].(string); ok {
		d.Reasoning = v
	}
	if v, ok := data["what_went_wrong"].(string); ok {
		d.WhatWentWrong = v
	}
	if v, ok := data["confidence"].(float64); ok {
		d.Confidence = v
	}
	if fixData, ok := data["fix"].(map[string]any); ok {
		if v, ok := fixData["route_to"].(string); ok {
			d.Fix.RouteTo = v
		}
		if v, ok := fixData["model_exclude"].([]any); ok {
			for _, item := range v {
				if s, ok := item.(string); ok {
					d.Fix.ModelExclude = append(d.Fix.ModelExclude, s)
				}
			}
		}
		if v, ok := fixData["revised_prompt_additions"].(string); ok {
			d.Fix.RevisedPromptAdditions = v
		}
		if v, ok := fixData["task_split_suggestion"].(string); ok {
			d.Fix.TaskSplitSuggestion = v
		}
	}
	return &d, nil
}

type SupervisorDecision struct {
	Action        string `json:"action"`
	TaskID        string `json:"task_id"`
	TaskNumber    string `json:"task_number"`
	Decision      string `json:"decision"`
	NextAction    string `json:"next_action"`
	FailureClass  string `json:"failure_class"`
	FailureDetail string `json:"failure_detail"`
	Checks        struct {
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
	FailureClass         string   `json:"failure_class"`
	FailureDetail        string   `json:"failure_detail"`
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

	// Extract plan_content separately to avoid unescaped quote issues
	planContent, cleanJSON := extractPlanContent(jsonStr)

	if planContent != "" {
		// Parse the clean JSON (plan_content replaced with empty string)
		if err := json.Unmarshal([]byte(sanitizeJSON(cleanJSON)), &p); err != nil {
			return nil, err
		}
		p.PlanContent = planContent
	} else {
		// No plan_content field or couldn't extract, try normal parsing
		if err := json.Unmarshal([]byte(sanitizeJSON(jsonStr)), &p); err != nil {
			return nil, err
		}
	}

	return &p, nil
}

func ParseTestResults(output string) (*TestResults, error) {
	var t TestResults
	jsonStr := sanitizeJSON(extractJSON(output))
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

// ParseTaskRunnerOutput extracts files from model output using a multi-strategy cascade:
//
//   Strategy 1: Structured JSON with {path, content} file objects
//   Strategy 2: Code blocks with filename hints (```python:path/to/file.py)
//   Strategy 3: Bare code blocks with language-only headers (```python, ```go)
//   Strategy 4: Raw output saved as task_output.txt (handled by caller)
//
// After extraction, files with content are validated and deduplicated.
// Files with empty content are discarded (models returning string paths).
func ParseTaskRunnerOutput(output string) (*TaskRunnerOutput, error) {
	cleanOutput := strings.TrimSpace(strings.ReplaceAll(output, "\r", ""))

	// Strategy 1: Try structured JSON extraction
	result := tryJSONExtraction(cleanOutput)
	if result != nil && hasFiles(result.Files) {
		return result, nil
	}

	// Strategy 2: Code blocks with filename annotations
	// Matches ```lang:path/to/file.ext or ``` path/to/file.ext or <!-- filename.ext -->
	codeFiles := extractCodeBlockFiles(cleanOutput)
	if len(codeFiles) > 0 {
		summary := extractSummaryFromProse(cleanOutput)
		return &TaskRunnerOutput{
			Status:  "complete",
			Summary: summary,
			Files:   codeFiles,
		}, nil
	}

	// Strategy 1 found JSON but files had no content (string paths).
	// Return the parsed result with empty files so the caller can handle it.
	if result != nil {
		return result, nil
	}

	// Nothing extractable. Caller handles raw output as task_output.txt.
	return nil, fmt.Errorf("no structured output or code blocks found in model response")
}

// tryJSONExtraction attempts to find and parse a JSON object from the output.
func tryJSONExtraction(output string) *TaskRunnerOutput {
	jsonStr := extractJSON(output)
	if jsonStr == "" || jsonStr == output {
		// extractJSON returned the whole output (no JSON found)
		if !strings.HasPrefix(strings.TrimSpace(output), "{") {
			return nil
		}
	}

	var t TaskRunnerOutput
	raw := sanitizeJSON(jsonStr)
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil
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

	return &t
}

// parseFilesArray handles multiple file formats models might return:
//   - [{path: "...", content: "..."}]  (correct, has content)
//   - ["path1.py", "path2.py"]         (wrong, no content)
func parseFilesArray(raw json.RawMessage) []File {
	if len(raw) == 0 {
		return nil
	}

	// Try structured files first
	var objectFiles []File
	if err := json.Unmarshal(raw, &objectFiles); err == nil {
		return objectFiles
	}

	// Try string paths (models that didn't read the format instructions)
	var stringFiles []string
	if err := json.Unmarshal(raw, &stringFiles); err == nil {
		files := make([]File, 0, len(stringFiles))
		for _, path := range stringFiles {
			if path != "" {
				files = append(files, File{Path: path})
			}
		}
		return files
	}

	return nil
}

// hasFiles returns true if any file has both path and content.
func hasFiles(files []File) bool {
	for _, f := range files {
		if f.Path != "" && f.Content != "" {
			return true
		}
	}
	return false
}

// extractCodeBlockFiles scans for code blocks and extracts them as files.
// Priority order for filename detection:
//   1. ```lang:path/to/file.ext (annotated code block)
//   2. <!-- filename: path/to/file.ext --> before code block
//   3. File: path/to/file.ext before code block
//   4. Bare code blocks with language: infer extension from language
func extractCodeBlockFiles(output string) []File {
	var files []File
	seen := make(map[string]bool)

	// Pattern 1: ```lang:path/to/file.ext
	annotatedBlock := regexp.MustCompile("(?s)```(?:[a-zA-Z]*[:/])([^\n`]+)\n(.*?)```")
	for _, match := range annotatedBlock.FindAllStringSubmatch(output, -1) {
		path := strings.TrimSpace(match[1])
		content := match[2]
		if path != "" && content != "" && !seen[path] {
			seen[path] = true
			files = append(files, File{Path: path, Content: content})
		}
	}
	if len(files) > 0 {
		return files
	}

	// Pattern 2: <!-- filename: path --> or File: path before code block
	// Look for (File: path or <!-- filename: path -->) followed by ```lang
	fileAnnotatedBlock := regexp.MustCompile("(?s)(?:File:\\s*|<!--\\s*filename(?:\\s+is)?:\\s*)([^<\n-]+?)\\s*--?>?\\s*\n\\s*```([a-zA-Z]*)\\s*\n(.*?)```")
	for _, match := range fileAnnotatedBlock.FindAllStringSubmatch(output, -1) {
		path := strings.TrimSpace(match[1])
		content := match[3]
		if path != "" && content != "" && !seen[path] {
			seen[path] = true
			files = append(files, File{Path: path, Content: content})
		}
	}
	if len(files) > 0 {
		return files
	}

	// Pattern 3: Bare code blocks with language header.
	// Only use these if we can infer a filename from context or language.
	// For single-block responses, use a generic name based on language.
	bareBlocks := regexp.MustCompile("(?s)```([a-zA-Z+]+)\n(.*?)```")
	matches := bareBlocks.FindAllStringSubmatch(output, -1)
	if len(matches) == 1 {
		// Single code block -- try to find a filename in the surrounding text
		lang := matches[0][1]
		content := matches[0][2]
		if filename := findFilenameNearBlock(output, lang); filename != "" && content != "" {
			return []File{{Path: filename, Content: content}}
		}
		// Single block, no filename hint -- use language-based name
		ext := langToExt(lang)
		if ext != "" && content != "" {
			return []File{{Path: "output" + ext, Content: content}}
		}
	}

	// Multiple bare blocks: only extract if we can find per-block filenames.
	// Otherwise we'd create ambiguous output_0.py, output_1.py etc.
	if len(matches) > 1 {
		for i, match := range matches {
			lang := match[1]
			content := match[2]
			// Look for filename hints near each block
			filename := findFilenameForBlock(output, matches, i, lang)
			if filename != "" && content != "" && !seen[filename] {
				seen[filename] = true
				files = append(files, File{Path: filename, Content: content})
			}
		}
		if len(files) > 0 {
			return files
		}
	}

	return files
}

// findFilenameNearBlock looks for filename hints in the text before a code block.
func findFilenameNearBlock(output string, lang string) string {
	ext := langToExt(lang)
	if ext == "" {
		return ""
	}
	escapedExt := regexp.QuoteMeta(ext)
	// Look for patterns like "creates hello.py" or "in src/main.go"
	patterns := []string{
		`(?:creates?|create|writes?|write|adds?|add|in|to|at)\s+([a-zA-Z0-9_/.-]+` + escapedExt + `)`,
		`([a-zA-Z0-9_/.-]+` + escapedExt + `)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(output); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// findFilenameForBlock tries to find a filename for a specific code block
// by looking at text between the previous block and this one.
func findFilenameForBlock(output string, blocks [][]string, idx int, lang string) string {
	// Find the position of this block in the output
	blockText := "```" + blocks[idx][1]
	pos := strings.Index(output, blockText)
	if pos < 0 {
		return ""
	}

	// Look at text before this block (between previous block end and this one)
	var preceding string
	if idx == 0 {
		preceding = output[:pos]
	} else {
		prevEnd := strings.Index(output[pos-500:], blockText)
		if prevEnd > 0 {
			preceding = output[pos-500 : pos]
		} else {
			preceding = output[:pos]
		}
	}

	// Search for filename hints in the preceding text
	ext := langToExt(lang)
	if ext == "" {
		return ""
	}
	escapedExt := regexp.QuoteMeta(ext)
	patterns := []string{
		`(?:creates?|create|writes?|write|adds?|add|in|to|at)\s+([a-zA-Z0-9_/.-]+` + escapedExt + `)`,
		`([a-zA-Z0-9_/.-]+` + escapedExt + `)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		// Find the LAST match before the block
		matches := re.FindAllStringSubmatch(preceding, -1)
		if len(matches) > 0 {
			lastMatch := matches[len(matches)-1]
			if len(lastMatch) > 1 && lastMatch[1] != "" {
				return lastMatch[1]
			}
		}
	}
	return ""
}

// langToExt maps programming language names to file extensions.
func langToExt(lang string) string {
	switch strings.ToLower(lang) {
	case "python", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "go":
		return ".go"
	case "rust":
		return ".rs"
	case "java":
		return ".java"
	case "ruby", "rb":
		return ".rb"
	case "c":
		return ".c"
	case "cpp", "c++":
		return ".cpp"
	case "csharp", "c#", "cs":
		return ".cs"
	case "swift":
		return ".swift"
	case "kotlin", "kt":
		return ".kt"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "scss":
		return ".scss"
	case "sql":
		return ".sql"
	case "sh", "bash", "shell":
		return ".sh"
	case "yaml", "yml":
		return ".yaml"
	case "json":
		return ".json"
	case "xml":
		return ".xml"
	case "markdown", "md":
		return ".md"
	case "dockerfile":
		return ".dockerfile"
	case "toml":
		return ".toml"
	case "ini", "conf", "config":
		return ".conf"
	case "lua":
		return ".lua"
	case "r":
		return ".R"
	case "perl", "pl":
		return ".pl"
	case "php":
		return ".php"
	default:
		return ""
	}
}

// extractSummaryFromProse extracts a summary from text that has no JSON.
// Returns first 500 chars of cleaned prose, or "Task output (unstructured)".
func extractSummaryFromProse(output string) string {
	// Remove code blocks for summary
	cleaned := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(output, "")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 500 {
		return cleaned[:500] + "..."
	}
	if cleaned != "" {
		return cleaned
	}
	return "Task output (unstructured)"
}

func extractJSON(output string) string {
	output = strings.TrimSpace(output)
	output = strings.ReplaceAll(output, "\r", "")

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
			trimmed := strings.TrimSpace(result)
			// Validate: must start with { and end with }
			if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
				return result
			}
			// Code fence extraction gave incomplete JSON (inner ``` cut it short)
			// Fall through to first { ... last } strategy
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
	case "truncation", "context_exceeded", "incomplete", "truncated_output":
		return "model_issue"
	case "drift", "wrong_output", "unexpected_changes", "quality_below_standard", "broken_output":
		return "quality_issue"
	case "security", "secrets", "no_hardcoded_secrets", "dangerous_output":
		return "security_issue"
	case "timeout", "rate_limited":
		return "platform_issue"
	case "prompt_needs_improvement", "task_too_large":
		return "prompt_issue"
	case "model_limitation":
		return "capability_issue"
	case "almost_perfect", "needs_revision":
		return "revision_issue"
	default:
		return "task_issue"
	}
}

// sanitizeJSON fixes common LLM JSON issues: unescaped newlines/tabs in strings,
// trailing commas, and other formatting problems that make json.Unmarshal fail.
func sanitizeJSON(input string) string {
	var result strings.Builder
	inString := false
	escape := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escape {
			escape = false
			result.WriteByte(ch)
			continue
		}

		if ch == '\\' && inString {
			escape = true
			result.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		if inString {
			switch ch {
			case '\n':
				result.WriteString("\\n")
				continue
			case '\r':
				result.WriteString("\\r")
				continue
			case '\t':
				result.WriteString("\\t")
				continue
			}
		}

		result.WriteByte(ch)
	}

	cleaned := result.String()

	// Remove trailing commas before } or ]
	for {
		replaced := strings.ReplaceAll(cleaned, ",\n}", "\n}")
		replaced = strings.ReplaceAll(replaced, ",\n]", "\n]")
		replaced = strings.ReplaceAll(replaced, ",}", "}")
		replaced = strings.ReplaceAll(replaced, ",]", "]")
		if replaced == cleaned {
			break
		}
		cleaned = replaced
	}

	return cleaned
}

// extractPlanContent safely extracts plan_content from raw LLM JSON output
// where the value may contain unescaped double quotes from markdown code blocks.
// Returns the plan_content string and the remaining JSON with plan_content replaced by empty string.
func extractPlanContent(raw string) (planContent string, cleanJSON string) {
	// Find the start of plan_content value
	marker := `"plan_content": "`
	idx := strings.Index(raw, marker)
	if idx == -1 {
		// Try with escaped quotes variant
		marker = `"plan_content":"`
		idx = strings.Index(raw, marker)
	}
	if idx == -1 {
		return "", raw
	}

	valueStart := idx + len(marker)

	// plan_content ends at the last occurrence of `"` followed by a comma or closing brace
	// at the top level. Look for the pattern: `"\n  "total_tasks"` or `"\n}`
	// We search backwards from the end for the closing quote of plan_content
	//
	// The JSON after plan_content has known keys: "total_tasks" and "status"
	// Find the last valid closing pattern
	endMarkers := []string{
		"\",\n  \"total_tasks\"",
		"\",\n  \"status\"",
		"\",\n\"total_tasks\"",
		"\",\n\"status\"",
		"\"\n}",
		"\"\n }",
	}

	for _, endMarker := range endMarkers {
		endIdx := strings.LastIndex(raw, endMarker)
		if endIdx > valueStart {
			planContent = raw[valueStart:endIdx]
			// Unescape JSON string escapes so plan file has real newlines
			planContent = unescapePlanContent(planContent)
			// Rebuild JSON with plan_content as empty string to parse other fields
			cleanJSON = raw[:valueStart] + "" + raw[endIdx:]
			return planContent, cleanJSON
		}
	}

	return "", raw
}
