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
	"strings"
	"time"
)

// VisionProvider configures one vision API endpoint.
type VisionProvider struct {
	Type    string `json:"type"`     // "gemini" or "openrouter"
	Model   string `json:"model"`    // model ID for this provider
	APIKey  string `json:"api_key"`  // API key (or vault key reference)
	BaseURL string `json:"base_url"` // override base URL
	RPM     int    `json:"rpm"`      // requests per minute limit (0 = unlimited)
	RPD     int    `json:"rpd"`      // requests per day limit (0 = unlimited)
}

// VisionProviderState tracks rate limiting per provider.
type VisionProviderState struct {
	Provider     *VisionProvider
	RequestTimes []time.Time
	DayRequests  int       // count of requests since midnight UTC
	DayReset     time.Time // when DayRequests was last reset
	Last429      time.Time
}

// canMakeRequest checks RPM and RPD limits. Returns true if OK.
func (s *VisionProviderState) canMakeRequest() bool {
	now := time.Now()

	// Reset daily counter at midnight UTC
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if s.DayReset.Before(midnight) {
		s.DayRequests = 0
		s.DayReset = now
	}

	// Check RPD
	if s.Provider.RPD > 0 && s.DayRequests >= s.Provider.RPD {
		return false
	}

	// Check RPM
	if s.Provider.RPM > 0 {
		recent := 0
		for _, t := range s.RequestTimes {
			if now.Sub(t) < time.Minute {
				recent++
			}
		}
		if recent >= s.Provider.RPM {
			return false
		}
	}

	return true
}

// Gemini API types
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

// OpenRouter API types
type ORMessage struct {
	Role    string      `json:"role"`
	Content []ORContent `json:"content"`
}

type ORContent struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *ORImageURL `json:"image_url,omitempty"`
}

type ORImageURL struct {
	URL string `json:"url"`
}

type ORRequest struct {
	Model    string      `json:"model"`
	Messages []ORMessage `json:"messages"`
}

type ORResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// visionCallResult is the internal result from any vision provider.
type visionCallResult struct {
	Text string
}

// compareImages sends images to a vision model for comparison.
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

	providers := v.getProviderChain()
	var lastErr error

	for _, pState := range providers {
		// Skip providers that 429'd recently (wait 60s before retry)
		if !pState.Last429.IsZero() && time.Since(pState.Last429) < 60*time.Second {
			fmt.Printf("[VisualQA] Skipping %s (%s): 429 cooldown (%.0fs remaining)\n",
				pState.Provider.Type, pState.Provider.Model,
				60-time.Since(pState.Last429).Seconds())
			continue
		}

		if !pState.canMakeRequest() {
			fmt.Printf("[VisualQA] Skipping %s (%s): rate limit (RPD %d/%d)\n",
				pState.Provider.Type, pState.Provider.Model,
				pState.DayRequests, pState.Provider.RPD)
			continue
		}

		result, err := v.callVisionProvider(ctx, pState, prompt, baselineB64, currentB64)
		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
				pState.Last429 = time.Now()
				pState.DayRequests++
				fmt.Printf("[VisualQA] Provider %s (%s) rate limited, trying next\n",
					pState.Provider.Type, pState.Provider.Model)
				lastErr = err
				continue
			}
			fmt.Printf("[VisualQA] Provider %s (%s) error: %v\n", pState.Provider.Type, pState.Provider.Model, err)
			lastErr = err
			continue
		}

		pState.RequestTimes = append(pState.RequestTimes, time.Now())
		pState.DayRequests++

		responseText := extractJSONFromText(result.Text)
		var comparisonResult ComparisonResult
		if err := json.Unmarshal([]byte(responseText), &comparisonResult); err != nil {
			fmt.Printf("[VisualQA] Failed to parse %s response as JSON: %v\n", pState.Provider.Type, err)
			lastErr = fmt.Errorf("[VisualQA] Failed to unmarshal comparison result from %s: %w, Text: %s", pState.Provider.Type, err, responseText)
			continue
		}

		return comparisonResult, nil
	}

	if lastErr != nil {
		return ComparisonResult{}, fmt.Errorf("[VisualQA] All vision providers failed. Last error: %w", lastErr)
	}
	return ComparisonResult{}, fmt.Errorf("[VisualQA] No vision providers available (all rate limited or errored)")
}

