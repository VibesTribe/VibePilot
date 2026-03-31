package connectors

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/vibepilot/governor/internal/runtime"
	"github.com/vibepilot/governor/internal/vault"
)

const (
	DefaultTimeoutSecs = 300
)

var DefaultCLIArgs = []string{"run", "--format", "json"}

type SecretProvider interface {
	GetSecret(ctx context.Context, keyName string) (string, error)
}

type CLIRunner struct {
	command string
	cliArgs []string
	timeout time.Duration
	workDir string
}

func NewCLIRunner(command string, timeoutSecs int) *CLIRunner {
	return NewCLIRunnerWithArgs(command, nil, timeoutSecs)
}

func NewCLIRunnerWithArgs(command string, cliArgs []string, timeoutSecs int) *CLIRunner {
	if timeoutSecs <= 0 {
		timeoutSecs = DefaultTimeoutSecs
	}
	if len(cliArgs) == 0 {
		cliArgs = DefaultCLIArgs
	}
	return &CLIRunner{
		command: command,
		cliArgs: cliArgs,
		timeout: time.Duration(timeoutSecs) * time.Second,
	}
}

func NewCLIRunnerWithWorkDir(command string, cliArgs []string, timeoutSecs int, workDir string) *CLIRunner {
	r := NewCLIRunnerWithArgs(command, cliArgs, timeoutSecs)
	r.workDir = workDir
	return r
}

func (r *CLIRunner) Run(ctx context.Context, prompt string, timeout int) (string, int, int, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	// FIX: Pass prompt via STDIN, not as command-line argument
	// This works for all CLI tools that use -p flag (claude, kilo, etc.)
	cmd := exec.CommandContext(ctx, r.command, r.cliArgs...)
	cmd.Stdin = strings.NewReader(prompt)

	if r.workDir != "" {
		cmd.Dir = r.workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", 0, 0, fmt.Errorf("%s: %w\nstderr: %s", r.command, err, stderr.String())
	}

	output := stdout.String()

	var content strings.Builder
	var tokensIn, tokensOut int

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		switch entryType {
		// Claude Code stream-json format: type "assistant" with message.content[].text
		case "assistant":
			if msg, ok := entry["message"].(map[string]interface{}); ok {
				if contentArr, ok := msg["content"].([]interface{}); ok {
					for _, item := range contentArr {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if text, ok := itemMap["text"].(string); ok {
								content.WriteString(text)
							}
						}
					}
				}
			}
		// Legacy format: type "text" with part.text
		case "text":
			if part, ok := entry["part"].(map[string]interface{}); ok {
				if text, ok := part["text"].(string); ok {
					content.WriteString(text)
				}
			}
		// Claude Code result format with usage
		case "result":
			if usage, ok := entry["usage"].(map[string]interface{}); ok {
				if in, ok := usage["input_tokens"].(float64); ok {
					tokensIn = int(in)
				}
				if out, ok := usage["output_tokens"].(float64); ok {
					tokensOut = int(out)
				}
			}
			// Also check for result field as fallback text
			if resultText, ok := entry["result"].(string); ok && content.Len() == 0 {
				content.WriteString(resultText)
			}
		// Legacy step_finish format
		case "step_finish":
			if part, ok := entry["part"].(map[string]interface{}); ok {
				if tokens, ok := part["tokens"].(map[string]interface{}); ok {
					if in, ok := tokens["input"].(float64); ok {
						tokensIn = int(in)
					}
					if out, ok := tokens["output"].(float64); ok {
						tokensOut = int(out)
					}
				}
			}
		}
	}

	result := content.String()
	if result == "" {
		return output, len(prompt) / 4, len(output) / 4, nil
	}

	return result, tokensIn, tokensOut, nil
}

func (r *CLIRunner) RunWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string, timeout int) (string, int, int, error) {
	combined := fmt.Sprintf("SYSTEM:\n%s\n\nUSER:\n%s", systemPrompt, userPrompt)
	return r.Run(ctx, combined, timeout)
}

type APIRunner struct {
	endpoint       string
	apiKeyRef      string
	model          string
	provider       string
	httpClient     *http.Client
	timeout        time.Duration
	secretProvider SecretProvider
}

type APIRunnerConfig struct {
	Endpoint       string
	APIKeyRef      string
	Model          string
	Provider       string
	TimeoutSeconds int
	SecretProvider SecretProvider
}

