package visualqa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AuditResult holds the findings from a standalone visual audit (no baseline needed).
type AuditResult struct {
	Issues    []AuditIssue `json:"issues"`
	Summary   string       `json:"summary"`
	Severity  string       `json:"severity"` // worst severity found: "clean", "info", "warning", "critical"
	Confidence float64     `json:"confidence"`
}

// AuditIssue is a single problem found during visual audit.
type AuditIssue struct {
	Type        string `json:"type"`        // "stray_text", "misalignment", "overflow", "clipping", "spacing", "layout", "visual"
	Severity    string `json:"severity"`    // "critical", "warning", "info"
	Element     string `json:"element"`     // CSS selector or element description
	Description string `json:"description"` // what's wrong
	Suggestion  string `json:"suggestion"`  // how to fix
	SourceFile  string `json:"source_file"` // suspected source file (if traceable)
}

// auditImage sends a single screenshot to Gemini for standalone quality analysis.
// Unlike compareImages, this does NOT need a baseline. It evaluates the page against
// UI quality rules: alignment, spacing, stray text, overflow, clipping, visual coherence.
func (v *VisualQA) auditImage(ctx context.Context, screenshotPath string, viewport int, domAuditResults []AuditIssue) (AuditResult, error) {
	timeout := 45
	if v.config.ComparisonTimeoutSeconds > 0 {
		timeout = v.config.ComparisonTimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	imgBytes, err := os.ReadFile(screenshotPath)
	if err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to read audit screenshot %s: %w", screenshotPath, err)
	}
	imgB64 := base64.StdEncoding.EncodeToString(imgBytes)

	// Build DOM audit context for the vision model
	domContext := ""
	if len(domAuditResults) > 0 {
		domContext = "\n\nDOM INSPECTION ALREADY FOUND THESE ISSUES (verify visually):\n"
		for _, issue := range domAuditResults {
			domContext += fmt.Sprintf("- [%s] %s: %s\n", issue.Severity, issue.Type, issue.Description)
		}
		domContext += "\nConfirm each DOM issue visually and find any the DOM check missed."
	}

	prompt := `You are a harsh visual QA auditor inspecting a web dashboard screenshot at ` + fmt.Sprintf("%dpx viewport width.", viewport) + `
You are NOT comparing to any baseline. You are evaluating this page on its own merits.

Find EVERYTHING that looks wrong, broken, or unprofessional. Be nitpicky. Check for:

1. STRAY TEXT: Numbers, code snippets, variable names, or raw text that looks like it leaked from the codebase (e.g. a bare "0", "{}", "undefined", technical strings that shouldn't be user-visible)
2. ALIGNMENT: Elements that should be centered but aren't. Elements that are visually off-balance. Columns that don't line up. Content that sits too high or too low in its container.
3. SPACING: Inconsistent gaps between similar elements. Too much whitespace in one area vs another. Elements crammed together that should have breathing room.
4. OVERFLOW/CLIPPING: Text that gets cut off on any side. Containers that are too narrow for their content. Scrollbars that shouldn't be needed. Content hidden behind other elements.
5. LAYOUT: Elements that overlap when they shouldn't. Content that extends beyond its parent container. Broken responsive behavior (content too wide or too narrow for the viewport).
6. TYPOGRAPHY: Text too small to read. Font size jumps that look accidental. Inconsistent styling between related elements.
7. VISUAL NOISE: Warning banners, alerts, or status messages that dominate the page or sit in wrong locations. Debug information left visible. Placeholder text.` + domContext + `

Return JSON only:
{
  "issues": [
    {
      "type": "stray_text|misalignment|overflow|clipping|spacing|layout|typography|visual_noise",
      "severity": "critical|warning|info",
      "element": "description of which element",
      "description": "what exactly is wrong",
      "suggestion": "how to fix it"
    }
  ],
  "summary": "one sentence overall assessment",
  "severity": "clean|info|warning|critical",
  "confidence": 0.0-1.0
}

If the page genuinely looks perfect, return empty issues with severity "clean" and confidence 1.0.
But default to finding problems. A real QA engineer doesn't say "looks fine" without looking carefully.`

	req := GeminiCompareRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
					{InlineData: &GeminiInlineData{MIMEType: "image/png", Data: imgB64}},
				},
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to marshal audit request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", v.config.Model, v.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to create audit HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(httpReq)
	if err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to send audit request: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to read audit response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return AuditResult{}, fmt.Errorf("[VisualQA] Audit API returned status %d: %s", res.StatusCode, string(respBody))
	}

	var geminiResp GeminiCompareResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to unmarshal audit response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return AuditResult{}, fmt.Errorf("[VisualQA] No content in audit response")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = extractJSONFromText(responseText)

	var auditResult AuditResult
	if err := json.Unmarshal([]byte(responseText), &auditResult); err != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] Failed to unmarshal audit result: %w, Text: %s", err, responseText)
	}

	// Merge DOM audit issues that the vision model confirmed or that it missed
	auditResult.Issues = mergeAuditIssues(auditResult.Issues, domAuditResults)

	// Update overall severity based on merged results
	auditResult.Severity = worstSeverity(auditResult.Issues)

	return auditResult, nil
}

