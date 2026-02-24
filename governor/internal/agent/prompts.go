package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type PromptManager struct {
	dir   string
	cache map[string]string
	mu    sync.RWMutex
}

func NewPromptManager(dir string) *PromptManager {
	return &PromptManager{
		dir:   dir,
		cache: make(map[string]string),
	}
}

func (p *PromptManager) Get(promptPath string) (string, error) {
	key := promptPath

	p.mu.RLock()
	if cached, ok := p.cache[key]; ok {
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	content, err := p.load(promptPath)
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	p.cache[key] = content
	p.mu.Unlock()

	return content, nil
}

func (p *PromptManager) load(promptPath string) (string, error) {
	fullPath := p.resolvePath(promptPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read prompt file %s: %w", fullPath, err)
	}

	return string(data), nil
}

func (p *PromptManager) resolvePath(promptPath string) string {
	if filepath.IsAbs(promptPath) {
		return promptPath
	}

	if strings.HasPrefix(promptPath, "prompts/") {
		relativePath := strings.TrimPrefix(promptPath, "prompts/")
		return filepath.Join(p.dir, relativePath)
	}

	return filepath.Join(p.dir, promptPath)
}

func (p *PromptManager) Reload(promptPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.cache, promptPath)

	content, err := p.load(promptPath)
	if err != nil {
		return err
	}

	p.cache[promptPath] = content
	return nil
}

func (p *PromptManager) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache = make(map[string]string)
}

func (p *PromptManager) Dir() string {
	return p.dir
}

func (p *PromptManager) List() ([]string, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("read prompts dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}