// getProviderChain builds the ordered list of vision providers to try.
// Gemini 2.5 Flash free tier: 10 RPM, 20 RPD, 250K TPM (verified 2026-05)
// OpenRouter free: no documented RPD, ~20 RPM per model
func (v *VisualQA) getProviderChain() []*VisionProviderState {
	var states []*VisionProviderState

	if v.apiKey != "" {
		model := v.config.Model
		if model == "" {
			model = "gemini-2.5-flash"
		}
		states = append(states, &VisionProviderState{
			Provider: &VisionProvider{
				Type:   "gemini",
				Model:  model,
				APIKey: v.apiKey,
				RPM:    10,
				RPD:    20,
			},
		})
	}

	if v.config.OpenRouterKey != "" {
		fallbacks := []struct {
			model string
			rpm   int
		}{
			{"google/gemma-4-31b-it:free", 20},
			{"nvidia/nemotron-nano-12b-v2-vl:free", 20},
		}
		for _, fb := range fallbacks {
			states = append(states, &VisionProviderState{
				Provider: &VisionProvider{
					Type:    "openrouter",
					Model:   fb.model,
					APIKey:  v.config.OpenRouterKey,
					BaseURL: "https://openrouter.ai/api/v1",
					RPM:     fb.rpm,
					RPD:     0,
				},
			})
		}
	}

	return states
}

// callVisionProvider dispatches to the right API based on provider type.
func (v *VisualQA) callVisionProvider(ctx context.Context, pState *VisionProviderState, prompt, img1B64, img2B64 string) (*visionCallResult, error) {
	switch pState.Provider.Type {
	case "gemini":
		return v.callGemini(ctx, pState.Provider, prompt, img1B64, img2B64)
	case "openrouter":
		return v.callOpenRouter(ctx, pState.Provider, prompt, img1B64, img2B64)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", pState.Provider.Type)
	}
}

// callGemini calls the Gemini direct API.
func (v *VisualQA) callGemini(ctx context.Context, p *VisionProvider, prompt, img1B64, img2B64 string) (*visionCallResult, error) {
	parts := []GeminiPart{
		{Text: prompt},
		{InlineData: &GeminiInlineData{MIMEType: "image/png", Data: img1B64}},
	}
	if img2B64 != "" {
		parts = append(parts,
			GeminiPart{Text: "CURRENT:"},
			GeminiPart{InlineData: &GeminiInlineData{MIMEType: "image/png", Data: img2B64}},
		)
	}

	req := GeminiCompareRequest{
		Contents: []GeminiContent{{Parts: parts}},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.Model, p.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create Gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send Gemini request: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Gemini response: %w", err)
	}

	if res.StatusCode == 429 {
		return nil, fmt.Errorf("Gemini API returned 429 (rate limited): %s", string(respBody))
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API returned status %d: %s", res.StatusCode, string(respBody))
	}

	var geminiResp GeminiCompareResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshal Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in Gemini response: %s", string(respBody))
	}

	return &visionCallResult{Text: geminiResp.Candidates[0].Content.Parts[0].Text}, nil
}

// callOpenRouter calls a vision model via OpenRouter's chat completions API.
func (v *VisualQA) callOpenRouter(ctx context.Context, p *VisionProvider, prompt, img1B64, img2B64 string) (*visionCallResult, error) {
	content := []ORContent{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &ORImageURL{URL: "data:image/png;base64," + img1B64}},
	}
	if img2B64 != "" {
		content = append(content,
			ORContent{Type: "text", Text: "CURRENT:"},
			ORContent{Type: "image_url", ImageURL: &ORImageURL{URL: "data:image/png;base64," + img2B64}},
		)
	}

	req := ORRequest{
		Model:    p.Model,
		Messages: []ORMessage{{Role: "user", Content: content}},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenRouter request: %w", err)
	}

	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	url := baseURL + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send OpenRouter request: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read OpenRouter response: %w", err)
	}

	var orResp ORResponse
	if err := json.Unmarshal(respBody, &orResp); err != nil {
		return nil, fmt.Errorf("unmarshal OpenRouter response: %w", err)
	}

	if orResp.Error != nil {
		errMsg := orResp.Error.Message
		if orResp.Error.Code == 429 || strings.Contains(errMsg, "rate") {
			return nil, fmt.Errorf("OpenRouter 429: %s", errMsg)
		}
		return nil, fmt.Errorf("OpenRouter error (%d): %s", orResp.Error.Code, errMsg)
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenRouter response: %s", string(respBody))
	}

	return &visionCallResult{Text: orResp.Choices[0].Message.Content}, nil
}

// extractJSONFromText extracts JSON from text that may be wrapped in markdown code blocks.
func extractJSONFromText(text string) string {
	start := -1

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

	depth := 0
	end := -1
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
