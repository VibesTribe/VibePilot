package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileReadTool struct {
	repoPath string
}

func NewFileReadTool(repoPath string) *FileReadTool {
	absPath, _ := filepath.Abs(repoPath)
	return &FileReadTool{repoPath: absPath}
}

func (t *FileReadTool) isPathAllowed(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if filepath.IsAbs(path) {
		return false
	}

	fullPath := filepath.Join(t.repoPath, path)
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		realPath = fullPath
	}

	realRepo, err := filepath.EvalSymlinks(t.repoPath)
	if err != nil {
		realRepo = t.repoPath
	}

	return strings.HasPrefix(realPath, realRepo+string(os.PathSeparator)) || realPath == realRepo
}

func (t *FileReadTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter required")
	}

	if !t.isPathAllowed(path) {
		return nil, fmt.Errorf("path not allowed: potential traversal attack")
	}

	fullPath := filepath.Join(t.repoPath, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"path":    path,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"path":    path,
		"content": string(content),
		"size":    len(content),
	})
}

type FileWriteTool struct {
	repoPath string
}

func NewFileWriteTool(repoPath string) *FileWriteTool {
	absPath, _ := filepath.Abs(repoPath)
	return &FileWriteTool{repoPath: absPath}
}

func (t *FileWriteTool) isPathAllowed(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if filepath.IsAbs(path) {
		return false
	}

	fullPath := filepath.Join(t.repoPath, path)

	dirPath := filepath.Dir(fullPath)
	realDir, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		realDir = dirPath
	}

	realRepo, err := filepath.EvalSymlinks(t.repoPath)
	if err != nil {
		realRepo = t.repoPath
	}

	return strings.HasPrefix(realDir, realRepo+string(os.PathSeparator)) || realDir == realRepo
}

func (t *FileWriteTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter required")
	}
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter required")
	}

	if !t.isPathAllowed(path) {
		return nil, fmt.Errorf("path not allowed: potential traversal attack")
	}

	fullPath := filepath.Join(t.repoPath, path)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("create directory: %v", err),
			"path":    path,
		})
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"path":    path,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"path":    path,
		"size":    len(content),
		"message": fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
	})
}

type FileDeleteTool struct {
	repoPath string
}

func NewFileDeleteTool(repoPath string) *FileDeleteTool {
	return &FileDeleteTool{repoPath: repoPath}
}

func (t *FileDeleteTool) Execute(ctx context.Context, args map[string]any) (json.RawMessage, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter required")
	}

	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("path traversal not allowed")
	}

	fullPath := filepath.Join(t.repoPath, path)

	if err := os.Remove(fullPath); err != nil {
		return json.Marshal(map[string]any{
			"success": false,
			"error":   err.Error(),
			"path":    path,
		})
	}

	return json.Marshal(map[string]any{
		"success": true,
		"path":    path,
		"message": fmt.Sprintf("Deleted %s", path),
	})
}
