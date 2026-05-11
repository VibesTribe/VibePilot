package visualqa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type BaselineEntry struct {
	BaselineFile       string    `json:"baseline_file"`
	LastApprovedCommit string    `json:"last_approved_commit"`
	LastApprovedAt     time.Time `json:"last_approved_at"`
}

type BaselineManifest map[string]map[int]BaselineEntry

// GetBaselinePath returns the filesystem path for a baseline image.
func (v *VisualQA) GetBaselinePath(pageName string, viewportWidth int) string {
	return filepath.Join(v.config.RepoPath, v.config.BaselineDir, "screenshots", fmt.Sprintf("%s_%d.png", pageName, viewportWidth))
}

// LoadManifest reads the baseline manifest from disk.
func (v *VisualQA) LoadManifest() (BaselineManifest, error) {
	manifestPath := filepath.Join(v.config.RepoPath, v.config.BaselineDir, v.config.ManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(BaselineManifest), nil
		}
		return nil, fmt.Errorf("[VisualQA] Failed to read baseline manifest: %w", err)
	}

	var manifest BaselineManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("[VisualQA] Failed to unmarshal baseline manifest: %w", err)
	}
	return manifest, nil
}

// SaveManifest writes the baseline manifest to disk.
func (v *VisualQA) SaveManifest(manifest BaselineManifest) error {
	manifestPath := filepath.Join(v.config.RepoPath, v.config.BaselineDir, v.config.ManifestFile)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to marshal baseline manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return fmt.Errorf("[VisualQA] Failed to create manifest directory: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("[VisualQA] Failed to write baseline manifest: %w", err)
	}
	return nil
}

// SaveBaseline moves a captured image to the baseline directory and updates the manifest.
func (v *VisualQA) SaveBaseline(ctx context.Context, pageName string, viewportWidth int, capturePath string) error {
	baselinePath := v.GetBaselinePath(pageName, viewportWidth)
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0755); err != nil {
		return fmt.Errorf("[VisualQA] Failed to create baseline directory: %w", err)
	}

	// Copy instead of rename so the temp dir cleanup doesn't fail
	srcData, err := os.ReadFile(capturePath)
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to read captured image: %w", err)
	}
	if err := os.WriteFile(baselinePath, srcData, 0644); err != nil {
		return fmt.Errorf("[VisualQA] Failed to write baseline image: %w", err)
	}

	manifest, err := v.LoadManifest()
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to load manifest for saving baseline: %w", err)
	}

	if manifest[pageName] == nil {
		manifest[pageName] = make(map[int]BaselineEntry)
	}

	manifest[pageName][viewportWidth] = BaselineEntry{
		BaselineFile:   filepath.Base(baselinePath),
		LastApprovedAt: time.Now(),
	}

	if err := v.SaveManifest(manifest); err != nil {
		return fmt.Errorf("[VisualQA] Failed to save manifest after saving baseline: %w", err)
	}

	if v.config.GitCommitBaselines && v.config.RepoPath != "" {
		commitMsg := fmt.Sprintf("feat(visualqa): Add/update baseline for %s_%d", pageName, viewportWidth)
		manifestPath := filepath.Join(v.config.RepoPath, v.config.BaselineDir, v.config.ManifestFile)
		if err := v.gitAddAndCommit(ctx, []string{baselinePath, manifestPath}, commitMsg); err != nil {
			return fmt.Errorf("[VisualQA] Failed to commit baseline to git: %w", err)
		}
	}

	return nil
}

// ApproveBaseline marks a baseline as approved and commits the change.
func (v *VisualQA) ApproveBaseline(ctx context.Context, pageName string, viewportWidth int) error {
	baselinePath := v.GetBaselinePath(pageName, viewportWidth)
	manifest, err := v.LoadManifest()
	if err != nil {
		return fmt.Errorf("[VisualQA] Failed to load manifest for approving baseline: %w", err)
	}

	if manifest[pageName] == nil || manifest[pageName][viewportWidth].BaselineFile == "" {
		return fmt.Errorf("[VisualQA] No baseline found to approve for %s_%d", pageName, viewportWidth)
	}

	manifest[pageName][viewportWidth] = BaselineEntry{
		BaselineFile:   filepath.Base(baselinePath),
		LastApprovedAt: time.Now(),
	}

	if err := v.SaveManifest(manifest); err != nil {
		return fmt.Errorf("[VisualQA] Failed to save manifest after approving baseline: %w", err)
	}

	if v.config.GitCommitBaselines && v.config.RepoPath != "" {
		commitMsg := fmt.Sprintf("feat(visualqa): Approve baseline for %s_%d", pageName, viewportWidth)
		manifestPath := filepath.Join(v.config.RepoPath, v.config.BaselineDir, v.config.ManifestFile)
		if err := v.gitAddAndCommit(ctx, []string{baselinePath, manifestPath}, commitMsg); err != nil {
			return fmt.Errorf("[VisualQA] Failed to commit baseline approval to git: %w", err)
		}
	}

	return nil
}

// gitAddAndCommit adds files and commits with the given message.
// files are relative to RepoPath. commitMsg is the commit message.
func (v *VisualQA) gitAddAndCommit(ctx context.Context, files []string, commitMsg string) error {
	if v.config.RepoPath == "" {
		return fmt.Errorf("[VisualQA] RepoPath is empty, cannot git commit")
	}

	// git add with explicit file paths
	args := []string{"-C", v.config.RepoPath, "add", "--"}
	args = append(args, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %v, Output: %s", err, string(output))
	}

	// git commit with message
	cmd = exec.CommandContext(ctx, "git", "-C", v.config.RepoPath, "commit", "-m", commitMsg)
	if output, err := cmd.CombinedOutput(); err != nil {
		if !bytes.Contains(output, []byte("nothing to commit")) {
			return fmt.Errorf("git commit failed: %v, Output: %s", err, string(output))
		}
	}

	return nil
}
