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

type GeminiCompareRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inline_data,omitempty"`
}

type GeminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type GeminiCompareResponse struct {
	Candidates []struct {
		Content GeminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

func (v *VisualQA) compareImages(ctx context.Context, baselinePath, currentPath string) (ComparisonResult, error) {
	timeout := 30
	if v.config.ComparisonTimeoutSeconds > 0 {
		timeout = v.config.ComparisonTimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	baselineBytes, err := os.ReadFile(baselinePath)
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to read baseline image %s: %w", baselinePath, err)
	}
	currentBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to read current image %s: %w", currentPath, err)
	}

	baselineB64 := base64.StdEncoding.EncodeToString(baselineBytes)
	currentB64 := base64.StdEncoding.EncodeToString(currentBytes)

	prompt := "You are a visual QA agent comparing two screenshots of the same web page. BASELINE is the approved reference. CURRENT is newly captured. Compare semantically. Ignore minor anti-aliasing, font rendering differences, and dynamic content (timestamps, dates, random IDs). Check for: layout shifts, missing elements, new elements, text changes, color changes, alignment issues. Return JSON only: {\"passed\": bool, \"confidence\": float, \"summary\": string, \"differences\": [{\"type\": string, \"severity\": string, \"region\": string, \"description\": string}]}"

	req := GeminiCompareRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
					{Text: "BASELINE:"},
					{InlineData: &GeminiInlineData{MIMEType: "image/png", Data: baselineB64}},
					{Text: "CURRENT:"},
					{InlineData: &GeminiInlineData{MIMEType: "image/png", Data: currentB64}},
				},
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to marshal Gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", v.config.Model, v.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(httpReq)
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to send Gemini request: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to read Gemini response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Gemini API returned non-OK status: %d, Body: %s", res.StatusCode, string(respBody))
	}

	var geminiResp GeminiCompareResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to unmarshal Gemini response: %w, Body: %s", err, string(respBody))
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] No content in Gemini response: %s", string(respBody))
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Extract JSON from response (may be wrapped in markdown code block)
	responseText = extractJSONFromText(responseText)

	var comparisonResult ComparisonResult
	if err := json.Unmarshal([]byte(responseText), &comparisonResult); err != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] Failed to unmarshal comparison result from Gemini response: %w, Text: %s", err, responseText)
	}

	return comparisonResult, nil
}

// extractJSONFromText extracts JSON from text that may be wrapped in markdown code blocks.
func extractJSONFromText(text string) string {
	// Try to find JSON within code blocks first
	start := -1
	end := -1

	// Check for ```json ... ``` blocks
	if idx := indexOf(text, "```json"); idx >= 0 {
		start = indexOf(text, "{", idx)
	}
	if start < 0 {
		if idx := indexOf(text, "```"); idx >= 0 {
			start = indexOf(text, "{", idx)
		}
	}
	if start < 0 {
		start = indexOf(text, "{")
	}

	if start < 0 {
		return text
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end > start {
		return text[start:end]
	}
	return text
}

func indexOf(s, substr string, fromIdx ...int) int {
	start := 0
	if len(fromIdx) > 0 {
		start = fromIdx[0]
	}
	if start >= len(s) {
		return -1
	}
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
