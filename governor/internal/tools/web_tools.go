package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vibepilot/governor/internal/runtime"
)

type WebSearchTool struct {
	allowlist []string
	config    *runtime.WebToolsConfig
}

func NewWebSearchTool(allowlist []string, config *runtime.WebToolsConfig) *WebSearchTool {
	if config == nil {
		config = &runtime.WebToolsConfig{
			SearchURL:        "https://api.duckduckgo.com/",
			UserAgent:        "Mozilla/5.0 (compatible; VibePilot/2.0)",
			MaxFetchLength:   10000,
			MaxRelatedTopics: 5,
			TimeoutSeconds:   30,
		}
	}
	return &WebSearchTool{allowlist: allowlist, config: config}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	escapedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1", t.config.SearchURL, escapedQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"query":   query,
		})
	}

	req.Header.Set("User-Agent", t.config.UserAgent)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"query":   query,
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"query":   query,
		})
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   "failed to parse search results",
			"query":   query,
		})
	}

	abstract, _ := result["Abstract"].(string)
	abstractText, _ := result["AbstractText"].(string)
	abstractSource, _ := result["AbstractSource"].(string)
	abstractURL, _ := result["AbstractURL"].(string)

	relatedTopics, _ := result["RelatedTopics"].([]any)
	var topics []map[string]string
	maxTopics := t.config.MaxRelatedTopics
	if maxTopics <= 0 {
		maxTopics = 5
	}
	for _, tp := range relatedTopics {
		if topic, ok := tp.(map[string]any); ok {
			text, _ := topic["Text"].(string)
			firstURL, _ := topic["FirstURL"].(string)
			if text != "" {
				topics = append(topics, map[string]string{
					"text": text,
					"url":  firstURL,
				})
			}
		}
		if len(topics) >= maxTopics {
			break
		}
	}

	return json.Marshal(map[string]any{
		"success": true,
		"query":   query,
		"result": map[string]any{
			"abstract":        abstract,
			"abstract_text":   abstractText,
			"abstract_source": abstractSource,
			"abstract_url":    abstractURL,
			"related_topics":  topics,
		},
	})
}

type WebFetchTool struct {
	allowlist []string
	config    *runtime.WebToolsConfig
}

func NewWebFetchTool(allowlist []string, config *runtime.WebToolsConfig) *WebFetchTool {
	if config == nil {
		config = &runtime.WebToolsConfig{
			SearchURL:        "https://api.duckduckgo.com/",
			UserAgent:        "Mozilla/5.0 (compatible; VibePilot/2.0)",
			MaxFetchLength:   10000,
			MaxRelatedTopics: 5,
			TimeoutSeconds:   30,
		}
	}
	return &WebFetchTool{allowlist: allowlist, config: config}
}

func (t *WebFetchTool) isAllowed(urlStr string) bool {
	if len(t.allowlist) == 0 {
		return true
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	host := parsedURL.Hostname()
	for _, allowed := range t.allowlist {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter required")
	}

	if !strings.HasPrefix(urlStr, "https://") {
		if strings.HasPrefix(urlStr, "http://") {
			return json.Marshal(map[string]any{
				"success": false,
				"error":   "only HTTPS URLs are allowed",
				"url":     urlStr,
			})
		}
		urlStr = "https://" + urlStr
	}

	if !t.isAllowed(urlStr) {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   "URL host not in allowlist",
			"url":     urlStr,
		})
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"url":     urlStr,
		})
	}

	req.Header.Set("User-Agent", t.config.UserAgent)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"url":     urlStr,
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"url":     urlStr,
		})
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)
	maxLen := t.config.MaxFetchLength
	if maxLen <= 0 {
		maxLen = 10000
	}
	if len(content) > maxLen {
		content = content[:maxLen] + "\n... (truncated)"
	}

	return json.Marshal(map[string]any{
		"success":      true,
		"url":          urlStr,
		"status":       resp.StatusCode,
		"content_type": contentType,
		"content":      content,
		"size":         len(body),
	})
}
