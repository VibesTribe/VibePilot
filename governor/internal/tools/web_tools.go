package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebSearchTool struct {
	timeout time.Duration
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{timeout: 30 * time.Second}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	escapedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", escapedQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"query":   query,
		})
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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
	for _, t := range relatedTopics {
		if topic, ok := t.(map[string]any); ok {
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
	timeout time.Duration
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{timeout: 30 * time.Second}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter required")
	}

	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"url":     urlStr,
		})
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VibePilot/1.0)")

	client := &http.Client{}
	resp, err := client.Do(req)
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
	var content string
	if strings.Contains(contentType, "application/json") {
		content = string(body)
	} else {
		content = string(body)
		if len(content) > 10000 {
			content = content[:10000] + "\n... (truncated)"
		}
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
