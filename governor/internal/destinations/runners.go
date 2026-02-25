package destinations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CLIRunner struct {
	command string
	timeout time.Duration
}

func NewCLIRunner(command string, timeoutSecs int) *CLIRunner {
	if timeoutSecs == 0 {
		timeoutSecs = 300
	}
	return &CLIRunner{
		command: command,
		timeout: time.Duration(timeoutSecs) * time.Second,
	}
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

	cmd := exec.CommandContext(ctx, r.command, "run", "--format", "json", prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", 0, 0, fmt.Errorf("%s: %w\nstderr: %s", r.command, err, stderr.String())
	}

	output := stdout.String()

	var result struct {
		Content      string `json:"content"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return output, len(prompt) / 4, len(output) / 4, nil
	}

	return result.Content, result.InputTokens, result.OutputTokens, nil
}

func (r *CLIRunner) RunWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string, timeout int) (string, int, int, error) {
	combined := fmt.Sprintf("SYSTEM:\n%s\n\nUSER:\n%s", systemPrompt, userPrompt)
	return r.Run(ctx, combined, timeout)
}

type APIRunner struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient interface {
		Do(req interface{}) (interface{}, error)
	}
	timeout time.Duration
}

type APIRunnerConfig struct {
	Endpoint   string
	APIKey     string
	Model      string
	TimeoutSec int
}

func NewAPIRunner(cfg *APIRunnerConfig) *APIRunner {
	timeout := 300 * time.Second
	if cfg.TimeoutSec > 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}
	return &APIRunner{
		endpoint: cfg.Endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		timeout:  timeout,
	}
}

func (r *APIRunner) Run(ctx context.Context, prompt string, timeout int) (string, int, int, error) {
	return "", 0, 0, fmt.Errorf("API runner not yet implemented - use CLI runner")
}

func (r *APIRunner) callGemini(ctx context.Context, prompt string) (string, int, int, error) {
	return "", 0, 0, nil
}

func (r *APIRunner) callDeepSeek(ctx context.Context, prompt string) (string, int, int, error) {
	return "", 0, 0, nil
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

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}
