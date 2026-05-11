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
	Type        string `json:"type"`        // "console_error", "visual", "click_test", "layout"
	Severity    string `json:"severity"`    // "critical", "warning", "info"
	Description string `json:"description"` // human-readable description
	Element     string `json:"element"`     // selector or element name
	Viewport    int    `json:"viewport"`    // viewport width where found
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
