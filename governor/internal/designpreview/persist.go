package designpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Manifest tracks all generated design files.
type Manifest struct {
	Designs []ManifestEntry `json:"designs"`
}

// ManifestEntry records a single design file in the manifest.
type ManifestEntry struct {
	DesignID  string    `json:"design_id"`
	TaskID    string    `json:"task_id"`
	Version   int       `json:"version"`
	HTMLPath  string    `json:"html_path"`
	ScreenshotPath string `json:"screenshot_path,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PGXDB adapts a pgxpool.Pool to the designpreview DB interface.
// Uses the same pattern as visualqa.PGXDB.
type PGXDB struct {
	pool interface {
		Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error)
		Query(ctx context.Context, query string, args ...interface{}) (json.RawMessage, error)
	}
}

// SaveManifest writes the design manifest to disk.
func SaveManifest(repoPath, manifestFile string, manifest *Manifest) error {
	manifestPath := filepath.Join(repoPath, manifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return fmt.Errorf("[DesignPreview] Failed to create manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("[DesignPreview] Failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("[DesignPreview] Failed to write manifest: %w", err)
	}

	return nil
}

// LoadManifest reads the design manifest from disk.
func LoadManifest(repoPath, manifestFile string) (*Manifest, error) {
	manifestPath := filepath.Join(repoPath, manifestFile)
	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return &Manifest{Designs: []ManifestEntry{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("[DesignPreview] Failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// AddToManifest adds a design entry to the manifest and saves it.
func AddToManifest(repoPath, manifestFile string, taskID string, version int, htmlPath, screenshotPath string) error {
	manifest, err := LoadManifest(repoPath, manifestFile)
	if err != nil {
		return err
	}

	entry := ManifestEntry{
		DesignID:      uuid.New().String(),
		TaskID:        taskID,
		Version:       version,
		HTMLPath:      htmlPath,
		ScreenshotPath: screenshotPath,
		CreatedAt:     time.Now(),
	}

	manifest.Designs = append(manifest.Designs, entry)
	return SaveManifest(repoPath, manifestFile, manifest)
}
