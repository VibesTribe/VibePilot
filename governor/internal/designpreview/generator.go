package designpreview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const designPromptTemplate = `You are a UI design generator. Generate a complete, standalone HTML mockup for the following task.

TASK: %s
DESCRIPTION: %s

Requirements:
- All CSS must be inline
- Use Inter font from Google Fonts CDN
- Dark theme: background #0a0a0a, surface #141414, text #e5e5e5, accent #3b82f6
- Must look like production UI, not a wireframe
- Use px for all measurements
- Include realistic placeholder data
- Responsive layout
- Return ONLY the HTML code, no markdown wrapping or explanation`

// Generator generates HTML mockups using the Gemini API.
type Generator struct {
	config  Config
	apiKey  string
}

// NewGenerator creates a new Generator instance.
func NewGenerator(config Config, apiKey string) *Generator {
	return &Generator{
		config: config,
		apiKey: apiKey,
	}
}

// GenerateHTMLMockup generates an HTML mockup using the Gemini API.
// Returns the HTML content and token counts.
func (g *Generator) GenerateHTMLMockup(ctx context.Context, taskTitle, taskDescription string) (string, int, int, error) {
	timeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := fmt.Sprintf(designPromptTemplate, taskTitle, taskDescription)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 8192,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", 0, 0, fmt.Errorf("[DesignPreview] Failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.config.Model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", 0, 0, fmt.Errorf("[DesignPreview] Failed to create API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("[DesignPreview] API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("[DesignPreview] Failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("[DesignPreview] Gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []map[string]interface{} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		return "", 0, 0, fmt.Errorf("[DesignPreview] Failed to decode response: %w", err)
	}

	if len(apiResponse.Candidates) == 0 || len(apiResponse.Candidates[0].Content.Parts) == 0 {
		return "", 0, 0, fmt.Errorf("[DesignPreview] No content in API response")
	}

	text, ok := apiResponse.Candidates[0].Content.Parts[0]["text"].(string)
	if !ok {
		return "", 0, 0, fmt.Errorf("[DesignPreview] No text content in response")
	}

	// Strip markdown code blocks if present
	text = stripMarkdownCodeBlock(text)

	tokensIn := apiResponse.UsageMetadata.PromptTokenCount
	tokensOut := apiResponse.UsageMetadata.CandidatesTokenCount

	return text, tokensIn, tokensOut, nil
}

// SaveHTML writes the generated HTML to a file in the repo.
func (g *Generator) SaveHTML(repoPath, designDir, taskID string, version int, htmlContent string) (string, error) {
	fullDir := filepath.Join(repoPath, designDir)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("[DesignPreview] Failed to create design directory: %w", err)
	}

	fileName := fmt.Sprintf("task-%s-v%d.html", taskID, version)
	filePath := filepath.Join(fullDir, fileName)

	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		return "", fmt.Errorf("[DesignPreview] Failed to write HTML file: %w", err)
	}

	// Return relative path from repo root for DB storage
	return filepath.Join(designDir, fileName), nil
}

// CaptureScreenshot takes a screenshot of a local HTML file using Playwright.
func (g *Generator) CaptureScreenshot(ctx context.Context, htmlFilePath string, viewportWidth int) (string, error) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	absPath, err := filepath.Abs(htmlFilePath)
	if err != nil {
		return "", fmt.Errorf("[DesignPreview] Failed to resolve path: %w", err)
	}

	screenshotPath := absPath + ".png"

	script := fmt.Sprintf(`
from playwright.sync_api import sync_playwright
import sys

def capture():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, args=["--no-sandbox", "--disable-gpu"])
        page = browser.new_page(viewport={"width": %d, "height": 900})
        page.goto("file://%s", wait_until="networkidle", timeout=15000)
        page.screenshot(path="%s", full_page=True)
        browser.close()

try:
    capture()
except Exception as e:
    print(f"CAPTURE_ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`, viewportWidth, absPath, screenshotPath)

	cmd := exec.CommandContext(ctx, "python3", "-u", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("[DesignPreview] Screenshot capture failed: %s: %w", string(output), err)
	}

	if _, err := os.Stat(screenshotPath); err != nil {
		return "", fmt.Errorf("[DesignPreview] Screenshot file not created: %s", screenshotPath)
	}

	return screenshotPath, nil
}

// stripMarkdownCodeBlock removes ```html ... ``` wrapping from LLM output.
func stripMarkdownCodeBlock(text string) string {
	// Check for ```html prefix
	if len(text) > 7 && text[:7] == "```html" {
		if end := findClosingBackticks(text, 7); end > 0 {
			return text[7:end]
		}
	}
	// Check for ``` prefix
	if len(text) > 3 && text[:3] == "```" {
		newlineIdx := 0
		for i := 3; i < len(text); i++ {
			if text[i] == '\n' {
				newlineIdx = i + 1
				break
			}
		}
		if newlineIdx > 0 {
			if end := findClosingBackticks(text, newlineIdx); end > 0 {
				return text[newlineIdx:end]
			}
		}
	}
	return text
}

func findClosingBackticks(text string, startFrom int) int {
	for i := len(text) - 3; i >= startFrom; i-- {
		if i >= 0 && i+3 <= len(text) && text[i:i+3] == "```" {
			return i
		}
	}
	return -1
}

// encodeFileToBase64 reads a file and returns its base64 encoding.
func encodeFileToBase64(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("[DesignPreview] Failed to read file %s: %w", filePath, err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