func NewAPIRunner(cfg *APIRunnerConfig) *APIRunner {
	timeoutSecs := DefaultTimeoutSecs
	if cfg.TimeoutSeconds > 0 {
		timeoutSecs = cfg.TimeoutSeconds
	}
	timeout := time.Duration(timeoutSecs) * time.Second
	return &APIRunner{
		endpoint:       cfg.Endpoint,
		apiKeyRef:      cfg.APIKeyRef,
		model:          cfg.Model,
		provider:       cfg.Provider,
		httpClient:     &http.Client{Timeout: timeout},
		timeout:        timeout,
		secretProvider: cfg.SecretProvider,
	}
}

func NewAPIRunnerFromConfig(conn runtime.ConnectorConfig, secrets SecretProvider) *APIRunner {
	model := ""
	if len(conn.Models) > 0 {
		model = conn.Models[0]
	}

	provider := conn.Provider
	if provider == "" {
		provider = detectProvider(conn.Endpoint)
	}

	timeoutSecs := DefaultTimeoutSecs
	if conn.TimeoutSeconds > 0 {
		timeoutSecs = conn.TimeoutSeconds
	}

	return NewAPIRunner(&APIRunnerConfig{
		Endpoint:       conn.Endpoint,
		APIKeyRef:      conn.APIKeyRef,
		Model:          model,
		Provider:       provider,
		TimeoutSeconds: timeoutSecs,
		SecretProvider: secrets,
	})
}

func detectProvider(endpoint string) string {
	if strings.Contains(endpoint, "generativelanguage.googleapis.com") {
		return "google"
	}
	if strings.Contains(endpoint, "api.deepseek.com") {
		return "deepseek"
	}
	if strings.Contains(endpoint, "api.openai.com") {
		return "openai"
	}
	if strings.Contains(endpoint, "api.anthropic.com") {
		return "anthropic"
	}
	return "unknown"
}

func (r *APIRunner) getAPIKey(ctx context.Context) (string, error) {
	if r.secretProvider == nil || r.apiKeyRef == "" {
		return "", fmt.Errorf("API key not configured for %s", r.endpoint)
	}
	return r.secretProvider.GetSecret(ctx, r.apiKeyRef)
}

func (r *APIRunner) Run(ctx context.Context, prompt string, timeout int) (string, int, int, error) {
	apiKey, err := r.getAPIKey(ctx)
	if err != nil {
		return "", 0, 0, fmt.Errorf("retrieve API key: %w", err)
	}

	switch r.provider {
	case "google":
		return r.callGemini(ctx, prompt, apiKey)
	case "deepseek":
		url := r.endpoint
		if !strings.Contains(url, "/chat/completions") {
			url = strings.TrimSuffix(r.endpoint, "/") + "/v1/chat/completions"
		}
		return r.callOpenAICompatible(ctx, prompt, url, apiKey)
	case "openai":
		url := strings.TrimSuffix(r.endpoint, "/") + "/chat/completions"
		return r.callOpenAICompatible(ctx, prompt, url, apiKey)
	default:
		return "", 0, 0, fmt.Errorf("unsupported provider: %s", r.provider)
	}
}

func (r *APIRunner) callGemini(ctx context.Context, prompt, apiKey string) (string, int, int, error) {
	url := fmt.Sprintf("%s/%s:generateContent", r.endpoint, r.model)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", 0, 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	content, tokensIn, tokensOut := parseGeminiResponse(respBody)
	if content == "" {
		return "", 0, 0, fmt.Errorf("empty response from Gemini")
	}

	return content, tokensIn, tokensOut, nil
}

func (r *APIRunner) callOpenAICompatible(ctx context.Context, prompt, url, apiKey string) (string, int, int, error) {
	reqBody := map[string]interface{}{
		"model": r.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", 0, 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	content, tokensIn, tokensOut := parseOpenAIResponse(respBody)
	if content == "" {
		return "", 0, 0, fmt.Errorf("empty response from API")
	}

	return content, tokensIn, tokensOut, nil
}

func parseGeminiResponse(body []byte) (string, int, int) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, 0
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", 0, 0
	}

	return result.Candidates[0].Content.Parts[0].Text,
		result.UsageMetadata.PromptTokenCount,
		result.UsageMetadata.CandidatesTokenCount
}

func parseOpenAIResponse(body []byte) (string, int, int) {
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, 0
	}

	if len(result.Choices) == 0 {
		return "", 0, 0
	}

	return result.Choices[0].Message.Content,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens
}

type VaultAdapter struct {
	v *vault.Vault
}

func NewVaultAdapter(v *vault.Vault) *VaultAdapter {
	return &VaultAdapter{v: v}
}

func (a *VaultAdapter) GetSecret(ctx context.Context, keyName string) (string, error) {
	return a.v.GetSecret(ctx, keyName)
}