// mergeAuditIssues combines vision-found issues with DOM-found issues,
// deduplicating where they overlap.
func mergeAuditIssues(visionIssues []AuditIssue, domIssues []AuditIssue) []AuditIssue {
	seen := map[string]bool{}
	var merged []AuditIssue

	for _, issue := range visionIssues {
		key := issue.Type + ":" + issue.Element
		seen[key] = true
		merged = append(merged, issue)
	}
	// Add DOM issues that vision didn't catch
	for _, issue := range domIssues {
		key := issue.Type + ":" + issue.Element
		if !seen[key] {
			issue.Description += " (DOM-detected, not confirmed by vision)"
			merged = append(merged, issue)
		}
	}
	return merged
}

func worstSeverity(issues []AuditIssue) string {
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return "critical"
		}
	}
	for _, issue := range issues {
		if issue.Severity == "warning" {
			return "warning"
		}
	}
	for _, issue := range issues {
		if issue.Severity == "info" {
			return "info"
		}
	}
	return "clean"
}

// domAuditScript returns JavaScript that runs DOM-level quality checks inside Playwright.
// These checks catch issues that visual comparison misses because they're in the baseline.
func domAuditScript(viewport int) string {
	return fmt.Sprintf(`() => {
	const issues = [];
	const vw = %d;

	// 1. STRAY TEXT NODES: Find text nodes that look like leaked code
	document.querySelectorAll('*').forEach(el => {
		for (const node of el.childNodes) {
			if (node.nodeType === 3) { // text node
				const text = node.textContent.trim();
				// Bare numbers that are direct children of buttons/containers
				if (/^\d+$/.test(text) && text.length <= 3) {
					const parent = node.parentElement;
					if (parent && (parent.tagName === 'BUTTON' || parent.tagName === 'DIV' || parent.tagName === 'SPAN')) {
						// Check if siblings already have styled spans with this content
						const styled = parent.querySelector('span, strong, em, .token-count');
						if (styled && styled.textContent.includes(text)) {
							issues.push({
								type: 'stray_text',
								severity: 'critical',
								element: parent.tagName + '.' + (parent.className || '').substring(0, 50),
								description: 'Duplicate bare number "' + text + '" rendered as text node alongside styled version. Likely a React falsy-value bug (0 && JSX renders literal 0).',
								suggestion: 'Change condition from "value && ..." to "value != null && ..." or "!!value && ..."'
							});
						}
					}
				}
				// Code-like text that shouldn't be visible
				if (/^(undefined|null|NaN|\[object|\{.*\}|=&gt;|function|var |let |const )/.test(text)) {
					issues.push({
						type: 'stray_text',
						severity: 'critical',
						element: el.tagName + '.' + (el.className || '').substring(0, 50),
						description: 'Code-like text rendered visible: "' + text.substring(0, 80) + '"',
						suggestion: 'Check that this value is handled for null/undefined cases'
					});
				}
			}
		}
	});

	// 2. OVERFLOW/CLIPPING: Elements with text cut off
	document.querySelectorAll('button, span, div, p, h1, h2, h3, h4, td, th, li').forEach(el => {
		if (el.scrollWidth > el.clientWidth + 2 && el.clientWidth > 0) {
			const style = getComputedStyle(el);
			if (style.overflow !== 'visible' && style.overflowX !== 'visible') {
				issues.push({
					type: 'overflow',
					severity: 'warning',
					element: el.tagName + '.' + (el.className || '').substring(0, 60),
					description: 'Text overflow: content is ' + el.scrollWidth + 'px wide but container is only ' + el.clientWidth + 'px. Text is being clipped.',
					suggestion: 'Increase container width, reduce font size, or add text-overflow: ellipsis'
				});
			}
		}
		if (el.scrollHeight > el.clientHeight + 2 && el.clientHeight > 0) {
			const style = getComputedStyle(el);
			if (style.overflow !== 'visible' && style.overflowY !== 'visible' && el.clientHeight < 40) {
				issues.push({
					type: 'clipping',
					severity: 'warning',
					element: el.tagName + '.' + (el.className || '').substring(0, 60),
					description: 'Vertical clipping: content is ' + el.scrollHeight + 'px tall but container is only ' + el.clientHeight + 'px.',
					suggestion: 'Increase container height or allow overflow'
				});
			}
		}
	});

	// 3. CENTERING CHECK: Elements that should be centered but aren't
	document.querySelectorAll('.slice-orbit, [class*="orbit"], [class*="center"], [class*="hub"]').forEach(el => {
		const rect = el.getBoundingClientRect();
		const parent = el.parentElement;
		if (parent) {
			const parentRect = parent.getBoundingClientRect();
			const expectedCenterX = parentRect.left + parentRect.width / 2;
			const actualCenterX = rect.left + rect.width / 2;
			const offset = Math.abs(expectedCenterX - actualCenterX);
			if (offset > 10) {
				issues.push({
					type: 'misalignment',
					severity: 'warning',
					element: el.tagName + '.' + (el.className || '').substring(0, 60),
					description: 'Element is ' + Math.round(offset) + 'px off-center horizontally. Expected center at ' + Math.round(expectedCenterX) + ', actual at ' + Math.round(actualCenterX),
					suggestion: 'Add margin: 0 auto, or justify-content: center, or align-items: center'
				});
			}
			const expectedCenterY = parentRect.top + parentRect.height / 2;
			const actualCenterY = rect.top + rect.height / 2;
			const offsetY = Math.abs(expectedCenterY - actualCenterY);
			if (offsetY > 15 && parentRect.height > 200) {
				issues.push({
					type: 'misalignment',
					severity: 'warning',
					element: el.tagName + '.' + (el.className || '').substring(0, 60),
					description: 'Element is ' + Math.round(offsetY) + 'px off-center vertically. Expected center at ' + Math.round(expectedCenterY) + ', actual at ' + Math.round(actualCenterY),
					suggestion: 'Add align-items: center to parent container'
				});
			}
		}
	});

	// 4. SPACING CONSISTENCY: Check sibling elements for inconsistent gaps
	document.querySelectorAll('.mission-header__stats, [class*="pills"], [class*="stats"], [class*="buttons"]').forEach(container => {
		const children = Array.from(container.children).filter(c => c.offsetHeight > 0);
		if (children.length >= 3) {
			const gaps = [];
			for (let i = 1; i < children.length; i++) {
				const prevRect = children[i-1].getBoundingClientRect();
				const currRect = children[i].getBoundingClientRect();
				gaps.push(Math.round(currRect.left - prevRect.right));
			}
			const avg = gaps.reduce((a,b) => a+b, 0) / gaps.length;
			for (let i = 0; i < gaps.length; i++) {
				if (Math.abs(gaps[i] - avg) > 8 && avg > 0) {
					issues.push({
						type: 'spacing',
						severity: 'info',
						element: container.tagName + '.' + (container.className || '').substring(0, 50),
						description: 'Inconsistent horizontal spacing: gap ' + i + ' is ' + gaps[i] + 'px (average is ' + Math.round(avg) + 'px)',
						suggestion: 'Use consistent gap/padding values for all sibling elements'
					});
					break; // one report per container
				}
			}
		}
	});

	// 5. ALERT/WARNING PLACEMENT: Check if alerts sit in inappropriate locations
	document.querySelectorAll('[style*="245, 158, 11"], [style*="rgba(245"], [class*="alert"], [class*="warn"]').forEach(el => {
		const parent = el.parentElement;
		if (parent) {
			const parentClass = parent.className || '';
			// Alerts inside header content area (between buttons and progress bar)
			if (parentClass.includes('header') || parentClass.includes('content')) {
				const siblings = Array.from(parent.children);
				const alertIdx = siblings.indexOf(el);
				const hasButtonsBefore = siblings.slice(0, alertIdx).some(s => s.tagName === 'BUTTON');
				const hasProgressAfter = siblings.slice(alertIdx + 1).some(s => (s.className || '').includes('progress'));
				if (hasButtonsBefore) {
					issues.push({
						type: 'visual_noise',
						severity: 'warning',
						element: el.tagName + '.' + (el.className || '').substring(0, 50),
						description: 'Alert/warning block sits between interactive controls (buttons) and progress bar in the header. This breaks the visual flow of the primary controls.',
						suggestion: 'Move alerts to a dedicated notification area, toast, or collapse into a small badge'
					});
				}
			}
		}
	});

	return issues;
}`, viewport)
}
