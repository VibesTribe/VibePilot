package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// fetchContent retrieves file content by trying GitHub raw URL first,
// then falling back to the local filesystem (REPO_PATH).
// This ensures the pipeline never breaks just because a file hasn't been
// pushed to GitHub yet (e.g. plan file push failed, or network issue).
func fetchContent(ctx context.Context, repoPath, filePath string) ([]byte, error) {
	// Try GitHub raw URL first (source of truth when available)
	repoOwner := "VibesTribe"
	repoName := "VibePilot"
	branch := "main"
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", repoOwner, repoName, branch, filePath)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err == nil && httpResp.StatusCode == 200 {
		content, readErr := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		if readErr == nil {
			return content, nil
		}
	}
	if httpResp != nil {
		httpResp.Body.Close()
	}

	// Fallback: local filesystem
	localPath := filepath.Join(repoPath, filePath)
	content, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("not on GitHub and not local (%s): %w", localPath, err)
	}
	log.Printf("[fetchContent] Using local file (GitHub unavailable): %s", localPath)
	return content, nil
}
