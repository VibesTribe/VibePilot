package visualqa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// CaptureResult holds the result of a screenshot capture.
type CaptureResult struct {
	Path     string
	FileSize int64
}

// UIIssue represents a visual or functional issue found during capture.
type UIIssue struct {
	Type        string `json:"type"`         // "console_error", "visual", "click_test", "layout"
	Severity    string `json:"severity"`     // "critical", "warning", "info"
	Description string `json:"description"`  // human-readable description
	Element     string `json:"element"`      // selector or element name
	Viewport    int    `json:"viewport"`     // viewport width where found
	PatternKey  string `json:"pattern_key"`  // stable key for feedback matching (type:element)
}

// UICaptureReport holds the full report from a capture including interactions.
type UICaptureReport struct {
	ScreenshotPath string    `json:"screenshot_path"`
	Issues         []UIIssue `json:"issues"`
	ConsoleErrors  []string  `json:"console_errors"`
	URL            string    `json:"url"`
	Viewport       int       `json:"viewport"`
	Title          string    `json:"title"`
	LoadTimeMs     int       `json:"load_time_ms"`
}

// captureScreenshot captures a single screenshot using Playwright headless Chromium.
// It includes retry logic with exponential backoff for transient failures.
// The UI interaction report is stored in v.lastReport for the caller to read.
func (v *VisualQA) captureScreenshot(ctx context.Context, url, outputPath string, viewportWidth int) (CaptureResult, error) {
	report, err := v.captureWithInteraction(ctx, url, outputPath, viewportWidth)
	if err != nil {
		return CaptureResult{}, err
	}
	// Store report for Run() to read UI issues
	v.lastReport = report
	// Log any issues found
	if len(report.Issues) > 0 {
		fmt.Printf("[VisualQA] UI issues found for %s at %dpx: %d issues\n", url, viewportWidth, len(report.Issues))
		for _, issue := range report.Issues {
			fmt.Printf("[VisualQA]   [%s] %s: %s\n", issue.Severity, issue.Type, issue.Description)
		}
	}
	info, _ := os.Stat(outputPath)
	sz := int64(0)
	if info != nil {
		sz = info.Size()
	}
	return CaptureResult{Path: report.ScreenshotPath, FileSize: sz}, nil
}

