package visualqa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	timeout := 90
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

	// Build provider chain for fallback
	providers := v.getProviderChain()
	var lastErr error

	for _, pState := range providers {
		// Skip providers that 429'd recently
		if !pState.Last429.IsZero() && time.Since(pState.Last429) < 60*time.Second {
			continue
		}

		// Check RPM and RPD limits
		if !pState.canMakeRequest() {
			fmt.Printf("[VisualQA] Audit skipping %s (%s): rate limit (RPD %d/%d)\n",
				pState.Provider.Type, pState.Provider.Model,
				pState.DayRequests, pState.Provider.RPD)
			continue
		}

		result, err := v.callVisionProvider(ctx, pState, prompt, imgB64, "")
		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
				pState.Last429 = time.Now()
				pState.DayRequests++ // count it against daily quota even on 429
				fmt.Printf("[VisualQA] Audit provider %s (%s) rate limited, trying next\n",
					pState.Provider.Type, pState.Provider.Model)
				lastErr = err
				continue
			}
			fmt.Printf("[VisualQA] Audit provider %s (%s) error: %v\n", pState.Provider.Type, pState.Provider.Model, err)
			lastErr = err
			continue
		}

		pState.RequestTimes = append(pState.RequestTimes, time.Now())
		pState.DayRequests++

		responseText := extractJSONFromText(result.Text)
		var auditResult AuditResult
		if err := json.Unmarshal([]byte(responseText), &auditResult); err != nil {
			fmt.Printf("[VisualQA] Failed to parse %s audit response: %v\n", pState.Provider.Type, err)
			lastErr = fmt.Errorf("[VisualQA] Failed to unmarshal audit result from %s: %w", pState.Provider.Type, err)
			continue
		}

		// Merge DOM audit issues that the vision model confirmed or that it missed
		auditResult.Issues = mergeAuditIssues(auditResult.Issues, domAuditResults)
		auditResult.Severity = worstSeverity(auditResult.Issues)
		return auditResult, nil
	}

	if lastErr != nil {
		return AuditResult{}, fmt.Errorf("[VisualQA] All vision providers failed for audit. Last error: %w", lastErr)
	}
	return AuditResult{}, fmt.Errorf("[VisualQA] No vision providers available for audit")
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
// These checks are designed to catch the REAL issues users see, not subpixel noise.
func domAuditScript(viewport int) string {
	return fmt.Sprintf(`() => {
	const issues = [];
	const vw = %d;
	const isMobile = vw < 500;
	const isTablet = vw >= 500 && vw < 900;

	// 1. STRAY TEXT: Code-like text that shouldn't be visible
	document.querySelectorAll('*').forEach(el => {
		for (const node of el.childNodes) {
			if (node.nodeType === 3) {
				const text = node.textContent.trim();
				if (/^(undefined|null|NaN|\[object|\{.*\}|=&gt;|function|var |let |const )/.test(text)) {
					issues.push({
						type: 'stray_text',
						severity: 'critical',
						element: el.tagName + '.' + (el.className || '').substring(0, 50),
						description: 'Code-like text rendered visible: "' + text.substring(0, 80) + '"',
						suggestion: 'Check that this value is handled for null/undefined cases'
					});
				}
				// Duplicate bare number alongside styled version (React 0 && JSX bug)
				if (/^\d+$/.test(text) && text.length <= 3) {
					const parent = node.parentElement;
					if (parent && (parent.tagName === 'BUTTON' || parent.tagName === 'DIV' || parent.tagName === 'SPAN')) {
						const styled = parent.querySelector('span, strong, em, .token-count');
						if (styled && styled.textContent.includes(text)) {
							issues.push({
								type: 'stray_text',
								severity: 'critical',
								element: parent.tagName + '.' + (parent.className || '').substring(0, 50),
								description: 'Duplicate bare number "' + text + '" rendered alongside styled version. React falsy-value bug.',
								suggestion: 'Change condition from "value && ..." to "value != null && ..."'
							});
						}
					}
				}
			}
		}
	});

	// 2. REAL OVERFLOW: Content significantly wider than its visible area (>20px, not subpixel noise)
	document.querySelectorAll('button, span, div, p, h1, h2, h3, h4, td, th, li, section, article').forEach(el => {
		if (el.clientWidth <= 0 || el.clientHeight <= 0) return;
		const overflowX = el.scrollWidth - el.clientWidth;
		const overflowY = el.scrollHeight - el.clientHeight;
		const style = getComputedStyle(el);

		// Only flag if content is meaningfully clipped (>20px hidden) AND overflow is hidden
		if (overflowX > 20 && style.overflow !== 'visible' && style.overflowX !== 'visible') {
			issues.push({
				type: 'overflow',
				severity: overflowX > 80 ? 'critical' : 'warning',
				element: el.tagName + '.' + (el.className || '').substring(0, 60),
				description: 'Content is ' + el.scrollWidth + 'px wide but only ' + el.clientWidth + 'px visible (' + overflowX + 'px hidden). Text or controls are cut off.',
				suggestion: 'Increase container width, allow horizontal scroll, or add text-overflow: ellipsis'
			});
		}
		if (overflowY > 30 && el.clientHeight < 60 && style.overflow !== 'visible' && style.overflowY !== 'visible') {
			issues.push({
				type: 'clipping',
				severity: overflowY > 50 ? 'critical' : 'warning',
				element: el.tagName + '.' + (el.className || '').substring(0, 60),
				description: 'Content is ' + el.scrollHeight + 'px tall but only ' + el.clientHeight + 'px visible (' + overflowY + 'px hidden). Content cut off vertically.',
				suggestion: 'Increase container height or allow overflow'
			});
		}
	});

	// 3. ELEMENTS EXTENDING BEYOND VIEWPORT (the real mobile killer)
	document.querySelectorAll('div, section, table, ul, ol').forEach(el => {
		const rect = el.getBoundingClientRect();
		// Skip tiny elements and offscreen elements
		if (rect.width < 100 || rect.height < 20) return;
		if (rect.top > window.innerHeight) return;

		const overRight = Math.round(rect.right - vw);
		const overLeft = Math.round(-rect.left);

		if (overRight > 15) {
			issues.push({
				type: 'layout',
				severity: overRight > 50 ? 'critical' : 'warning',
				element: el.tagName + '.' + (el.className || '').substring(0, 60),
				description: 'Element extends ' + overRight + 'px beyond right edge of viewport. Content on the right side is cut off and inaccessible.',
				suggestion: 'Add max-width: 100vw or overflow-x: auto to contain within viewport'
			});
		}
	});

	// 4. TEXT CONTRAST: Grey text on dark backgrounds that's hard to read
	document.querySelectorAll('p, span, li, td, th, label, h1, h2, h3, h4, h5, h6, a, button').forEach(el => {
		const style = getComputedStyle(el);
		const color = style.color;
		// Parse rgba
		const match = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
		if (match) {
			const r = parseInt(match[1]), g = parseInt(match[2]), b = parseInt(match[3]);
			// Perceived brightness
			const brightness = (r * 299 + g * 587 + b * 114) / 1000;
			// On a dark background, text with brightness 80-140 is "too dark grey"
			// We check if the element is inside a dark container
			const parentBg = getComputedStyle(el.parentElement || el).backgroundColor;
			const bgMatch = parentBg.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
			if (bgMatch) {
				const br = parseInt(bgMatch[1]), bg = parseInt(bgMatch[2]), bb = parseInt(bgMatch[3]);
				const bgBrightness = (br * 299 + bg * 587 + bb * 114) / 1000;
				if (bgBrightness < 80 && brightness > 60 && brightness < 150 && el.textContent.trim().length > 5) {
					issues.push({
						type: 'contrast',
						severity: 'warning',
						element: el.tagName + '.' + (el.className || '').substring(0, 50),
						description: 'Low contrast text (brightness ' + Math.round(brightness) + ') on dark background (brightness ' + Math.round(bgBrightness) + '). Text is hard to read: "' + el.textContent.trim().substring(0, 50) + '"',
						suggestion: 'Increase text color brightness to at least 180 (use rgba(200,210,230,...) or similar)'
					});
				}
			}
		}
	});

	// 5. EXCESSIVE SPACING: Large gaps between filter bar and content
	document.querySelectorAll('ul, .model-panel__list, [class*="list"], [class*="items"]').forEach(list => {
		const firstItem = list.querySelector(':scope > li, :scope > div, :scope > tr');
		if (!firstItem) return;
		const listRect = list.getBoundingClientRect();
		const firstRect = firstItem.getBoundingClientRect();
		const gap = firstRect.top - listRect.top;

		// If there's more than 60px gap between list top and first item, that's excessive
		if (gap > 60 && listRect.height > 100) {
			issues.push({
				type: 'spacing',
				severity: gap > 120 ? 'warning' : 'info',
				element: list.tagName + '.' + (list.className || '').substring(0, 50),
				description: 'Excessive gap (' + Math.round(gap) + 'px) between filter bar and first content item. User has to scroll past empty space.',
				suggestion: 'Reduce padding-top or margin-top on this list to bring content closer to filters'
			});
		}
	});

	return issues;
}`, viewport)
}
