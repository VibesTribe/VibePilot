package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	WebSearchURL   = "https://api.duckduckgo.com/"
	WebFetchMaxLen = 10000
	UserAgent      = "Mozilla/5.0 (compatible; VibePilot/2.0)"
)

type WebSearchTool struct {
	allowlist []string
}

func NewWebSearchTool(allowlist []string) *WebSearchTool {
	return &WebSearchTool{allowlist: allowlist}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	escapedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1", WebSearchURL, escapedQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"query":   query,
		})
	}

	req.Header.Set("User-Agent", UserAgent)

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
		if len(topics) >= 5 {
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
}

func NewWebFetchTool(allowlist []string) *WebFetchTool {
	return &WebFetchTool{allowlist: allowlist}
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

	req.Header.Set("User-Agent", UserAgent)

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
	if len(content) > WebFetchMaxLen {
		content = content[:WebFetchMaxLen] + "\n... (truncated)"
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
