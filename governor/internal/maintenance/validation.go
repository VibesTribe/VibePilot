package maintenance

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *Maintenance) Backup(target string) (string, error) {
	if target == "" {
		return "", nil
	}

	srcPath := filepath.Join(m.repoPath, target)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", nil
	}

	backupDir := filepath.Join(m.sandboxDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	safeName := strings.ReplaceAll(target, "/", "_")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s-%s.bak", safeName, timestamp))

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return "", fmt.Errorf("copy to backup: %w", err)
	}

	log.Printf("Maintenance: backed up %s to %s", target, backupPath)
	return backupPath, nil
}

func (m *Maintenance) Rollback(backupPath, target string) error {
	if backupPath == "" || target == "" {
		return nil
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupPath)
	}

	srcFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer srcFile.Close()

	targetPath := filepath.Join(m.repoPath, target)
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	dstFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create target: %w", err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return fmt.Errorf("restore from backup: %w", err)
	}

	log.Printf("Maintenance: rolled back %s from %s", target, backupPath)
	return nil
}

func (m *Maintenance) ValidateConfig(path string, content []byte) error {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		return m.validateJSON(content)
	case ".yaml", ".yml":
		return nil
	case ".go":
		return nil
	case ".sql":
		return nil
	default:
		return nil
	}
}

func (m *Maintenance) validateJSON(content []byte) error {
	var js interface{}
	if err := json.Unmarshal(content, &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func (m *Maintenance) ValidatePlatforms(content []byte) error {
	var platforms struct {
		Platforms []map[string]interface{} `json:"platforms"`
	}

	if err := json.Unmarshal(content, &platforms); err != nil {
		return fmt.Errorf("invalid platforms.json: %w", err)
	}

	for i, p := range platforms.Platforms {
		if _, ok := p["id"]; !ok {
			return fmt.Errorf("platform %d missing id", i)
		}
		if _, ok := p["type"]; !ok {
			return fmt.Errorf("platform %d missing type", i)
		}
	}

	return nil
}

func (m *Maintenance) ValidateModels(content []byte) error {
	var models struct {
		Models []map[string]interface{} `json:"models"`
	}

	if err := json.Unmarshal(content, &models); err != nil {
		return fmt.Errorf("invalid models.json: %w", err)
	}

	for i, m := range models.Models {
		if _, ok := m["id"]; !ok {
			return fmt.Errorf("model %d missing id", i)
		}
		if _, ok := m["destination"]; !ok {
			return fmt.Errorf("model %d missing destination", i)
		}
	}

	return nil
}

func (m *Maintenance) ValidateRoles(content []byte) error {
	var roles struct {
		Roles []map[string]interface{} `json:"roles"`
	}

	if err := json.Unmarshal(content, &roles); err != nil {
		return fmt.Errorf("invalid roles.json: %w", err)
	}

	for i, r := range roles.Roles {
		if _, ok := r["id"]; !ok {
			return fmt.Errorf("role %d missing id", i)
		}
		if _, ok := r["prompt"]; !ok {
			return fmt.Errorf("role %d missing prompt", i)
		}
	}

	return nil
}

func (m *Maintenance) CanRollback(changeID string) (bool, error) {
	backupDir := filepath.Join(m.sandboxDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), changeID) {
			return true, nil
		}
	}

	return false, nil
}

func (m *Maintenance) GetBackups() ([]string, error) {
	backupDir := filepath.Join(m.sandboxDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

func (m *Maintenance) CleanupOldBackups(maxAge time.Duration) error {
	backupDir := filepath.Join(m.sandboxDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	var cleaned int

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(backupDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("Maintenance: failed to remove old backup %s: %v", entry.Name(), err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("Maintenance: cleaned up %d old backups", cleaned)
	}

	return nil
}
