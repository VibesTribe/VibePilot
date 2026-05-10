package visualqa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// CaptureResult holds the result of a screenshot capture.
type CaptureResult struct {
	Path     string
	FileSize int64
}

// captureScreenshot captures a single screenshot using Playwright headless Chromium.
// It includes retry logic with exponential backoff for transient failures.
func (v *VisualQA) captureScreenshot(ctx context.Context, url, outputPath string, viewportWidth int) (CaptureResult, error) {
	timeout := time.Duration(v.config.CaptureTimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return CaptureResult{}, fmt.Errorf("failed to create capture directory: %w", err)
	}

	// Playwright Python script for screenshot capture.
	// Uses --no-sandbox for headless Linux environments without a user namespace.
	script := fmt.Sprintf(`
from playwright.sync_api import sync_playwright
import sys, os

def capture():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, args=["--no-sandbox", "--disable-gpu"])
        page = browser.new_page(viewport={"width": %d, "height": 900})
        page.goto("%s", wait_until="networkidle", timeout=30000)
        page.screenshot(path="%s", full_page=True)
        sz = os.path.getsize("%s")
        if sz < 1000:
            print(f"WARNING: screenshot is only {sz} bytes, possibly blank", file=sys.stderr)
        browser.close()

try:
    capture()
except Exception as e:
    print(f"CAPTURE_ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`, viewportWidth, url, outputPath, outputPath)

	var lastErr error
	maxRetries := 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return CaptureResult{}, ctx.Err()
			}
			fmt.Printf("[VisualQA] Retrying capture for %s at %dpx (attempt %d/%d)\n", url, viewportWidth, attempt+1, maxRetries+1)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(attemptCtx, "python3", "-u", "-c", script)
		output, err := cmd.CombinedOutput()
		cancel()

		if err == nil {
			// Verify file was created and has reasonable size
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.Size() > 1000 {
				return CaptureResult{Path: outputPath, FileSize: info.Size()}, nil
			}
			if statErr != nil {
				lastErr = fmt.Errorf("screenshot file not created: %s", outputPath)
			} else {
				lastErr = fmt.Errorf("screenshot too small (%d bytes), possibly blank", info.Size())
			}
		} else {
			lastErr = fmt.Errorf("playwright capture failed for %s (%dpx): %s: %w", url, viewportWidth, string(output), err)
		}
	}

	return CaptureResult{}, lastErr
}