// captureWithInteraction captures a screenshot AND does interactive UI testing.
func (v *VisualQA) captureWithInteraction(ctx context.Context, url, outputPath string, viewportWidth int) (*UICaptureReport, error) {
	timeout := time.Duration(v.config.CaptureTimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 90 * time.Second
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create capture directory: %w", err)
	}

	issuesPath := outputPath + ".issues.json"
	vw := strconv.Itoa(viewportWidth)

	// Build the Playwright Python script using string concatenation to avoid fmt.Sprintf format conflicts
	domAuditJS := domAuditScript(viewportWidth)
	script := `
from playwright.sync_api import sync_playwright
import sys, os, json, time

def capture():
    issues = []
    console_errors = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"])
        page = browser.new_page(viewport={"width": ` + vw + `, "height": 900})

        # Collect console errors
        page.on("console", lambda msg: console_errors.append(msg.text) if msg.type == "error" else None)
        page.on("pageerror", lambda err: console_errors.append("PAGE_ERROR: " + str(err)))

        start = time.time()
        page.goto("` + url + `", wait_until="domcontentloaded", timeout=30000)
        load_ms = int((time.time() - start) * 1000)

        page.wait_for_timeout(3000)
        title = page.title()

        # Check console errors
        skip_patterns = ["favicon", "manifest.json", ".well-known", "chrome-extension"]
        for err in console_errors:
            if any(p in err.lower() for p in skip_patterns):
                continue
            issues.append({
                "type": "console_error",
                "severity": "warning",
                "description": err[:300],
                "element": "",
                "viewport": ` + vw + `
            })

        # Click test: try clicking nav links to verify they work
        click_selectors = ["nav a", "nav button", "[role='navigation'] a", "aside a", ".sidebar a", "a[href]"]
        clicked = set()
        for sel in click_selectors:
            try:
                els = page.query_selector_all(sel)
                for el in els[:5]:
                    try:
                        href = el.get_attribute("href") or ""
                        text = (el.inner_text() or "")[:50]
                        if not text and not href:
                            continue
                        key = sel + ":" + text + ":" + href
                        if key in clicked:
                            continue
                        clicked.add(key)
                        if not el.is_visible():
                            continue
                        el.scroll_into_view_if_needed(timeout=3000)
                        page.wait_for_timeout(200)
                        el.click(timeout=3000)
                        page.wait_for_timeout(1000)
                        try:
                            page.title()
                        except:
                            issues.append({
                                "type": "click_test",
                                "severity": "critical",
                                "description": "Page crashed after clicking: " + text + " (" + href + ")",
                                "element": sel,
                                "viewport": ` + vw + `
                            })
                            page.go_back(timeout=10000)
                            page.wait_for_timeout(2000)
                    except Exception:
                        pass
            except Exception:
                pass

        # Check layout: page not blank
        body_html = page.evaluate("() => document.body.innerHTML")
        if len(body_html) < 100:
            issues.append({
                "type": "layout",
                "severity": "critical",
                "description": "Page body is nearly empty, possibly blank or broken",
                "element": "body",
                "viewport": ` + vw + `
            })

        # Check for visible error banners
        error_texts = page.evaluate("""() => {
            const results = [];
            const errorSelectors = ['.error', '[role="alert"]', '.toast-error', '.notification-error'];
            for (const sel of errorSelectors) {
                document.querySelectorAll(sel).forEach(el => {
                    if (el.textContent.trim()) results.push(sel + ': ' + el.textContent.trim().substring(0, 200));
                });
            }
            return results;
        }""")
        for et in (error_texts or []):
            issues.append({
                "type": "visual",
                "severity": "warning",
                "description": et,
                "element": "error-banner",
                "viewport": ` + vw + `
            })

        # DOM QUALITY AUDIT: Check for stray text, overflow, centering, spacing
        dom_issues = page.evaluate("""` + domAuditJS + `""")
        if dom_issues:
            for di in dom_issues:
                issues.append({
                    "type": di.get("type", "visual"),
                    "severity": di.get("severity", "warning"),
                    "description": di.get("description", ""),
                    "element": di.get("element", ""),
                    "viewport": ` + vw + `
                })

        # Take the initial full-page screenshot
        page.screenshot(path="` + outputPath + `", full_page=True)

        # INTERACTIVE STATE AUDIT: Click buttons/tabs, open panels, audit each state
        def audit_state(label):
            """Run DOM + modal audit on current page state after an interaction."""
            state_issues = page.evaluate("""` + domAuditJS + `""")
            if state_issues:
                for si in state_issues:
                    issues.append({
                        "type": si.get("type", "visual"),
                        "severity": si.get("severity", "warning"),
                        "description": "[after clicking " + label + "] " + si.get("description", ""),
                        "element": si.get("element", ""),
                        "viewport": ` + vw + `
                    })
            modal_issues = page.evaluate("""() => {
                const issues = [];
                document.querySelectorAll('[role="dialog"], .modal, [data-state="active"], [data-state="open"], [aria-expanded="true"]').forEach(el => {
                    const rect = el.getBoundingClientRect();
                    if (rect.width === 0 || rect.height === 0) return;
                    el.querySelectorAll('p, span, div, h1, h2, h3, h4, td, th, li, button').forEach(child => {
                        if (child.scrollWidth > child.clientWidth + 2 && child.clientWidth > 0) {
                            const cs = getComputedStyle(child);
                            if (cs.overflow !== 'visible' && cs.overflowX !== 'visible') {
                                issues.push({
                                    type: "overflow", severity: "warning",
                                    element: child.tagName + "." + (child.className || "").substring(0, 50),
                                    description: "Text overflow in popup: content is " + child.scrollWidth + "px but container is " + child.clientWidth + "px."
                                });
                            }
                        }
                        if (child.scrollHeight > child.clientHeight + 2 && child.clientHeight > 0 && child.clientHeight < 50) {
                            issues.push({
                                type: "clipping", severity: "warning",
                                element: child.tagName + "." + (child.className || "").substring(0, 50),
                                description: "Content clipped in popup: " + child.scrollHeight + "px tall but container only " + child.clientHeight + "px."
                            });
                        }
                    });
                    if (rect.right > window.innerWidth + 10) {
                        issues.push({type: "layout", severity: "critical", element: "popup",
                            description: "Popup extends " + Math.round(rect.right - window.innerWidth) + "px beyond right edge. Content cut off."});
                    }
                    if (rect.bottom > window.innerHeight + 50) {
                        issues.push({type: "layout", severity: "warning", element: "popup",
                            description: "Popup extends " + Math.round(rect.bottom - window.innerHeight) + "px below viewport."});
                    }
                    el.querySelectorAll('*').forEach(child => {
                        for (const node of child.childNodes) {
                            if (node.nodeType === 3) {
                                const t = node.textContent.trim();
                                if (/^(undefined|null|NaN|\\[object|\\{.*\\}|=>|function)/.test(t)) {
                                    issues.push({type: "stray_text", severity: "critical", element: "popup-content",
                                        description: "Code-like text in popup: " + t.substring(0, 80)});
                                }
                            }
                        }
                    });
                });
                return issues;
            }""")
            if modal_issues:
                for mi in modal_issues:
                    issues.append({
                        "type": mi.get("type", "visual"),
                        "severity": mi.get("severity", "warning"),
                        "description": "[popup: " + label + "] " + mi.get("description", ""),
                        "element": mi.get("element", ""),
                        "viewport": ` + vw + `
                    })

        # Click dashboard tabs (Logs, Models, Docs, Admin)
        for btn_text in ["Logs", "Models", "Docs", "Admin"]:
            try:
                el = page.get_by_text(btn_text, exact=True).first
                if el and el.is_visible():
                    el.scroll_into_view_if_needed(timeout=2000)
                    page.wait_for_timeout(100)
                    el.click(timeout=3000)
                    page.wait_for_timeout(2000)
                    audit_state(btn_text)
            except Exception:
                pass

        # Click stat pills and other interactive buttons
        interactive_selectors = [
            "button.mission-header__stat-pill",
            "button.slice-orbit__center",
        ]
        for sel in interactive_selectors:
            try:
                els = page.query_selector_all(sel)
                for idx, el in enumerate(els[:6]):
                    try:
                        if not el.is_visible():
                            continue
                        text = (el.inner_text() or "").strip()[:40].replace("\\n", " ")
                        if not text:
                            text = sel.split(".")[-1] + "-" + str(idx)
                        el.scroll_into_view_if_needed(timeout=2000)
                        page.wait_for_timeout(100)
                        el.click(timeout=3000)
                        page.wait_for_timeout(1500)
                        audit_state(text)
                    except Exception:
                        pass
            except Exception:
                pass

        # Go back to initial state for final screenshot
        try:
            page.reload(wait_until="domcontentloaded", timeout=15000)
            page.wait_for_timeout(2000)
        except Exception:
            pass
        page.screenshot(path="` + outputPath + `", full_page=True)
        sz = os.path.getsize("` + outputPath + `")
        if sz < 1000:
            issues.append({
                "type": "visual",
                "severity": "warning",
                "description": "Screenshot is only " + str(sz) + " bytes, page may be blank",
                "element": "page",
                "viewport": ` + vw + `
            })

        browser.close()

    report = {
        "url": "` + url + `",
        "viewport": ` + vw + `,
        "title": title,
        "load_time_ms": load_ms,
        "issues": issues,
        "console_errors": console_errors[:50],
        "screenshot_path": "` + outputPath + `"
    }
    with open("` + issuesPath + `", "w") as f:
        json.dump(report, f, indent=2)

try:
    capture()
except Exception as e:
    print("CAPTURE_ERROR: " + str(e), file=sys.stderr)
    sys.exit(1)
`

	var lastErr error
	maxRetries := 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			fmt.Printf("[VisualQA] Retrying capture for %s at %spx (attempt %d/%d)\n", url, vw, attempt+1, maxRetries+1)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(attemptCtx, "python3", "-u", "-c", script)
		output, err := cmd.CombinedOutput()
		cancel()

		if err == nil {
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.Size() > 500 {
				report := &UICaptureReport{
					ScreenshotPath: outputPath,
					URL:            url,
					Viewport:       viewportWidth,
				}
				reportData, readErr := os.ReadFile(issuesPath)
				if readErr == nil {
					_ = json.Unmarshal(reportData, report)
				}
				return report, nil
			}
			if statErr != nil {
				lastErr = fmt.Errorf("screenshot file not created: %s", outputPath)
			} else {
				lastErr = fmt.Errorf("screenshot too small (%d bytes), possibly blank", info.Size())
			}
		} else {
			lastErr = fmt.Errorf("playwright capture failed for %s (%spx): %s: %w", url, vw, string(output), err)
		}
	}

	return nil, lastErr
}
