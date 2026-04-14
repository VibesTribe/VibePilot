package dag

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Registry manages loaded workflow definitions.
type Registry struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow // name -> workflow
	dir       string
}

// NewRegistry creates a workflow registry that loads from a directory.
func NewRegistry(pipelinesDir string) *Registry {
	return &Registry{
		workflows: make(map[string]*Workflow),
		dir:       pipelinesDir,
	}
}

// LoadAll reads all .yaml/.yml files from the pipelines directory.
func (r *Registry) LoadAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no pipelines dir is fine
		}
		return fmt.Errorf("read pipelines dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(r.dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		wf, err := LoadWorkflow(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		r.workflows[wf.Name] = wf
	}

	return nil
}

// Get returns a workflow by name.
func (r *Registry) Get(name string) *Workflow {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.workflows[name]
}

// List returns all loaded workflow names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.workflows))
	for name := range r.workflows {
		names = append(names, name)
	}
	return names
}

// Reload re-reads all pipeline files (for hot reload).
func (r *Registry) Reload() error {
	newWorkflows := make(map[string]*Workflow)

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.mu.Lock()
			r.workflows = newWorkflows
			r.mu.Unlock()
			return nil
		}
		return fmt.Errorf("read pipelines dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(r.dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		wf, err := LoadWorkflow(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		newWorkflows[wf.Name] = wf
	}

	r.mu.Lock()
	r.workflows = newWorkflows
	r.mu.Unlock()
	return nil
}
